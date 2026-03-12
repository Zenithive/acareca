package field

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqFormField) (*RsFormField, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormField, error)
	Update(ctx context.Context, id uuid.UUID, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqUpdateFormField) (*RsFormField, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error)
}

const MaxFieldsPerVersion = 200

type Service struct {
	repo            IRepository
	coaSvc          coa.Service
	clinicSvc       clinic.Service
	practitionerSvc practitioner.IService
	versionSvc      version.IService
	entryRepo       entry.IRepository
}

func NewService(repo IRepository, coaSvc coa.Service, clinicSvc clinic.Service, practitionerSvc practitioner.IService, versionSvc version.IService, entryRepo entry.IRepository) IService {
	return &Service{
		repo:            repo,
		coaSvc:          coaSvc,
		clinicSvc:       clinicSvc,
		practitionerSvc: practitionerSvc,
		versionSvc:      versionSvc,
		entryRepo:       entryRepo,
	}
}

func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqFormField) (*RsFormField, error) {
	current, err := s.repo.ListByFormVersionID(ctx, formVersionID)
	if err != nil {
		return nil, err
	}
	if len(current)+1 > MaxFieldsPerVersion {
		return nil, errors.New("too many fields")
	}
	coaID, err := uuid.Parse(req.CoaID)
	if err != nil {
		return nil, err
	}
	if _, err := s.clinicSvc.GetClinicByID(ctx, practitionerID, clinicID); err != nil {
		return nil, err
	}
	if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
		if errors.Is(err, coa.ErrNotFound) {
			return nil, errors.New("coa not found")
		}
		return nil, err
	}
	f := req.ToDB(formVersionID)
	if err := s.repo.Create(ctx, f); err != nil {
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

func (s *Service) Update(ctx context.Context, id uuid.UUID, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqUpdateFormField) (*RsFormField, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.clinicSvc.GetClinicByID(ctx, practitionerID, clinicID); err != nil {
		return nil, err
	}
	if req.CoaID != nil {
		coaID, err := uuid.Parse(*req.CoaID)
		if err != nil {
			return nil, err
		}
		if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
			if errors.Is(err, coa.ErrNotFound) {
				return nil, errors.New("coa not found")
			}
			return nil, err
		}
		existing.CoaID = coaID
	}
	if req.Label != nil {
		existing.Label = *req.Label
	}
	if req.SectionType != nil {
		existing.SectionType = *req.SectionType
	}
	if req.PaymentResponsibility != nil {
		existing.PaymentResponsibility = *req.PaymentResponsibility
	}
	if req.TaxType != nil {
		existing.TaxType = *req.TaxType
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if s.entryRepo != nil {
		has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
		if err != nil {
			return err
		}
		if has {
			return errors.New("field has submitted entries")
		}
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error) {
	list, err := s.repo.ListByFormVersionID(ctx, formVersionID)
	if err != nil {
		return nil, err
	}
	rs := make([]*RsFormField, 0, len(list))
	for _, f := range list {
		rs = append(rs, f.ToRs())
	}
	return rs, nil
}
