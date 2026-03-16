package version

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	Create(ctx context.Context, formID, clinicID uuid.UUID, req *RqFormVersion, practitionerID uuid.UUID) (*RsFormVersion, error)
	Get(ctx context.Context, id, clinicID uuid.UUID) (*RsFormVersion, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormVersion, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *RqUpdateFormVersion) (*RsFormVersion, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	List(ctx context.Context, formID, clinicID uuid.UUID) ([]*RsFormVersion, error)
	GetVersionByFormID(ctx context.Context, formID uuid.UUID) (RsFormVersion, error)
}

type service struct {
	repo       IRepository
	formClinic clinic.Service
	db         *sqlx.DB
}

func NewService(db *sqlx.DB, repo IRepository, clinicSvc clinic.Service) IService {
	return &service{db: db, repo: repo, formClinic: clinicSvc}
}

// Create implements [IService].
func (s *service) Create(ctx context.Context, formID, clinicID uuid.UUID, req *RqFormVersion, userID uuid.UUID) (*RsFormVersion, error) {
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if clinic.ID != clinicID {
		return nil, ErrForbidden
	}
	v := req.ToDB(formID, userID)
	if err := s.repo.Create(ctx, v); err != nil {
		return nil, err
	}
	return v.ToRs(), nil
}

// Get implements [IService].
func (s *service) Get(ctx context.Context, id, clinicID uuid.UUID) (*RsFormVersion, error) {
	v, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if clinic.ID != clinicID {
		return nil, ErrForbidden
	}
	return v.ToRs(), nil
}

// GetByID returns the version by ID without clinic check.
func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormVersion, error) {
	v, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return v.ToRs(), nil
}

// Update implements [IService].
func (s *service) Update(ctx context.Context, id, clinicID uuid.UUID, req *RqUpdateFormVersion) (*RsFormVersion, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if clinic.ID != clinicID {
		return nil, ErrForbidden
	}
	if req.Version != nil {
		existing.Version = *req.Version
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

// Delete implements [IService].
func (s *service) Delete(ctx context.Context, id, clinicID uuid.UUID) error {
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
	if err != nil {
		return err
	}
	if clinic.ID != clinicID {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}

// List implements [IService].
func (s *service) List(ctx context.Context, formID, clinicID uuid.UUID) ([]*RsFormVersion, error) {
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	if clinic.ID != clinicID {
		return nil, ErrForbidden
	}
	list, err := s.repo.ListByFormID(ctx, formID)
	if err != nil {
		return nil, err
	}
	rs := make([]*RsFormVersion, 0, len(list))
	for _, v := range list {
		rs = append(rs, v.ToRs())
	}
	return rs, nil
}

// GetVersionByFormID implements [IService].
func (s *service) GetVersionByFormID(ctx context.Context, formID uuid.UUID) (RsFormVersion, error) {
	v, err := s.repo.ListVersionByFormID(ctx, formID)
	if err != nil {
		return RsFormVersion{}, err
	}
	return *v.ToRs(), nil
}
