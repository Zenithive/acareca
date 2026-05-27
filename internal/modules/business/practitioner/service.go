package practitioner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	invitationPkg "github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type ModelInvitationRepo struct {
	InternalRepo invitationPkg.Repository
}

type IService interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error
	ListPractitioners(ctx context.Context, f *Filter) (*util.RsList, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	UpdatePractitionerProfile(ctx context.Context, userID uuid.UUID, req *RqUpdatePractitioner) error
	GetLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*string, error)
	UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error
	VerifyAccountantAccessToPractitioner(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) error
}

type service struct {
	db               *sqlx.DB
	repo             Repository
	subscription     subscription.Service
	userSubscription userSubscription.Service
	coaRepo          coa.Repository
	invitationModel  *ModelInvitationRepo
	auditSvc         audit.Service
	fyrepo           fy.Repository
	notificationSvc  notification.Service
}

func NewService(db *sqlx.DB, repo Repository, subscription subscription.Service, userSubscription userSubscription.Service, coaRepo coa.Repository, auditSvc audit.Service, fyrepo fy.Repository, invitationRepo ...interface{}) IService {
	svc := &service{db: db, repo: repo, subscription: subscription, userSubscription: userSubscription, coaRepo: coaRepo, auditSvc: auditSvc, fyrepo: fyrepo}
	if len(invitationRepo) > 0 {
		if casted, ok := invitationRepo[0].(invitationPkg.Repository); ok {
			svc.invitationModel = &ModelInvitationRepo{InternalRepo: casted}
		}
	}
	return svc
}

func (s *service) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error) {
	existing, err := s.repo.GetPractitionerByUserID(ctx, req.UserID)
	if err == nil && existing != nil {
		return existing, nil
	}

	var t *RsPractitioner

	onboardWork := func(workCtx context.Context, activeTx *sqlx.Tx) error {
		var err error
		t, err = s.repo.CreatePractitioner(workCtx, &RqCreatePractitioner{
			UserID:     req.UserID,
			EntityType: req.EntityType,
			EntityName: req.EntityName,
			ABN:        req.ABN,
			ACN:        req.ACN,
			Address:    req.Address,
			Profession: req.Profession,
		}, activeTx)
		if err != nil {
			return err
		}

		trial, err := s.subscription.FindByName(workCtx, "Trial")
		if err != nil {
			return err
		}

		start := time.Now()
		end := start.AddDate(0, 0, trial.DurationDays)
		_, err = s.userSubscription.Create(workCtx, t.ID, &userSubscription.RqCreatePractitionerSubscription{
			SubscriptionID: trial.ID,
			StartDate:      start.Format(time.RFC3339),
			EndDate:        end.Format(time.RFC3339),
			Status:         userSubscription.StatusActive,
		}, activeTx)
		if err != nil {
			log.Printf("onboarding: create trial subscription for practitioner %s: %v", t.ID, err)
			return err
		}

		if err := coa.SeedDefaultsForPractitioner(workCtx, s.coaRepo, t.ID, activeTx); err != nil {
			log.Printf("onboarding: seed default chart of accounts for practitioner %s: %v", t.ID, err)
			return err
		}
		return nil
	}

	if tx != nil {
		if err := onboardWork(ctx, tx); err != nil {
			return nil, err
		}
	} else {
		if err := util.RunInTransaction(ctx, s.db, onboardWork); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func (s *service) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePractitioner(ctx, id)
}

func (s *service) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	return s.repo.GetPractitioner(ctx, id)
}

func (s *service) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	return s.repo.GetPractitionerByUserID(ctx, userID)
}

func (s *service) UpdatePractitionerProfile(ctx context.Context, userID uuid.UUID, req *RqUpdatePractitioner) error {
	return s.repo.UpdatePractitionerProfile(ctx, userID, req)
}

func (s *service) ListPractitioners(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	var (
		list  []*PractitionerWithUser
		total int
		err   error
	)

	if f.AccountantID != nil {
		list, err = s.repo.ListPractitionersForAccountant(ctx, *f.AccountantID, ft)
		if err != nil {
			return nil, err
		}
		total, err = s.repo.CountPractitionersForAccountant(ctx, *f.AccountantID, ft)
		if err != nil {
			return nil, err
		}
	} else {
		list, err = s.repo.ListPractitioners(ctx, ft)
		if err != nil {
			return nil, err
		}
		total, err = s.repo.CountPractitioners(ctx, ft)
		if err != nil {
			return nil, err
		}
	}

	data := make([]*RsPractitioner, 0, len(list))
	for _, p := range list {
		data = append(data, p.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(data, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) GetLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*string, error) {
	settings, err := s.repo.GetFinancialSettings(ctx, practitionerID, fyID)
	if err != nil || settings == nil {
		return nil, err
	}
	return settings.LockDate, nil
}

func (s *service) UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error {
	fyYear, err := s.fyrepo.GetFinancialYearByID(ctx, fyID)
	if err != nil {
		return fmt.Errorf("invalid financial year: %w", err)
	}

	if lockDate != nil && *lockDate != "" {
		parsedLockDate, err := time.Parse("2006-01-02", *lockDate)
		if err != nil {
			return fmt.Errorf("invalid lock date format: %w", err)
		}
		if parsedLockDate.Before(fyYear.StartDate) {
			return fmt.Errorf("lock date cannot be before the financial year start date: %s", fyYear.StartDate.Format("2006-01-02"))
		}
		if parsedLockDate.After(time.Now().Truncate(24 * time.Hour)) {
			return fmt.Errorf("lock date cannot be in the future")
		}
	}

	meta := auditctx.GetMetadata(ctx)
	beforeState, err := s.repo.GetFinancialSettings(ctx, practitionerID, fyID)
	if err != nil {
		return fmt.Errorf("get before state: %w", err)
	}

	if err = s.repo.UpdateLockDate(ctx, practitionerID, fyID, lockDate); err != nil {
		return err
	}

	afterState, err := s.repo.GetFinancialSettings(ctx, practitionerID, fyID)
	if err != nil {
		return fmt.Errorf("get after state: %w", err)
	}

	fyIDStr := fyID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionLockDateUpdated,
		Module:      auditctx.ModuleBusiness,
		EntityType:  lo.ToPtr(auditctx.EntityFinancialSettings),
		EntityID:    &fyIDStr,
		BeforeState: beforeState,
		AfterState:  afterState,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
}

func (s *service) VerifyAccountantAccessToPractitioner(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) error {
	if s.invitationModel == nil {
		return errors.New("invitation repository not available")
	}

	perms, err := s.invitationModel.InternalRepo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, accountantID)
	if err != nil || perms == nil {
		if err != nil {
			return fmt.Errorf("failed to get permissions: %w", err)
		}
		return errors.New("accountant does not have access to this practitioner")
	}

	lockDatePerms, exists := (*perms)[invitationPkg.PermLockDates]
	if !exists || !lockDatePerms.Read {
		return errors.New("accountant does not have read permission for lock dates")
	}

	return nil
}
