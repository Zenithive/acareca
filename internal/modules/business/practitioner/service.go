package practitioner

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/subscription"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
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
}

type service struct {
	repo             Repository
	subscription     subscription.Service
	userSubscription userSubscription.Service
	coaRepo          coa.Repository
}

func NewService(repo Repository, subscription subscription.Service, userSubscription userSubscription.Service, coaRepo coa.Repository) IService {
	return &service{repo: repo, subscription: subscription, userSubscription: userSubscription, coaRepo: coaRepo}
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
