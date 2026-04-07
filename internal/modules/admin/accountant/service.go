package accountant

import (
	"context"

	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	ListAccountantsWithPractitioners(ctx context.Context, f *Filter) (*util.RsList, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// ListAccountantsWithPractitioners returns a list of accountants with their associated practitioners
func (s *service) ListAccountantsWithPractitioners(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()

	list, err := s.repo.ListAccountantsWithPractitioners(ctx, ft)
	if err != nil {
		return nil, err
	}

	var rsList util.RsList
	rsList.MapToList(list, len(list), *ft.Offset, *ft.Limit)
	return &rsList, nil
}
