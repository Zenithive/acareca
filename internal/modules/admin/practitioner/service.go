package practitioner

import (
	"context"

	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	ListPractitionersWithSubscriptions(ctx context.Context, f *Filter) (*util.RsList, error)
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

	var rsList util.RsList
	rsList.MapToList(list, len(list), *ft.Offset, *ft.Limit)
	return &rsList, nil
}
