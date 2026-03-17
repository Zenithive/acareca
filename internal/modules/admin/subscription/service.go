package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
)

type Service interface {
	CreateSubscription(ctx context.Context, req *RqCreateSubscription) (*RsSubscription, error)
	GetSubscription(ctx context.Context, id int) (*RsSubscription, error)
	ListSubscriptions(ctx context.Context) ([]*RsSubscription, error)
	UpdateSubscription(ctx context.Context, id int, req *RqUpdateSubscription) (*RsSubscription, error)
	DeleteSubscription(ctx context.Context, id int) error
	FindByName(ctx context.Context, name string) (*RsSubscription, error)
}

type service struct {
	repo     Repository
	auditSvc audit.Service
}

func NewService(repo Repository, auditSvc audit.Service) Service {
	return &service{repo: repo, auditSvc: auditSvc}
}

func (s *service) CreateSubscription(ctx context.Context, req *RqCreateSubscription) (*RsSubscription, error) {
	sub := req.ToSubscription()
	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return nil, err
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
	return sub.ToRs(), nil
}

func (s *service) ListSubscriptions(ctx context.Context) ([]*RsSubscription, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*RsSubscription, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) UpdateSubscription(ctx context.Context, id int, req *RqUpdateSubscription) (*RsSubscription, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Capture before state for audit
	beforeState := *existing

	applyUpdate(existing, req)
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
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

	err = s.repo.Delete(ctx, id)
	if err != nil {
		return err
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
