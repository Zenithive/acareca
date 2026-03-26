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
	db         *sqlx.DB
	repo       IRepository
	versionSvc version.IService
	limitsSvc  limits.Service
}

func NewService(db *sqlx.DB, repo IRepository, versionSvc version.IService) IService {
	return &Service{db: db, repo: repo, versionSvc: versionSvc, limitsSvc: limits.NewService(db)}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, d *RqFormDetail, clinicID uuid.UUID, practitionerID uuid.UUID) (*RsFormDetail, error) {
	if err := s.limitsSvc.Check(ctx, practitionerID, limits.KeyFormCreate); err != nil {
		return nil, err
	}

	formDetail := d.ToDB(clinicID)
	var result *RsFormDetail
	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.CreateTx(ctx, tx, formDetail); err != nil {
			return err
		}
		_, err := s.versionSvc.CreateTx(ctx, tx, formDetail.ID, clinicID, &version.RqFormVersion{
			Version:  1,
			IsActive: true,
		}, practitionerID)
		if err != nil {
			return err
		}
		result = formDetail.ToRs()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
	rsList.MapToList(items, total, *ft.Offset, *ft.Limit)

	return &rsList, nil
}

func applyFormUpdatePatch(existing *FormDetail, d *RqUpdateFormDetail) error {
	if existing.Status == StatusArchived {
		return errors.New("form is archived")
	}
	if d.Name != nil {
		existing.Name = *d.Name
	}
	if d.Description != nil {
		existing.Description = d.Description
	}
	if d.OwnerShare != nil {
		existing.OwnerShare = *d.OwnerShare
	}
	if d.ClinicShare != nil {
		existing.ClinicShare = *d.ClinicShare
	}
	if d.SuperComponent != nil {
		existing.SuperComponent = d.SuperComponent
	}
	if existing.Status != StatusPublished {
		if d.Status != nil {
			existing.Status = *d.Status
		}
		if d.Method != nil {
			existing.Method = *d.Method
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
	allVersions, err := s.versionSvc.List(ctx, existing.ID, existing.ClinicID)
	if err != nil {
		return nil, err
	}
	versionNum := 1
	for _, v := range allVersions {
		if v.Version >= versionNum {
			versionNum = v.Version + 1
		}
	}
	var updated *RsFormDetail
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		upd, err := s.repo.UpdateTx(ctx, tx, existing)
		if err != nil {
			return err
		}
		_, err = s.versionSvc.CreateTx(ctx, tx, existing.ID, existing.ClinicID, &version.RqFormVersion{
			Version:  versionNum,
			IsActive: true,
		}, practitionerID)
		if err != nil {
			return err
		}
		updated = upd.ToRs()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
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
