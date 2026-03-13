package subscription

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	Create(ctx context.Context, practitionerID uuid.UUID, req *RqCreatePractitionerSubscription, tx *sqlx.Tx) (*RsPractitionerSubscription, error)
	GetByID(ctx context.Context, id int) (*RsPractitionerSubscription, error)
	ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID) ([]*RsPractitionerSubscription, error)
	Update(ctx context.Context, id int, req *RqUpdatePractitionerSubscription) (*RsPractitionerSubscription, error)
	Delete(ctx context.Context, id int) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, practitionerID uuid.UUID, req *RqCreatePractitionerSubscription, tx *sqlx.Tx) (*RsPractitionerSubscription, error) {
	start, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, err
	}
	sub := &PractitionerSubscription{
		PractitionerID: practitionerID,
		SubscriptionID: req.SubscriptionID,
		StartDate:      start,
		EndDate:        end,
		Status:         req.Status,
	}
	created, err := s.repo.Create(ctx, sub, tx)
	if err != nil {
		return nil, err
	}
	return created.ToRs(), nil
}

func (s *service) GetByID(ctx context.Context, id int) (*RsPractitionerSubscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return sub.ToRs(), nil
}

func (s *service) ListByPractitionerID(ctx context.Context, practitionerID uuid.UUID) ([]*RsPractitionerSubscription, error) {
	list, err := s.repo.ListByPractitionerID(ctx, practitionerID)
	if err != nil {
		return nil, err
	}
	out := make([]*RsPractitionerSubscription, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, id int, req *RqUpdatePractitionerSubscription) (*RsPractitionerSubscription, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	existing.UpdatedAt = time.Now()
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

func (s *service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
