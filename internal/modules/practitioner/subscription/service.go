package subscription

import (
	"context"
	"time"
)

type Service interface {
	Create(ctx context.Context, tentantID int, req *RqCreateTentantSubscription) (*RsTentantSubscription, error)
	GetByID(ctx context.Context, id int) (*RsTentantSubscription, error)
	ListByTentantID(ctx context.Context, tentantID int) ([]*RsTentantSubscription, error)
	Update(ctx context.Context, id int, req *RqUpdateTentantSubscription) (*RsTentantSubscription, error)
	Delete(ctx context.Context, id int) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, tentantID int, req *RqCreateTentantSubscription) (*RsTentantSubscription, error) {
	start, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, err
	}
	status := StatusActive
	if req.Status != nil {
		status = *req.Status
	}
	sub := &TentantSubscription{
		TentantID:      tentantID,
		SubscriptionID: req.SubscriptionID,
		StartDate:      start,
		EndDate:        end,
		Status:         status,
	}
	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return nil, err
	}
	return created.ToRs(), nil
}

func (s *service) GetByID(ctx context.Context, id int) (*RsTentantSubscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return sub.ToRs(), nil
}

func (s *service) ListByTentantID(ctx context.Context, tentantID int) ([]*RsTentantSubscription, error) {
	list, err := s.repo.ListByTentantID(ctx, tentantID)
	if err != nil {
		return nil, err
	}
	out := make([]*RsTentantSubscription, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, id int, req *RqUpdateTentantSubscription) (*RsTentantSubscription, error) {
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
