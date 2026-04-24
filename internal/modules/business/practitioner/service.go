package practitioner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"

	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	invitationPkg "github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	userSubscription "github.com/iamarpitzala/acareca/internal/modules/business/subscription"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error
	ListPractitioners(ctx context.Context, f *Filter) (*util.RsList, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	UpdateABN(ctx context.Context, userID uuid.UUID, abn *string) error
	GetLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*string, error)
	UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error
	VerifyAccountantAccessToPractitioner(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) error
	GetBulkLockDates(ctx context.Context, pIDs []uuid.UUID, fyID uuid.UUID) ([]map[string]interface{}, error)
}

type service struct {
	repo             Repository
	subscription     subscription.Service
	userSubscription userSubscription.Service
	coaRepo          coa.Repository
	invitationRepo   interface{}
}

func NewService(repo Repository, subscription subscription.Service, userSubscription userSubscription.Service, coaRepo coa.Repository, invitationRepo ...interface{}) IService {
	svc := &service{repo: repo, subscription: subscription, userSubscription: userSubscription, coaRepo: coaRepo}
	if len(invitationRepo) > 0 {
		svc.invitationRepo = invitationRepo[0]
	}
	return svc
}

func (s *service) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error) {

	existing, err := s.repo.GetPractitionerByUserID(ctx, req.UserID)
	if err == nil && existing != nil {
		return existing, nil
	}
	t, err := s.repo.CreatePractitioner(ctx, &RqCreatePractitioner{UserID: req.UserID}, tx)
	if err != nil {
		return nil, err
	}
	trial, err := s.subscription.FindByName(ctx, "Trial")
	if err != nil {
		return nil, err
	}
	start := time.Now()
	end := start.AddDate(0, 0, trial.DurationDays)
	_, err = s.userSubscription.Create(ctx, t.ID, &userSubscription.RqCreatePractitionerSubscription{
		SubscriptionID: trial.ID,
		StartDate:      start.Format(time.RFC3339),
		EndDate:        end.Format(time.RFC3339),
		Status:         userSubscription.StatusActive,
	}, tx)
	if err != nil {
		log.Printf("onboarding: create trial subscription for practitioner %s: %v", t.ID, err)
		return nil, err
	}

	if err := coa.SeedDefaultsForPractitioner(ctx, s.coaRepo, t.ID, tx); err != nil {
		log.Printf("onboarding: seed default chart of accounts for practitioner %s: %v", t.ID, err)
		return nil, err
	}
	return t, nil
}

// DeletePractitioner implements [IService].
func (s *service) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePractitioner(ctx, id)
}

// GetPractitioner implements [IService].
func (s *service) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	return s.repo.GetPractitioner(ctx, id)
}

// GetPractitionerByUserID implements [IService].
func (s *service) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	return s.repo.GetPractitionerByUserID(ctx, userID)
}

// UpdateABN implements [IService].
func (s *service) UpdateABN(ctx context.Context, userID uuid.UUID, abn *string) error {
	return s.repo.UpdateABN(ctx, userID, abn)
}

// ListPractitioners implements [IService].
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

// GetLockDate retrieves the lock date for a specific practitioner and financial year
func (s *service) GetLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*string, error) {
	settings, err := s.repo.GetFinancialSettings(ctx, practitionerID, fyID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}
	return settings.LockDate, nil
}

// UpdateLockDate updates or clears the lock date
func (s *service) UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error {
	// Business Logic: You might want to prevent setting a lock date too far in the future
	if lockDate != nil && *lockDate != "" {
		// Example: return fmt.Errorf("lock date cannot be more than 1 year in the future")
	}

	return s.repo.UpdateLockDate(ctx, practitionerID, fyID, lockDate)
}

func (s *service) GetBulkLockDates(ctx context.Context, pIDs []uuid.UUID, fyID uuid.UUID) ([]map[string]interface{}, error) {
	// 1. Call repository to get existing records
	settings, err := s.repo.GetBulkLockDates(ctx, pIDs, fyID)
	if err != nil {
		return nil, fmt.Errorf("service get bulk lock dates: %w", err)
	}

	// 2. Create a map for quick lookup of results found in DB
	foundMap := make(map[uuid.UUID]string)
	for _, item := range settings {
		if item.LockDate != nil {
			foundMap[item.PractitionerID] = *item.LockDate
		}
	}

	// 3. Prepare the final response list
	// This ensures every requested Practitioner ID gets a response entry,
	// even if it's "null" in the DB.
	finalResults := make([]map[string]interface{}, 0, len(pIDs))
	for _, id := range pIDs {
		lockDate, exists := foundMap[id]

		result := map[string]interface{}{
			"practitioner_id": id,
			"lock_date":       nil,
		}

		if exists {
			result["lock_date"] = lockDate
		}

		finalResults = append(finalResults, result)
	}

	return finalResults, nil
}

// VerifyAccountantAccessToPractitioner verifies that an accountant has access to a practitioner
func (s *service) VerifyAccountantAccessToPractitioner(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) error {
	if s.invitationRepo == nil {
		return errors.New("invitation repository not available")
	}

	// Cast the invitationRepo to the correct type
	invitationRepo, ok := s.invitationRepo.(invitationPkg.Repository)
	if !ok {
		return errors.New("invalid invitation repository type")
	}

	// Get permissions for this accountant-practitioner relationship
	perms, err := invitationRepo.GetPermissionsByPractitionerAndAccountant(ctx, practitionerID, accountantID)
	if err != nil {
		return fmt.Errorf("failed to get permissions: %w", err)
	}

	// If no permissions exist, the accountant doesn't have access
	if perms == nil {
		return errors.New("accountant does not have access to this practitioner")
	}

	// Check if accountant has read access to lock_dates
	lockDatePerms, exists := (*perms)[invitationPkg.PermLockDates]
	if !exists || !lockDatePerms.Read {
		return errors.New("accountant does not have read permission for lock dates")
	}

	return nil
}
