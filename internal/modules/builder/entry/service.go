package entry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, practitionerID uuid.UUID) (*RsFormEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, formVersionID uuid.UUID, filter Filter) (*util.RsList, error)
	GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)

	ListTransactions(ctx context.Context, filter TransactionFilter) (*util.RsList, error)
}

type Service struct {
	repo      IRepository
	fieldRepo field.IRepository
	methodSvc method.IService
	limitsSvc limits.Service
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc, limitsSvc: limits.NewService(db)}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, practitionerID uuid.UUID) (*RsFormEntry, error) {
	if err := s.limitsSvc.Check(ctx, practitionerID, limits.KeyTransactionCreate); err != nil {
		return nil, err
	}

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
	created, vals, err := s.repo.GetByID(ctx, e.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch entry after create: %w", err)
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
	updated, vals, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch entry after update: %w", err)
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

// ListTransactions implements [IService].
func (s *Service) ListTransactions(ctx context.Context, filter TransactionFilter) (*util.RsList, error) {
	f := filter.ToCommonFilter()

	items, err := s.repo.ListTransactions(ctx, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountTransactions(ctx, f)
	if err != nil {
		return nil, err
	}

	var rs util.RsList
	rs.MapToList(items, total, f.Offset, f.Limit)
	return &rs, nil
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

		if field.TaxType == nil {
			return nil, fmt.Errorf("field %s has no tax type set", fieldID)
		}

		var gstAmount *float64
		netBase := v.Amount
		grossTotal := v.Amount

		taxType := method.TaxTreatment(*field.TaxType)
		switch taxType {

		case method.TaxTreatmentInclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: v.Amount})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount
			netBase = result.Amount         // ex-GST base  (e.g. 100 when input is 110)
			grossTotal = result.TotalAmount // = v.Amount  (e.g. 110)

		case method.TaxTreatmentExclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: v.Amount})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount
			netBase = v.Amount              // ex-GST base  (e.g. 100)
			grossTotal = result.TotalAmount // base + GST (e.g. 110)

		case method.TaxTreatmentManual:
			gstAmount = v.GstAmount
			netBase = v.Amount
			if gstAmount != nil {
				grossTotal = v.Amount + *gstAmount
			}

		case method.TaxTreatmentZero:
			gstAmount = nil
			netBase = v.Amount
			grossTotal = v.Amount

		default:
			return nil, fmt.Errorf("unsupported tax treatment: %s", taxType)
		}

		formValue := &FormEntryValue{
			ID:          uuid.New(),
			EntryID:     entryID,
			FormFieldID: fieldID,
			NetAmount:   &netBase,
			GstAmount:   gstAmount,
			GrossAmount: &grossTotal,
		}

		out = append(out, formValue)
	}

	return out, nil
}
