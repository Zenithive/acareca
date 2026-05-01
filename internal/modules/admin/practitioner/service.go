package practitioner

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	ListPractitionersWithSubscriptions(ctx context.Context, f *Filter) (*util.RsList, error)
	GetPractitionerWithSubscription(ctx context.Context, id uuid.UUID) (*RsPractitionerWithSubscription, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// ListPractitionersWithSubscriptions returns a list of practitioners with their active subscriptions
func (s *service) ListPractitionersWithSubscriptions(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()

	list, err := s.repo.ListPractitionersWithSubscriptions(ctx, ft, f.HasActiveSubscription)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountPractitionersWithSubscriptions(ctx, ft, f.HasActiveSubscription)
	if err != nil {
		return nil, err
	}

	var rsList util.RsList
	rsList.MapToList(list, total, *ft.Offset, *ft.Limit)
	return &rsList, nil
}

// GetPractitionerWithSubscription returns a single practitioner with subscription details
func (s *service) GetPractitionerWithSubscription(ctx context.Context, id uuid.UUID) (*RsPractitionerWithSubscription, error) {
	return s.repo.GetPractitionerWithSubscription(ctx, id)
}
