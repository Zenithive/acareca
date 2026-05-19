package invoice

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, invoice *RqInvoice) error
	Update(ctx context.Context, invoice *RqUpdateInvoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error)
	List(ctx context.Context) ([]*RsInvoice, error)
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
func (s *Service) Create(ctx context.Context, invoice *RqInvoice) error {
	return s.repo.Create(ctx, invoice.ToInvoice())
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Get implements [IService].
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error) {
	invoice, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return invoice.ToRsInvoice(), nil
}

// List implements [IService].
func (s *Service) List(ctx context.Context) ([]*RsInvoice, error) {
	invoices, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	rsInvoices := make([]*RsInvoice, 0, len(invoices))
	for _, invoice := range invoices {
		rsInvoices = append(rsInvoices, invoice.ToRsInvoice())
	}

	return rsInvoices, nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, invoice *RqUpdateInvoice) error {
	existing, err := s.repo.Get(ctx, invoice.ID)
	if err != nil {
		return err
	}

	return s.repo.Update(ctx, invoice.ApplyToInvoice(existing))
}
