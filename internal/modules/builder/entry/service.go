package entry

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
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
	repo           IRepository
	fieldRepo      field.IRepository
	methodSvc      method.IService
	limitsSvc      limits.Service
	detailSvc      detail.IService
	versionSvc     version.IService
	auditSvc       audit.Service
	eventsSvc      events.Service
	accountantRepo accountant.Repository
	authRepo       auth.Repository
	clinicRepo     clinic.Repository
	formClinic     clinic.Service
	formulaSvc     formula.IService
	fieldSvc       field.IService
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService, detailSvc detail.IService, versionSvc version.IService, auditSvc audit.Service, eventsSvc events.Service, accRepo accountant.Repository, authRepo auth.Repository, clinicRepo clinic.Repository, clinicSvc clinic.Service, formulaSvc formula.IService, fieldSvc field.IService) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc, limitsSvc: limits.NewService(db), detailSvc: detailSvc, versionSvc: versionSvc, auditSvc: auditSvc, formulaSvc: formulaSvc, eventsSvc: eventsSvc, accountantRepo: accRepo, authRepo: authRepo, clinicRepo: clinicRepo, formClinic: clinicSvc, fieldSvc: fieldSvc}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, practitionerID uuid.UUID) (*RsFormEntry, error) {
	meta := auditctx.GetMetadata(ctx)
	// Resolve the REAL owner at the start of THIS function
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, req.ClinicID)
	if err != nil {
		return nil, err
	}

	realOwnerID := clinic.PractitionerID

	if err := s.limitsSvc.Check(ctx, realOwnerID, limits.KeyTransactionCreate); err != nil {
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
		Date:          req.Date,
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
	s.attachFieldMetadata(ctx, result)
	s.attachICCalculation(ctx, result)

	// Record Shared Event
	metaMap := events.JSONBMap{
		"entry_id":        result.ID.String(),
		"form_version_id": formVersionID.String(),
		"clinic_id":       req.ClinicID.String(),
		"status":          result.Status,
	}

	s.recordSharedEvent(ctx, req.ClinicID, formVersionID, auditctx.ActionEntryCreated, result.ID,
		"Accountant %s created a new entry for form: %s",
		metaMap,
	)

	// Audit log: entry created
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
	s.attachFieldMetadata(ctx, rs)
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
	if req.Date != nil {
		existing.Date = req.Date
	}

	// Start as nil. Only calculate if the request actually contains new values.
	var valuesToUpdate []*FormEntryValue = nil
	if len(req.Values) > 0 {
		valuesToUpdate, err = s.CalculateValues(ctx, existing.ID, req.Values)
		if err != nil {
			return nil, err
		}
	}

	// If valuesToUpdate is nil, the repo only updates the status.
	if err := s.repo.Update(ctx, existing, valuesToUpdate); err != nil {
		return nil, err
	}
	updated, vals, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch entry after update: %w", err)
	}

	result := updated.ToRs(vals)
	s.attachFieldMetadata(ctx, result)
	s.attachICCalculation(ctx, result)

	// Record Shared Event
	metaMap := events.JSONBMap{
		"entry_id":        result.ID.String(),
		"form_version_id": existing.FormVersionID.String(),
		"clinic_id":       existing.ClinicID.String(),
		"status":          result.Status,
	}

	s.recordSharedEvent(ctx, existing.ClinicID, existing.FormVersionID, auditctx.ActionEntryUpdated, id,
		"Accountant %s updated entry for form: %s",
		metaMap,
	)

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

	// Record Shared Event
	metaMap := events.JSONBMap{
		"entry_id":  existing.ID.String(),
		"clinic_id": existing.ClinicID.String(),
	}

	s.recordSharedEvent(ctx, existing.ClinicID, existing.FormVersionID, auditctx.ActionEntryDeleted, id,
		"Accountant %s deleted an entry for form: %s",
		metaMap,
	)

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
	rs.MapToList(data, total, *f.Offset, *f.Limit)
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
	rs.MapToList(items, total, *f.Offset, *f.Limit)
	return &rs, nil
}

func (s *Service) CalculateValues(ctx context.Context, entryID uuid.UUID, rq []RqEntryValue) ([]*FormEntryValue, error) {
	out := make([]*FormEntryValue, 0, len(rq))

	keyValues := make(map[string]float64, len(rq))
	taxTypeByKey := make(map[string]string, len(rq))

	for _, v := range rq {
		fieldID, err := uuid.Parse(v.FormFieldID)
		if err != nil {
			return nil, err
		}

		f, err := s.fieldRepo.GetByID(ctx, fieldID)
		if err != nil {
			return nil, err
		}

		if f.IsComputed {
			continue
		}

		// Handle both old format (amount) and new format (net_amount/gross_amount)
		var inputAmount float64
		if v.NetAmount != nil {
			// New format: use net_amount
			inputAmount = *v.NetAmount
		} else if v.GrossAmount != nil {
			// New format: use gross_amount
			inputAmount = *v.GrossAmount
		} else {
			// Old format: use amount
			inputAmount = v.Amount
		}

		var gstAmount *float64
		netBase := inputAmount
		grossTotal := inputAmount

		if f.TaxType == nil {
			// No tax type: net = gross, use netBase for formulas
			// EXCEPTION: OTHER_COST always uses gross (which equals net here)
			keyValues[f.FieldKey] = netBase
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

		taxType := method.TaxTreatment(*f.TaxType)
		switch taxType {

		case method.TaxTreatmentInclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: inputAmount})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount
			netBase = result.Amount
			grossTotal = result.TotalAmount

		case method.TaxTreatmentExclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: inputAmount})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount
			netBase = inputAmount
			grossTotal = result.TotalAmount

		case method.TaxTreatmentManual:
			fm, err := s.fieldSvc.GetByID(ctx, f.ID)
			if err != nil {
				return nil, fmt.Errorf("get form for field %s: %w", f.FieldKey, err)
			}

			if fm.SectionType != nil && *fm.SectionType == "COLLECTION" {
				gstAmount = v.GstAmount
				grossTotal = inputAmount
				if v.GstAmount != nil {
					netBase = inputAmount - *v.GstAmount
				}
			} else {
				gstAmount = v.GstAmount
				netBase = inputAmount
				if gstAmount != nil {
					grossTotal = inputAmount + *gstAmount
				}
			}

		case method.TaxTreatmentZero:
			gstAmount = nil
			netBase = inputAmount
			grossTotal = inputAmount

		default:
			return nil, fmt.Errorf("unsupported tax treatment: %s", taxType)
		}

		// CRITICAL: Always use NET amount for formulas
		// EXCEPTION: OTHER_COST fields use GROSS amount (to match live calculation)
		valueForFormula := netBase
		if f.SectionType != nil && *f.SectionType == "OTHER_COST" {
			valueForFormula = grossTotal
		}
		keyValues[f.FieldKey] = valueForFormula
		taxTypeByKey[f.FieldKey] = string(taxType)
		out = append(out, &FormEntryValue{
			ID:          uuid.New(),
			EntryID:     entryID,
			FormFieldID: fieldID,
			NetAmount:   &netBase,
			GstAmount:   gstAmount,
			GrossAmount: &grossTotal,
		})
	}

	if s.formulaSvc != nil && len(rq) > 0 {
		firstFieldID, err := uuid.Parse(rq[0].FormFieldID)
		if err != nil {
			return nil, err
		}
		firstField, err := s.fieldRepo.GetByID(ctx, firstFieldID)
		if err != nil {
			return nil, err
		}

		// Get all fields to compute section totals
		allFields, err := s.fieldRepo.ListByFormVersionID(ctx, firstField.FormVersionID)
		if err != nil {
			return nil, err
		}

		fieldByID := make(map[uuid.UUID]*field.FormField, len(allFields))
		for _, af := range allFields {
			fieldByID[af.ID] = af
		}

		// Compute section totals using NET amounts from out
		// EXCEPTION: OTHER_COST uses GROSS amounts (to match live calculation)
		sectionTotals := make(map[string]float64)
		for _, entryVal := range out {
			f, ok := fieldByID[entryVal.FormFieldID]
			if ok && f.SectionType != nil && *f.SectionType != "" && entryVal.NetAmount != nil {
				sectionKey := "SECTION:" + *f.SectionType
				
				// For OTHER_COST, use GROSS amount instead of NET
				amountToAdd := *entryVal.NetAmount
				if *f.SectionType == "OTHER_COST" && entryVal.GrossAmount != nil {
					amountToAdd = *entryVal.GrossAmount
				}
				
				sectionTotals[sectionKey] += amountToAdd
			}
		}

		// Merge section totals into keyValues
		maps.Copy(keyValues, sectionTotals)
		
		// CRITICAL FIX: Add computed fields with tax types to taxTypeByKey
		// This ensures the formula feedback mechanism uses GROSS values for dependent formulas
		for _, f := range allFields {
			if f.IsComputed && f.TaxType != nil && *f.TaxType != "" {
				taxTypeByKey[f.FieldKey] = *f.TaxType
			}
		}

		// Collect manually entered GST amounts for computed fields with MANUAL tax type
		manualGSTByKey := make(map[string]float64)
		for _, v := range rq {
			if v.GstAmount == nil {
				continue
			}
			fieldID, err := uuid.Parse(v.FormFieldID)
			if err != nil {
				continue
			}
			f, ok := fieldByID[fieldID]
			if !ok || !f.IsComputed {
				continue
			}
			if f.TaxType != nil && *f.TaxType == "MANUAL" {
				manualGSTByKey[f.FieldKey] = *v.GstAmount
			}
		}

		computed, err := s.formulaSvc.EvalFormulas(ctx, firstField.FormVersionID, keyValues, taxTypeByKey, manualGSTByKey)
		if err != nil {
			return nil, fmt.Errorf("evaluate formulas: %w", err)
		}

		// Track which field IDs already have a value in out to prevent duplicates.
		alreadyAdded := make(map[uuid.UUID]bool, len(out))
		for _, v := range out {
			alreadyAdded[v.FormFieldID] = true
		}

		for fieldID, val := range computed {
			f, ok := fieldByID[fieldID]
			if !ok {
				continue
			}
			if alreadyAdded[fieldID] {
				continue
			}

			// CRITICAL FIX: Formula already returns NET amount
			// We should NOT re-extract net from it
			netBase := val
			grossTotal := val
			var gstAmount *float64

			if f.TaxType != nil {
				taxType := method.TaxTreatment(*f.TaxType)
				
				switch taxType {
				case method.TaxTreatmentInclusive:
					// Formula returns NET, calculate GST and GROSS from NET
					gst := val * 0.10
					gstAmount = &gst
					netBase = val  // Keep as NET
					grossTotal = val + gst  // NET + GST = GROSS
				case method.TaxTreatmentExclusive:
					// Formula returns NET, calculate GST and GROSS from NET
					gst := val * 0.10
					gstAmount = &gst
					netBase = val  // Keep as NET
					grossTotal = val + gst  // NET + GST = GROSS
				}
			}

			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: fieldID,
				NetAmount:   &netBase,
				GstAmount:   gstAmount,
				GrossAmount: &grossTotal,
			})
		}
	}

	return out, nil
}

// attachFieldMetadata enriches each value in the response with field_key, label, and is_computed.
func (s *Service) attachFieldMetadata(ctx context.Context, rs *RsFormEntry) {
	for i, v := range rs.Values {
		f, err := s.fieldRepo.GetByID(ctx, v.FormFieldID)
		if err != nil {
			continue
		}
		rs.Values[i].FieldKey = f.FieldKey
		rs.Values[i].Label = f.Label
		rs.Values[i].IsComputed = f.IsComputed
	}
}

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

	fieldMap := make(map[uuid.UUID]*field.FormField, len(rs.Values))
	for _, v := range rs.Values {
		f, err := s.fieldRepo.GetByID(ctx, v.FormFieldID)
		if err != nil {
			return
		}
		fieldMap[v.FormFieldID] = f
	}

	var incomeSum, expenseSum, otherCostSum float64
	for _, v := range rs.Values {
		f, ok := fieldMap[v.FormFieldID]
		if !ok || f.SectionType == nil {
			continue
		}
		switch *f.SectionType {
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
	shifted := v * 100
	if shifted < 0 {
		shifted -= 0.5
	} else {
		shifted += 0.5
	}
	return float64(int(shifted)) / 100
}

func strPtr(s string) *string { return &s }

// Helper to record shared events
func (s *Service) recordSharedEvent(ctx context.Context, clinicID uuid.UUID, formVersionID uuid.UUID, action string, entryID uuid.UUID, descriptionTemplate string, metadata events.JSONBMap) {
	meta := auditctx.GetMetadata(ctx)

	// Only act if the user is an Accountant
	if meta.UserType == nil || !strings.EqualFold(*meta.UserType, util.RoleAccountant) || meta.UserID == nil {
		return
	}

	actorUserID, _ := uuid.Parse(*meta.UserID)

	// Resolve Form Name
	formName := "Form"
	ver, err := s.versionSvc.GetByID(ctx, formVersionID)
	if err == nil {
		form, err := s.detailSvc.GetByID(ctx, ver.FormId)
		if err == nil {
			formName = form.Name
		}
	}

	// Resolve PractitionerID from Clinic
	clinic, err := s.clinicRepo.GetClinicByID(ctx, clinicID)
	if err != nil {
		return
	}

	// Resolve Accountant Id & Full Name
	var accountantID uuid.UUID
	var fullName string

	accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
	if err == nil {
		accountantID = accProfile.ID
	} else {
		accountantID = actorUserID
	}

	user, err := s.authRepo.FindByID(ctx, actorUserID)
	if err == nil {
		fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}

	// Record Event
	_ = s.eventsSvc.Record(ctx, events.SharedEvent{
		ID:             uuid.New(),
		PractitionerID: clinic.PractitionerID,
		AccountantID:   accountantID,
		ActorID:        actorUserID,
		ActorName:      &fullName,
		ActorType:      util.RoleAccountant,
		EventType:      action,
		EntityType:     "FORM",
		EntityID:       entryID,
		Description:    fmt.Sprintf(descriptionTemplate, fullName, formName),
		Metadata:       metadata,
		CreatedAt:      time.Now(),
	})
}
