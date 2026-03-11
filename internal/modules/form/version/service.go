package version

import (
	"context"

	"github.com/google/uuid"
)

// FormClinicResolver resolves the clinic that owns a form. Used to enforce clinic scoping.
// If nil, clinic validation is skipped.
type FormClinicResolver interface {
	FormClinicID(ctx context.Context, formID uuid.UUID) (uuid.UUID, error)
}

type IService interface {
	Create(ctx context.Context, formID, clinicID uuid.UUID, req *RqFormVersion, userID uuid.UUID) (*RsFormVersion, error)
	Get(ctx context.Context, id, clinicID uuid.UUID) (*RsFormVersion, error)
	Update(ctx context.Context, id, clinicID uuid.UUID, req *RqUpdateFormVersion) (*RsFormVersion, error)
	Delete(ctx context.Context, id, clinicID uuid.UUID) error
	List(ctx context.Context, formID, clinicID uuid.UUID) ([]*RsFormVersion, error)
}

type service struct {
	repo     IRepository
	formClinic FormClinicResolver
}

func NewService(repo IRepository, formClinic FormClinicResolver) IService {
	return &service{repo: repo, formClinic: formClinic}
}

func (s *service) validateFormClinic(ctx context.Context, formID, clinicID uuid.UUID) error {
	if s.formClinic == nil || clinicID == uuid.Nil {
		return nil
	}
	resolved, err := s.formClinic.FormClinicID(ctx, formID)
	if err != nil {
		return err
	}
	if resolved != clinicID {
		return ErrForbidden
	}
	return nil
}

// Create implements [IService].
func (s *service) Create(ctx context.Context, formID, clinicID uuid.UUID, req *RqFormVersion, userID uuid.UUID) (*RsFormVersion, error) {
	if err := s.validateFormClinic(ctx, formID, clinicID); err != nil {
		return nil, err
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
	if err := s.validateFormClinic(ctx, v.FormId, clinicID); err != nil {
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
	if err := s.validateFormClinic(ctx, existing.FormId, clinicID); err != nil {
		return nil, err
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
	v, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.validateFormClinic(ctx, v.FormId, clinicID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// List implements [IService].
func (s *service) List(ctx context.Context, formID, clinicID uuid.UUID) ([]*RsFormVersion, error) {
	if err := s.validateFormClinic(ctx, formID, clinicID); err != nil {
		return nil, err
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
