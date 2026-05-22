package field

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, clinicID *uuid.UUID, practitionerID *uuid.UUID, req *RqFormField) (*RsFormField, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormField, error)
	GetFieldMap(ctx context.Context, formVersionID uuid.UUID) (map[uuid.UUID]*RsFormField, error)
	Update(ctx context.Context, id uuid.UUID, clinicID uuid.UUID, practitionerID *uuid.UUID, req *RqUpdateFormField) (*RsFormField, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error)
}

const MaxFieldsPerVersion = 200

type Service struct {
	db              *sqlx.DB
	repo            IRepository
	coaSvc          coa.Service
	clinicSvc       clinic.Service
	practitionerSvc practitioner.IService
	versionSvc      version.IService
}

func NewService(repo IRepository, coaSvc coa.Service, clinicSvc clinic.Service, practitionerSvc practitioner.IService, versionSvc version.IService, db *sqlx.DB) IService {
	return &Service{
		repo:            repo,
		coaSvc:          coaSvc,
		clinicSvc:       clinicSvc,
		practitionerSvc: practitionerSvc,
		versionSvc:      versionSvc,
		db:              db,
	}
}

func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, clinicID *uuid.UUID, practitionerID *uuid.UUID, req *RqFormField) (*RsFormField, error) {
	var f *FormField
	var err error

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		current, err := s.repo.ListByFormVersionID(ctx, formVersionID)
		if err != nil {
			return err
		}
		if len(current)+1 > MaxFieldsPerVersion {
			return errors.New("too many fields")
		}

		f = req.ToDB(formVersionID)

		if !req.IsComputed {
			coaID, err := uuid.Parse(req.CoaID)
			if err != nil {
				return err
			}
			if _, err := s.clinicSvc.GetClinicByID(ctx, *practitionerID, *clinicID); err != nil {
				return err
			}
			if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, *practitionerID); err != nil {
				if errors.Is(err, coa.ErrNotFound) {
					return errors.New("coa not found")
				}
				return err
			}
		}

		if err := s.repo.Create(ctx, tx, f); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return f.ToRs(), nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormField, error) {
	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return f.ToRs(), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, clinicID uuid.UUID, practitionerID *uuid.UUID, req *RqUpdateFormField) (*RsFormField, error) {
	var updated *FormField
	var err error

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		existing, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if _, err := s.clinicSvc.GetClinicByID(ctx, *practitionerID, clinicID); err != nil {
			return err
		}
		if req.CoaID != nil {
			parsed, err := uuid.Parse(*req.CoaID)
			if err != nil {
				return err
			}
			if _, err := s.coaSvc.GetChartOfAccount(ctx, parsed, *practitionerID); err != nil {
				if errors.Is(err, coa.ErrNotFound) {
					return errors.New("coa not found")
				}
				return err
			}
			existing.CoaID = &parsed
		}
		if req.Label != nil {
			existing.Label = *req.Label
		}
		if req.SectionType != nil {
			existing.SectionType = req.SectionType
		}
		if req.PaymentResponsibility != nil {
			existing.PaymentResponsibility = req.PaymentResponsibility
		}
		if req.TaxType != nil {
			existing.TaxType = req.TaxType
		}
		if req.SortOrder != nil {
			existing.SortOrder = *req.SortOrder
		}
		if req.IsHighlighted != nil {
			existing.IsHighlighted = *req.IsHighlighted
		}
		updated, err = s.repo.Update(ctx, tx, existing)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updated.ToRs(), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.Delete(ctx, tx, id); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error) {
	return s.repo.ListRsByFormVersionID(ctx, formVersionID)
}

func (s *Service) GetFieldMap(ctx context.Context, formVersionID uuid.UUID) (map[uuid.UUID]*RsFormField, error) {
	fields, err := s.repo.ListRsByFormVersionID(ctx, formVersionID)
	if err != nil {
		return nil, err
	}
	m := make(map[uuid.UUID]*RsFormField, len(fields))
	for _, f := range fields {
		m[f.ID] = f
	}
	return m, nil
}
