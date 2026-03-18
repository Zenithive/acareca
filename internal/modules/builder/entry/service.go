package entry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, formVersionID uuid.UUID, filter Filter) (*util.RsList, error)
	GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
}

type Service struct {
	repo      IRepository
	fieldRepo field.IRepository
	methodSvc method.IService
}

func NewService(repo IRepository, fieldRepo field.IRepository, methodSvc method.IService) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	status := EntryStatusDraft
	if req.Status != "" {
		status = req.Status
	}
	var submittedAt *string
	if status == EntryStatusSubmitted {
		now := time.Now().UTC().Format(time.RFC3339)
		submittedAt = &now
	}
	e := &FormEntry{
		ID:            uuid.New(),
		FormVersionID: formVersionID,
		ClinicID:      req.ClinicID,
		SubmittedBy:   submittedBy,
		SubmittedAt:   submittedAt,
		Status:        status,
	}
	values, err := s.CalculateValues(ctx, e.ID, req.Values)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, e, values); err != nil {
		return nil, err
	}
	created, vals, _ := s.repo.GetByID(ctx, e.ID)
	if created == nil {
		return e.ToRs(values), nil
	}
	return created.ToRs(vals), nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.ToRs(values), nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		existing.Status = *req.Status
		if *req.Status == EntryStatusSubmitted && existing.SubmittedAt == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			existing.SubmittedAt = &now
		}
		existing.SubmittedBy = submittedBy
	}
	newValues := values
	if len(req.Values) > 0 {
		newValues, err = s.CalculateValues(ctx, existing.ID, req.Values)
		if err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, existing, newValues); err != nil {
		return nil, err
	}
	updated, vals, _ := s.repo.GetByID(ctx, id)
	if updated == nil {
		return existing.ToRs(values), nil
	}
	return updated.ToRs(vals), nil
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List implements [IService].
func (s *Service) List(ctx context.Context, formVersionID uuid.UUID, filter Filter) (*util.RsList, error) {
	f := filter.MapToFilter()

	list, err := s.repo.ListByFormVersionID(ctx, formVersionID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountByFormVersionID(ctx, formVersionID, f)
	if err != nil {
		return nil, err
	}

	data := make([]*RsFormEntry, 0, len(list))
	for _, e := range list {
		data = append(data, e.ToRs(nil))
	}

	var rs util.RsList
	rs.MapToList(data, total, f.Offset, f.Limit)
	return &rs, nil
}

// GetByVersionID implements [IService].
func (s *Service) GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByVersionID(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.ToRs(values), nil
}

func (s *Service) CalculateValues(ctx context.Context, entryID uuid.UUID, rq []RqEntryValue) ([]*FormEntryValue, error) {
	out := make([]*FormEntryValue, 0, len(rq))

	for _, v := range rq {
		fieldID, err := uuid.Parse(v.FormFieldID)
		if err != nil {
			return nil, err
		}

		field, err := s.fieldRepo.GetByID(ctx, fieldID)
		if err != nil {
			return nil, err
		}

		var gstAmount *float64

		taxType := method.TaxTreatment(*field.TaxType)
		switch taxType {
		case method.TaxTreatmentInclusive, method.TaxTreatmentExclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{
				Amount: v.Amount,
			})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount

		case method.TaxTreatmentManual:
			gstAmount = v.GstAmount

		case method.TaxTreatmentZero:
			gstAmount = nil

		default:
			return nil, fmt.Errorf("unsupported tax treatment: %s", taxType)
		}

		totalAmount := v.Amount
		if gstAmount != nil {
			totalAmount += *gstAmount
		}

		formValue := &FormEntryValue{
			ID:          uuid.New(),
			EntryID:     entryID,
			FormFieldID: fieldID,
			NetAmount:   &v.Amount,
			GstAmount:   gstAmount,
			GrossAmount: &totalAmount,
		}

		out = append(out, formValue)
	}

	return out, nil
}
