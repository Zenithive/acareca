package detail

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID, practitionerID uuid.UUID) (*RsFormDetail, error)
	GetByID(ctx context.Context, formID uuid.UUID) (*RsFormDetail, error)
	Update(ctx context.Context, d *RqUpdateFormDetail, practitionerID uuid.UUID) (*RsFormDetail, error)
	UpdateMetadata(ctx context.Context, d *RqUpdateFormDetail) (*RsFormDetail, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error)
}

type Service struct {
	repo       IRepository
	versionSvc version.IService
	limitsSvc  limits.Service
}

func NewService(db *sqlx.DB, repo IRepository, versionSvc version.IService) IService {
	return &Service{repo: repo, versionSvc: versionSvc, limitsSvc: limits.NewService(db)}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID, practitionerID uuid.UUID) (*RsFormDetail, error) {
	if err := s.limitsSvc.Check(ctx, practitionerID, limits.KeyFormCreate); err != nil {
		return nil, err
	}

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
func (s *Service) List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error) {
	ft := filter.MapToFilter()
	formDetails, err := s.repo.ListForm(ctx, ft, practitionerID)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountForm(ctx, ft, practitionerID)
	if err != nil {
		return nil, err
	}

	items := make([]*RsFormDetail, 0, len(formDetails))
	for _, item := range formDetails {
		items = append(items, item.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(items, total, ft.Offset, ft.Limit)

	return &rsList, nil
}

func applyFormUpdatePatch(existing *FormDetail, d *RqUpdateFormDetail) error {
	if existing.Status == StatusArchived {
		return errors.New("form is archived")
	}
	if existing.Status == StatusPublished {
		if d.Status != nil || d.Method != nil || d.OwnerShare != nil || d.ClinicShare != nil {
			return errors.New("form is published and cannot be updated")
		}
	}
	if d.Name != nil {
		existing.Name = *d.Name
	}
	if d.Description != nil {
		existing.Description = d.Description
	}
	if existing.Status != StatusPublished {
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
	}
	return nil
}

// Update updates form metadata and creates a new active version, deactivating the previous one.
func (s *Service) Update(ctx context.Context, d *RqUpdateFormDetail, practitionerID uuid.UUID) (*RsFormDetail, error) {
	existing, err := s.repo.GetByID(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if err := applyFormUpdatePatch(existing, d); err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	allVersions, err := s.versionSvc.List(ctx, updated.ID, updated.ClinicID)
	if err != nil {
		return nil, err
	}
	versionNum := 1
	for _, v := range allVersions {
		if v.Version >= versionNum {
			versionNum = v.Version + 1
		}
	}
	_, err = s.versionSvc.Create(ctx, updated.ID, updated.ClinicID, &version.RqFormVersion{
		Version:  versionNum,
		IsActive: true,
	}, practitionerID)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

// UpdateMetadata updates only the form row; no version creation. Used by update-with-fields flow.
func (s *Service) UpdateMetadata(ctx context.Context, d *RqUpdateFormDetail) (*RsFormDetail, error) {
	existing, err := s.repo.GetByID(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if err := applyFormUpdatePatch(existing, d); err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, existing)
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
