package detail

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/form/version"
)

type IService interface {
	Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID, practitionerID uuid.UUID) (*RsFormDetail, error)
	GetByID(ctx context.Context, formID uuid.UUID) (*RsFormDetail, error)
	Update(ctx context.Context, d *RqUpdateFormDetail, practitionerID uuid.UUID) (*RsFormDetail, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	ListForm(ctx context.Context, filter Filter) ([]*RsFormDetail, error)
}

type Service struct {
	repo       IRepository
	versionSvc version.IService
}

func NewService(repo IRepository, versionSvc version.IService) IService {
	return &Service{repo: repo, versionSvc: versionSvc}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID, practitionerID uuid.UUID) (*RsFormDetail, error) {
	formDetail := d.ToDB(clinicID)
	if err := s.repo.Create(ctx, formDetail); err != nil {
		return nil, err
	}
	_, err := s.versionSvc.Create(ctx, formDetail.ID, clinicID, &version.RqFormVersion{
		Version:  1,
		IsActive: true,
	}, practitionerID)
	if err != nil {
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
func (s *Service) Update(ctx context.Context, d *RqUpdateFormDetail, practitionerID uuid.UUID) (*RsFormDetail, error) {
	existing, err := s.repo.GetByID(ctx, d.ID)
	if err != nil {
		return nil, err
	}

	if d.Name != nil {
		existing.Name = *d.Name
	}
	if d.Description != nil {
		existing.Description = d.Description
	}
	if d.Status != nil {
		existing.Status = *d.Status
	}
	if d.Method != nil {
		existing.Method = *d.Method
	}
	if d.OwnerShare != nil {
		existing.OwnerShare = *d.OwnerShare
	}
	if d.ClinicShare != nil {
		existing.ClinicShare = *d.ClinicShare
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	// Deactivate currently active versions
	activeVersions, err := s.versionSvc.List(ctx, updated.ID, updated.ClinicID)
	if err != nil {
		return nil, err
	}

	// Deactivate all currently active versions
	for _, v := range activeVersions {
		if v.IsActive {
			isActive := false
			_, err := s.versionSvc.Update(ctx, v.Id, updated.ClinicID, &version.RqUpdateFormVersion{
				Version:  &v.Version,
				IsActive: &isActive,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	// Find the latest version number
	versionNum := 1
	if len(activeVersions) > 0 {
		maxVersion := activeVersions[0].Version
		for _, v := range activeVersions {
			if v.Version > maxVersion {
				maxVersion = v.Version
			}
		}
		versionNum = maxVersion + 1
	}

	// Create a new active version
	isActive := true
	_, err = s.versionSvc.Create(ctx, updated.ID, updated.ClinicID, &version.RqFormVersion{
		Version:  versionNum,
		IsActive: isActive,
	}, practitionerID)
	if err != nil {
		return nil, err
	}

	return updated.ToRs(), nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, formID uuid.UUID) (*RsFormDetail, error) {
	formDetail, err := s.repo.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}

	return formDetail.ToRs(), nil
}
