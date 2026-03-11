package field

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/form/entry"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, practitionerID uuid.UUID, req *RqFormField) (*RsFormField, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormField, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormField) (*RsFormField, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID) ([]*RsFormField, error)
	BulkSyncFields(ctx context.Context, formVersionID uuid.UUID, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
}

var ErrCoaNotFound = errors.New("chart of account not found or does not belong to this practice")
var ErrFieldWrongVersion = errors.New("field does not belong to this form version")
var ErrFieldHasSubmittedEntries = errors.New("cannot delete field: it has submitted entry values")

type Service struct {
	repo            IRepository
	coaSvc          coa.Service
	clinicSvc       clinic.Service
	practitionerSvc practitioner.IService
	entryRepo       entry.IRepository
}

func NewService(repo IRepository, coaSvc coa.Service, clinicSvc clinic.Service, practitionerSvc practitioner.IService, entryRepo entry.IRepository) IService {
	return &Service{
		repo:            repo,
		coaSvc:          coaSvc,
		clinicSvc:       clinicSvc,
		practitionerSvc: practitionerSvc,
		entryRepo:       entryRepo,
	}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, practitionerID uuid.UUID, req *RqFormField) (*RsFormField, error) {

	coaID, err := uuid.Parse(req.CoaID)
	if err != nil {
		return nil, err
	}
	if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
		if errors.Is(err, coa.ErrNotFound) {
			return nil, ErrCoaNotFound
		}
		return nil, err
	}
	f := req.ToDB(formVersionID)
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f.ToRs(), nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormField, error) {
	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return f.ToRs(), nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormField) (*RsFormField, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	clinic, err := s.clinicSvc.GetClinicByID(ctx, existing.FormVersionID)
	if err != nil {
		return nil, err
	}
	if req.CoaID != nil {
		coaID, err := uuid.Parse(*req.CoaID)
		if err != nil {
			return nil, err
		}
		practiceID, err := uuid.Parse(clinic.PracticeID)
		if err != nil {
			return nil, err
		}
		if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practiceID); err != nil {
			if errors.Is(err, coa.ErrNotFound) {
				return nil, ErrCoaNotFound
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
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

// Delete implements [IService]. Rejects delete if the field has SUBMITTED entry values.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if s.entryRepo != nil {
		has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
		if err != nil {
			return err
		}
		if has {
			return ErrFieldHasSubmittedEntries
		}
	}
	return s.repo.Delete(ctx, id)
}

// ListByFormVersionID implements [IService].
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

// BulkSyncFields implements [IService]. Runs delete → update → create in one transaction (repo uses util.RunInTransaction).
func (s *Service) BulkSyncFields(ctx context.Context, formVersionID uuid.UUID, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error) {
	out := &RsBulkSyncFields{
		Created: make([]RsFormField, 0, len(req.Create)),
		Updated: make([]RsFormField, 0, len(req.Update)),
		Deleted: make([]uuid.UUID, 0, len(req.Delete)),
	}

	err := s.repo.RunInTransaction(ctx, func(ctx context.Context, r IRepositoryTx) error {
		for _, id := range req.Delete {
			existing, err := r.GetByID(ctx, id)
			if err != nil {
				return err
			}
			if existing.FormVersionID != formVersionID {
				return ErrFieldWrongVersion
			}
			if s.entryRepo != nil {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
				if err != nil {
					return err
				}
				if has {
					return ErrFieldHasSubmittedEntries
				}
			}
			if err := r.Delete(ctx, id); err != nil {
				return err
			}
			out.Deleted = append(out.Deleted, id)
		}
		for i := range req.Update {
			item := &req.Update[i]
			existing, err := r.GetByID(ctx, item.ID)
			if err != nil {
				return err
			}
			if existing.FormVersionID != formVersionID {
				return ErrFieldWrongVersion
			}
			if item.CoaID != nil {
				coaID, err := uuid.Parse(*item.CoaID)
				if err != nil {
					return err
				}
				if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
					if errors.Is(err, coa.ErrNotFound) {
						return ErrCoaNotFound
					}
					return err
				}
				existing.CoaID = coaID
			}
			if item.Label != nil {
				existing.Label = *item.Label
			}
			if item.SectionType != nil {
				existing.SectionType = *item.SectionType
			}
			if item.PaymentResponsibility != nil {
				existing.PaymentResponsibility = *item.PaymentResponsibility
			}
			if item.TaxType != nil {
				existing.TaxType = *item.TaxType
			}
			updated, err := r.Update(ctx, existing)
			if err != nil {
				return err
			}
			out.Updated = append(out.Updated, *updated.ToRs())
		}
		for i := range req.Create {
			item := &req.Create[i]
			coaID, err := uuid.Parse(item.CoaID)
			if err != nil {
				return err
			}
			if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
				if errors.Is(err, coa.ErrNotFound) {
					return ErrCoaNotFound
				}
				return err
			}
			f := item.ToDB(formVersionID)
			if err := r.Create(ctx, f); err != nil {
				return err
			}
			out.Created = append(out.Created, *f.ToRs())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
