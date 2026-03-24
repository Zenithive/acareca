package entry

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
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
	repo       IRepository
	fieldRepo  field.IRepository
	methodSvc  method.IService
	limitsSvc  limits.Service
	detailSvc  detail.IService
	versionSvc version.IService
	auditSvc   audit.Service
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService, detailSvc detail.IService, versionSvc version.IService, auditSvc audit.Service) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc, limitsSvc: limits.NewService(db), detailSvc: detailSvc, versionSvc: versionSvc, auditSvc: auditSvc}
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

	result := created.ToRs(vals)
	s.attachICCalculation(ctx, result)

	// Audit log: entry created
	meta := auditctx.GetMetadata(ctx)
	idStr := created.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionEntryCreated,
		Module:     auditctx.ModuleForms,
		EntityType: strPtr(auditctx.EntityFormFieldEntry),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

// GetByID implements [IService].
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := e.ToRs(values)
	s.attachICCalculation(ctx, rs)
	return rs, nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	beforeState := existing.ToRs(values)
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

	result := updated.ToRs(vals)
	s.attachICCalculation(ctx, result)

	// Audit log: entry updated
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionEntryUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  strPtr(auditctx.EntityFormFieldEntry),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  result,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return result, nil
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Get entry details before deletion for audit log
	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	beforeState := existing.ToRs(values)

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit log: entry deleted
	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionEntryDeleted,
		Module:      auditctx.ModuleForms,
		EntityType:  strPtr(auditctx.EntityFormFieldEntry),
		EntityID:    &idStr,
		BeforeState: beforeState,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
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

		var gstAmount *float64
		netBase := v.Amount
		grossTotal := v.Amount

		// nil tax_type means no GST — treat as zero-rated
		if field.TaxType == nil {
			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: fieldID,
				NetAmount:   &netBase,
				GstAmount:   nil,
				GrossAmount: &grossTotal,
			})
			continue
		}

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

// attachICCalculation fetches the form detail for the given version and, if the
// form method is INDEPENDENT_CONTRACTOR, computes commission, GST on commission,
// and payment received, then attaches them to the response.
func (s *Service) attachICCalculation(ctx context.Context, rs *RsFormEntry) {
	if s.detailSvc == nil || s.versionSvc == nil {
		return
	}

	ver, err := s.versionSvc.GetByID(ctx, rs.FormVersionID)
	if err != nil {
		return
	}

	form, err := s.detailSvc.GetByID(ctx, ver.FormId)
	if err != nil || form.Method != "INDEPENDENT_CONTRACTOR" {
		return
	}

	// Build fieldMap from the entry values.
	fieldMap := make(map[uuid.UUID]*field.FormField, len(rs.Values))
	for _, v := range rs.Values {
		f, err := s.fieldRepo.GetByID(ctx, v.FormFieldID)
		if err != nil {
			return
		}
		fieldMap[v.FormFieldID] = f
	}

	// Compute net totals per section.
	var incomeSum, expenseSum, otherCostSum float64
	for _, v := range rs.Values {
		f, ok := fieldMap[v.FormFieldID]
		if !ok {
			continue
		}
		switch f.SectionType {
		case field.SectionTypeCollection:
			if v.NetAmount != nil {
				incomeSum += *v.NetAmount
			}
		case field.SectionTypeCost:
			if v.NetAmount != nil {
				expenseSum += *v.NetAmount
			}
		case field.SectionTypeOtherCost:
			if v.NetAmount != nil {
				otherCostSum += *v.NetAmount
			}
		}
	}

	netAmount := incomeSum - expenseSum - otherCostSum
	ownerShare := float64(form.OwnerShare)
	commission := netAmount * (ownerShare / 100)
	gstOnCommission := commission * 0.10
	paymentReceived := commission + gstOnCommission

	// Apply super component if set on the form.
	if form.SuperComponent != nil && *form.SuperComponent > 0 {
		superAmount := commission * (*form.SuperComponent / 100)
		paymentReceived += superAmount
	}

	commission = roundEntry(commission)
	gstOnCommission = roundEntry(gstOnCommission)
	paymentReceived = roundEntry(paymentReceived)

	rs.Commission = &commission
	rs.GstOnCommission = &gstOnCommission
	rs.PaymentReceived = &paymentReceived
}

func roundEntry(v float64) float64 {
	// Round to 2 decimal places.
	shifted := v * 100
	if shifted < 0 {
		shifted -= 0.5
	} else {
		shifted += 0.5
	}
	return float64(int(shifted)) / 100
}

func strPtr(s string) *string { return &s }
