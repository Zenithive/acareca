package subscription

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	sharedstripe "github.com/iamarpitzala/acareca/internal/shared/stripe"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateSubscription(ctx context.Context, req *RqCreateSubscription) (*RsSubscription, error)
	GetSubscription(ctx context.Context, id int) (*RsSubscription, error)
	ListSubscriptions(ctx context.Context, f *Filter) (*util.RsList, error)
	UpdateSubscription(ctx context.Context, id int, req *RqUpdateSubscription) (*RsSubscription, error)
	DeleteSubscription(ctx context.Context, id int) error
	FindByName(ctx context.Context, name string) (*RsSubscription, error)

	// Permission management
	ListPermissions(ctx context.Context, subscriptionID int) ([]*RsSubscriptionPermission, error)
	UpdatePermission(ctx context.Context, subscriptionID int, key string, req *RqUpdatePermission) (*RsSubscriptionPermission, error)
}

type service struct {
	db           *sqlx.DB
	repo         Repository
	auditSvc     audit.Service
	stripeClient sharedstripe.StripeClient
}

func NewService(db *sqlx.DB, repo Repository, auditSvc audit.Service, sc sharedstripe.StripeClient) Service {
	return &service{db: db, repo: repo, auditSvc: auditSvc, stripeClient: sc}
}

func (s *service) CreateSubscription(ctx context.Context, req *RqCreateSubscription) (*RsSubscription, error) {
	sub := req.ToSubscription()

	// Stripe-first: if paid plan, sync to Stripe before DB write
	var productID, priceID string
	if req.Price > 0 && s.stripeClient != nil {
		var err error
		desc := ""
		if req.Description != nil {
			desc = *req.Description
		}
		productID, err = s.stripeClient.CreateProduct(req.Name, desc)
		if err != nil {
			return nil, fmt.Errorf("stripe create product: %w", err)
		}
		priceID, err = s.stripeClient.CreatePrice(productID, int64(req.Price*100), "aud")
		if err != nil {
			return nil, fmt.Errorf("stripe create price: %w", err)
		}
	}

	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return nil, err
	}

	// Persist Stripe IDs after successful DB insert
	if productID != "" && priceID != "" {
		if err := s.repo.UpdateStripeIDs(ctx, created.ID, productID, priceID); err != nil {
			log.Printf("ALERT: stripe product created but db update failed: product=%s price=%s err=%v", productID, priceID, err)
			return nil, err
		}
		created.StripeProductID = &productID
		created.StripePriceID = &priceID
	}

	// Seed permissions if provided in the request
	if len(req.Permissions) > 0 {
		if err := s.repo.BulkInsertPermissions(ctx, created.ID, req.Permissions); err != nil {
			return nil, fmt.Errorf("seed permissions: %w", err)
		}
	}

	// Audit log: subscription created
	meta := auditctx.GetMetadata(ctx)
	idStr := intToStr(created.ID)
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionSubscriptionCreated,
		Module:     auditctx.ModuleAdmin,
		EntityType: strPtr(auditctx.EntitySubscription),
		EntityID:   &idStr,
		AfterState: created,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return created.ToRs(), nil
}

func (s *service) GetSubscription(ctx context.Context, id int) (*RsSubscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	res := sub.ToRs()

	perms, err := s.repo.ListPermissions(ctx, id)

	res.Permissions = make([]*RsSubscriptionPermission, 0)

	if err == nil && len(perms) > 0 {
		for _, p := range perms {
			res.Permissions = append(res.Permissions, p.ToRs())
		}
	}

	return res, nil
}

func (s *service) ListSubscriptions(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	data := make([]*RsSubscription, 0, len(list))
	for _, item := range list {
		data = append(data, item.ToRs())
	}
	var rsList util.RsList
	rsList.MapToList(data, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) UpdateSubscription(ctx context.Context, id int, req *RqUpdateSubscription) (*RsSubscription, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Capture before state for audit
	beforeState := *existing

	// Sync changes to Stripe for paid plans
	if existing.Price > 0 && s.stripeClient != nil && existing.StripeProductID != nil {
		// Sync name/description changes to Stripe product
		nameChanged := req.Name != nil && *req.Name != existing.Name
		descChanged := req.Description != nil
		if nameChanged || descChanged {
			newName := existing.Name
			if req.Name != nil {
				newName = *req.Name
			}
			newDesc := ""
			if req.Description != nil {
				newDesc = *req.Description
			} else if existing.Description != nil {
				newDesc = *existing.Description
			}
			if err := s.stripeClient.UpdateProduct(*existing.StripeProductID, newName, newDesc); err != nil {
				return nil, fmt.Errorf("stripe update product: %w", err)
			}
		}

		// Sync price changes: create new price, set as default, archive old price, update DB
		if req.Price != nil && *req.Price != existing.Price && *req.Price > 0 && existing.StripePriceID != nil {
			newPriceID, err := s.stripeClient.CreatePrice(*existing.StripeProductID, int64(*req.Price*100), "aud")
			if err != nil {
				return nil, fmt.Errorf("stripe create price: %w", err)
			}
			// Set new price as default before archiving old one (Stripe requires this)
			if err := s.stripeClient.SetDefaultPrice(*existing.StripeProductID, newPriceID); err != nil {
				return nil, fmt.Errorf("stripe set default price: %w", err)
			}
			if err := s.stripeClient.ArchivePrice(*existing.StripePriceID); err != nil {
				return nil, fmt.Errorf("stripe archive price: %w", err)
			}
			if err := s.repo.UpdateStripeIDs(ctx, id, *existing.StripeProductID, newPriceID); err != nil {
				log.Printf("ALERT: stripe price updated but db update failed: product=%s price=%s err=%v", *existing.StripeProductID, newPriceID, err)
				return nil, err
			}
		}

		// Archive Stripe Product when deactivating, unarchive when reactivating
		if req.IsActive != nil && !*req.IsActive && existing.IsActive {
			if err := s.stripeClient.ArchiveProduct(*existing.StripeProductID); err != nil {
				return nil, fmt.Errorf("stripe archive product: %w", err)
			}
		}
		if req.IsActive != nil && *req.IsActive && !existing.IsActive {
			if err := s.stripeClient.UnarchiveProduct(*existing.StripeProductID); err != nil {
				return nil, fmt.Errorf("stripe unarchive product: %w", err)
			}
		}
	}

	applyUpdate(existing, req)
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	// Upsert permissions if provided
	if len(req.Permissions) > 0 {
		if err := s.repo.BulkInsertPermissions(ctx, updated.ID, req.Permissions); err != nil {
			return nil, fmt.Errorf("update permissions: %w", err)
		}
	}

	// Audit log: subscription updated
	meta := auditctx.GetMetadata(ctx)
	idStr := intToStr(updated.ID)
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionSubscriptionUpdated,
		Module:      auditctx.ModuleAdmin,
		EntityType:  strPtr(auditctx.EntitySubscription),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  updated,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return updated.ToRs(), nil
}

func applyUpdate(s *Subscription, req *RqUpdateSubscription) {
	if req.Name != nil {
		s.Name = *req.Name
	}
	if req.Description != nil {
		s.Description = req.Description
	}
	if req.Price != nil {
		s.Price = *req.Price
	}
	if req.DurationDays != nil {
		s.DurationDays = *req.DurationDays
	}
	if req.IsActive != nil {
		s.IsActive = *req.IsActive
	}
	s.UpdatedAt = time.Now()
}

func (s *service) DeleteSubscription(ctx context.Context, id int) error {
	// Get existing for audit log
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Archive Stripe Product before deleting (Requirement 3.5)
	if existing.StripeProductID != nil && s.stripeClient != nil {
		if err := s.stripeClient.ArchiveProduct(*existing.StripeProductID); err != nil {
			return fmt.Errorf("stripe archive product: %w", err)
		}
	}

	err = s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	// Soft-delete associated permissions
	if err := s.repo.DeletePermissions(ctx, id); err != nil {
		return fmt.Errorf("delete permissions: %w", err)
	}

	// Audit log: subscription deleted
	meta := auditctx.GetMetadata(ctx)
	idStr := intToStr(id)
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionSubscriptionDeleted,
		Module:      auditctx.ModuleAdmin,
		EntityType:  strPtr(auditctx.EntitySubscription),
		EntityID:    &idStr,
		BeforeState: existing,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
}

func (s *service) FindByName(ctx context.Context, name string) (*RsSubscription, error) {
	sub, err := s.repo.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return sub.ToRs(), nil
}

// Helper functions for audit logging

func strPtr(s string) *string {
	return &s
}

func intToStr(i int) string {
	return fmt.Sprintf("%d", i)
}

func (s *service) ListPermissions(ctx context.Context, subscriptionID int) ([]*RsSubscriptionPermission, error) {
	list, err := s.repo.ListPermissions(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	out := make([]*RsSubscriptionPermission, len(list))
	for i, p := range list {
		out[i] = p.ToRs()
	}
	return out, nil
}

func (s *service) UpdatePermission(ctx context.Context, subscriptionID int, key string, req *RqUpdatePermission) (*RsSubscriptionPermission, error) {
	updated, err := s.repo.UpdatePermission(ctx, subscriptionID, key, req)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}
