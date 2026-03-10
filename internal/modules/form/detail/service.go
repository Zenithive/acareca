package detail

import (
	"context"

	"github.com/google/uuid"
)

type IService interface {
	Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID) (*RsFormDetail, error)
	GetByID(ctx context.Context, formID uuid.UUID) (*RsFormDetail, error)
	Update(ctx context.Context, d *RqUpdateFormDetail) (*RsFormDetail, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	ListForm(ctx context.Context, filter Filter) ([]*RsFormDetail, error)
}

type Service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &Service{repo: repo}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID) (*RsFormDetail, error) {
	formDetail := d.ToDB(clinicID)
	if err := s.repo.Create(ctx, formDetail); err != nil {
		return nil, err
	}
	return formDetail.ToRs(), nil
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, formID uuid.UUID) error {
	return s.repo.Delete(ctx, formID)
}

// ListForm implements [IService].
func (s *Service) ListForm(ctx context.Context, filter Filter) ([]*RsFormDetail, error) {
	formDetails, err := s.repo.ListForm(ctx, filter)
	if err != nil {
		return nil, err
	}
	rs := make([]*RsFormDetail, 0, len(formDetails))
	for _, d := range formDetails {
		rs = append(rs, d.ToRs())
	}
	return rs, nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, d *RqUpdateFormDetail) (*RsFormDetail, error) {
	form := d.Update()
	updatedForm, err := s.repo.Update(ctx, form)
	if err != nil {
		return nil, err
	}
	return updatedForm.ToRs(), nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, formID uuid.UUID) (*RsFormDetail, error) {
	formDetail, err := s.repo.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}

	return formDetail.ToRs(), nil
}
