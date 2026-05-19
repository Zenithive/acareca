package item

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, item *Item) error
	Update(ctx context.Context, item *Item) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*Item, error)
	List(ctx context.Context) ([]*Item, error)
}

type Service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &Service{
		repo: repo,
	}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, item *Item) error {
	panic("unimplemented")
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	panic("unimplemented")
}

// Get implements [IService].
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Item, error) {
	panic("unimplemented")
}

// List implements [IService].
func (s *Service) List(ctx context.Context) ([]*Item, error) {
	panic("unimplemented")
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, item *Item) error {
	panic("unimplemented")
}
