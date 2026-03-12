package subscription

import (
	"context"
	"time"
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
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateSubscription(ctx context.Context, req *RqCreateSubscription) (*RsSubscription, error) {
	sub := req.ToSubscription()
	created, err := s.repo.Create(ctx, sub)
	if err != nil {
		return nil, err
	}
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
	applyUpdate(existing, req)
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
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
	return s.repo.Delete(ctx, id)
}

func (s *service) FindByName(ctx context.Context, name string) (*RsSubscription, error) {
	sub, err := s.repo.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return sub.ToRs(), nil
}
