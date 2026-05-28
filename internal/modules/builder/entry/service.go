package entry

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"maps"
	"strconv"
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
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/xuri/excelize/v2"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID) (*RsFormEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, formVersionID uuid.UUID, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error)
	GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	ListTransactions(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error)
	// COA-grouped endpoints
	ListCoaEntries(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*util.RsList, error)
	ListCoaEntryDetails(ctx context.Context, coaID string, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error)
	ExportTransactionReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, exportType string, userID uuid.UUID, PracIDs []uuid.UUID) (interface{}, string, error)
	generateExcelReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, fullName string, practitionerABN string, period string) (*bytes.Buffer, error)
	ExportTransactionData(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*RsExportData, error)
}

type Service struct {
	repo            IRepository
	fieldRepo       field.IRepository
	methodSvc       method.IService
	limitsSvc       limits.Service
	detailSvc       detail.IService
	versionSvc      version.IService
	auditSvc        audit.Service
	eventsSvc       events.Service
	accountantRepo  accountant.Repository
	authRepo        auth.Repository
	clinicRepo      clinic.Repository
	formClinic      clinic.Service
	formulaSvc      formula.IService
	fieldSvc        field.IService
	invitationSvc   invitation.Service
	detailRepo      detail.IRepository
	financialRepo   fy.Repository
	practitionerSvc practitioner.IService
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService, detailSvc detail.IService, versionSvc version.IService, auditSvc audit.Service, eventsSvc events.Service, accRepo accountant.Repository, authRepo auth.Repository, clinicRepo clinic.Repository, clinicSvc clinic.Service, formulaSvc formula.IService, fieldSvc field.IService, invitationSvc invitation.Service, detailRepo detail.IRepository, financialRepo fy.Repository, practitionerSvc practitioner.IService) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc, limitsSvc: limits.NewService(db), detailSvc: detailSvc, versionSvc: versionSvc, auditSvc: auditSvc, formulaSvc: formulaSvc, eventsSvc: eventsSvc, accountantRepo: accRepo, authRepo: authRepo, clinicRepo: clinicRepo, formClinic: clinicSvc, fieldSvc: fieldSvc, invitationSvc: invitationSvc, detailRepo: detailRepo, financialRepo: financialRepo, practitionerSvc: practitionerSvc}
}

func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID) (*RsFormEntry, error) {
	meta := auditctx.GetMetadata(ctx)

	var result *RsFormEntry

	err := util.RunInTransaction(ctx, s.repo.(*Repository).db, func(ctx context.Context, tx *sqlx.Tx) error {

		clinic, err := s.formClinic.GetClinicByIDInternal(ctx, req.ClinicID)
		if err != nil {
			return err
		}
		if clinic == nil {
			return fmt.Errorf("clinic not found: %s", req.ClinicID)
		}

		realOwnerID := clinic.PractitionerID

		if err := s.limitsSvc.Check(ctx, realOwnerID, limits.KeyTransactionCreate); err != nil {
			return err
		}

		if req.Date != nil && *req.Date != "" {
			parsedDate, err := time.Parse("2006-01-02", *req.Date)
			if err != nil {
				return fmt.Errorf("invalid date format: %w", err)
			}

			_, err = s.financialRepo.GetFinancialYearByDate(ctx, parsedDate)
			if err != nil {
				return fmt.Errorf("the date %s does not fall within an active financial year", parsedDate.Format("02-01-2006"))
			}
		}

		if err := s.validateLockDate(ctx, tx, realOwnerID, req.Date, nil); err != nil {
			return err
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

		var targetID uuid.UUID
		if entityID != uuid.Nil {
			targetID = entityID
		} else {
			targetID = uuid.New()
		}

		e := &FormEntry{
			ID:            targetID,
			FormVersionID: formVersionID,
			ClinicID:      req.ClinicID,
			SubmittedBy:   submittedBy,
			SubmittedAt:   submittedAt,
			Date:          req.Date,
			Status:        status,
		}

		values, err := s.CalculateValues(ctx, e.ID, req.Values)
		if err != nil {
			return err
		}

		if err := s.repo.Create(ctx, tx, e, values); err != nil {
			return err
		}

		if err := s.handleDocumentLinks(ctx, tx, e.ID, req.Documents); err != nil {
			return err
		}

		created, vals, err := s.repo.GetByID(ctx, tx, e.ID)
		if err != nil {
			return fmt.Errorf("fetch entry after create: %w", err)
		}
		if created == nil {
			return fmt.Errorf("failed to retrieve newly created entry: %s", e.ID)
		}

		result = created.ToRs(vals)
		s.enrichResponseMetadata(ctx, result)
		s.attachDocuments(ctx, e.ID, result)

		metaMap := events.JSONBMap{
			"entry_id":        result.ID.String(),
			"form_version_id": formVersionID.String(),
			"clinic_id":       req.ClinicID.String(),
			"status":          result.Status,
		}

		s.recordSharedEvent(ctx, tx, req.ClinicID, formVersionID, auditctx.ActionEntryCreated, result.ID,
			"Accountant %s created a new entry for form: %s",
			metaMap,
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	idStr := result.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionEntryCreated,
		Module:     auditctx.ModuleForms,
		EntityType: lo.ToPtr(auditctx.EntityFormFieldEntry),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	var e *FormEntry
	var values []*FormEntryValue

	err := util.RunInTransaction(ctx, s.repo.(*Repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error

		e, values, err = s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	rs := e.ToRs(values)
	s.enrichResponseMetadata(ctx, rs)
	s.attachDocuments(ctx, id, rs)

	return rs, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	var result *RsFormEntry
	var beforeState interface{}

	err := util.RunInTransaction(ctx, s.repo.(*Repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		var innerErr error

		existing, values, innerErr := s.repo.GetByID(ctx, tx, id)
		if innerErr != nil {
			return innerErr
		}
		if existing == nil {
			return fmt.Errorf("form entry not found: %s", id)
		}

		beforeState = existing.ToRs(values)

		dateToCheck := existing.Date
		if req.Date != nil {
			dateToCheck = req.Date
		}

		if dateToCheck != nil && *dateToCheck != "" {
			parsedDate, innerErr := time.Parse("2006-01-02", *dateToCheck)
			if innerErr != nil {
				return fmt.Errorf("invalid date format: %w", innerErr)
			}

			_, innerErr = s.financialRepo.GetFinancialYearByDate(ctx, parsedDate)
			if innerErr != nil {
				return fmt.Errorf("the date %s does not fall within an active financial year", parsedDate.Format("02-01-2006"))
			}
		}

		if innerErr = s.validateLockDate(ctx, tx, existing.PractitionerID, dateToCheck, &existing.CreatedAt); innerErr != nil {
			return innerErr
		}

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

		var valuesToUpdate []*FormEntryValue = nil
		if len(req.Values) > 0 {
			valuesToUpdate, innerErr = s.CalculateValues(ctx, existing.ID, req.Values)
			if innerErr != nil {
				return innerErr
			}
		}

		if innerErr = s.repo.Update(ctx, tx, existing, valuesToUpdate); innerErr != nil {
			return innerErr
		}

		// Handle document create/delete ops
		if req.Documents != nil {
			if err := s.handleDocumentLinks(ctx, tx, id, req.Documents); err != nil {
				return err
			}
			if err := s.handleDocumentUnlinks(ctx, tx, id, req.Documents); err != nil {
				return err
			}
		}

		updated, vals, innerErr := s.repo.GetByID(ctx, tx, id)
		if innerErr != nil {
			return fmt.Errorf("fetch entry after update: %w", innerErr)
		}
		if updated == nil {
			return fmt.Errorf("failed to retrieve updated form entry: %s", id)
		}

		result = updated.ToRs(vals)
		s.enrichResponseMetadata(ctx, result)
		s.attachDocuments(ctx, id, result)

		metaMap := events.JSONBMap{
			"entry_id":        result.ID.String(),
			"form_version_id": existing.FormVersionID.String(),
			"clinic_id":       existing.ClinicID.String(),
			"status":          result.Status,
		}

		s.recordSharedEvent(ctx, tx, existing.ClinicID, existing.FormVersionID, auditctx.ActionEntryUpdated, id,
			"Accountant %s updated entry for form: %s",
			metaMap,
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	meta := auditctx.GetMetadata(ctx)
	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionEntryUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  lo.ToPtr(auditctx.EntityFormFieldEntry),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  result,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return result, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return util.RunInTransaction(ctx, s.repo.(*Repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		existing, values, err := s.repo.GetByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("form entry not found: %s", id)
		}
		beforeState := existing.ToRs(values)

		if err := s.validateLockDate(ctx, tx, existing.ClinicID, existing.Date, &existing.CreatedAt); err != nil {
			return err
		}

		metaMap := events.JSONBMap{
			"entry_id":  existing.ID.String(),
			"clinic_id": existing.ClinicID.String(),
		}

		s.recordSharedEvent(ctx, tx, existing.ClinicID, existing.FormVersionID, auditctx.ActionEntryDeleted, id,
			"Accountant %s deleted an entry for form: %s",
			metaMap,
		)

		if err := s.repo.Delete(ctx, tx, id); err != nil {
			return err
		}

		meta := auditctx.GetMetadata(ctx)
		idStr := id.String()
		s.auditSvc.LogAsync(&audit.LogEntry{
			PracticeID:  meta.PracticeID,
			UserID:      meta.UserID,
			Action:      auditctx.ActionEntryDeleted,
			Module:      auditctx.ModuleForms,
			EntityType:  lo.ToPtr(auditctx.EntityFormFieldEntry),
			EntityID:    &idStr,
			BeforeState: beforeState,
			IPAddress:   meta.IPAddress,
			UserAgent:   meta.UserAgent,
		})

		return nil
	})
}

func (s *Service) List(ctx context.Context, formVersionID uuid.UUID, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error) {
	f := filter.MapToFilter()

	list, err := s.repo.ListByFormVersionID(ctx, formVersionID, f, actorID, role)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountByFormVersionID(ctx, formVersionID, f, actorID, role)
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

func (s *Service) GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByVersionID(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.ToRs(values), nil
}

func (s *Service) ListTransactions(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error) {
	f := filter.ToCommonFilter()

	items, err := s.repo.ListTransactions(ctx, f, actorID, role)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountTransactions(ctx, f, actorID, role)
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
		if err := v.Validate(); err != nil {
			return nil, err
		}

		if v.CoaID != nil && *v.CoaID != "" {
			coaID, err := uuid.Parse(*v.CoaID)
			if err != nil {
				return nil, fmt.Errorf("invalid coa_id: %w", err)
			}

			inputAmount := v.Amount
			if v.NetAmount != nil {
				inputAmount = *v.NetAmount
			}

			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: nil,
				CoaID:       &coaID,
				NetAmount:   &inputAmount,
				GstAmount:   nil,
				GrossAmount: &inputAmount,
				Description: v.Description,
			})
			continue
		}

		var fieldID uuid.UUID
		var err error
		if v.FormFieldID != nil && *v.FormFieldID != "" {
			fieldID, err = uuid.Parse(*v.FormFieldID)
			if err != nil {
				return nil, fmt.Errorf("invalid form_field_id: %w", err)
			}
		} else {
			return nil, fmt.Errorf("form_field_id is required for form-based entries")
		}

		f, err := s.fieldRepo.GetByID(ctx, fieldID)
		if err != nil {
			return nil, err
		}

		if f.IsComputed {
			continue
		}

		var inputAmount float64
		if v.NetAmount != nil {
			inputAmount = *v.NetAmount
		} else if v.GrossAmount != nil {
			inputAmount = *v.GrossAmount
		} else {
			inputAmount = v.Amount
		}

		var gstAmount *float64
		netBase := inputAmount
		grossTotal := inputAmount

		if f.TaxType == nil && v.GstAmount != nil && *v.GstAmount > 0 {
			gstAmount = v.GstAmount
			grossTotal = inputAmount
			netBase = inputAmount - *v.GstAmount

			keyValues[f.FieldKey] = netBase
			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: &fieldID,
				NetAmount:   &netBase,
				GstAmount:   gstAmount,
				GrossAmount: &grossTotal,
			})
			continue
		}

		if f.TaxType == nil {
			keyValues[f.FieldKey] = netBase
			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: &fieldID,
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
			gstAmount = v.GstAmount

			// MANUAL tax type: User enters GROSS + GST
			// CREATE: net_amount contains GROSS, gst_amount contains GST
			// UPDATE: gross_amount contains GROSS, net_amount contains pre-calculated NET, gst_amount contains GST

			if v.GrossAmount != nil {
				// UPDATE case: explicit gross_amount provided
				grossTotal = *v.GrossAmount
				if v.GstAmount != nil {
					netBase = *v.GrossAmount - *v.GstAmount
				} else {
					netBase = *v.GrossAmount
				}
			} else {
				// CREATE case: net_amount contains GROSS (misleading field name)
				grossTotal = inputAmount
				if v.GstAmount != nil {
					netBase = inputAmount - *v.GstAmount
				} else {
					netBase = inputAmount
				}
			}

		case method.TaxTreatmentZero:
			gstAmount = nil
			netBase = inputAmount
			grossTotal = inputAmount

		default:
			return nil, fmt.Errorf("unsupported tax treatment: %s", taxType)
		}

		valueForFormula := netBase
		if taxType == method.TaxTreatmentManual {
			valueForFormula = grossTotal
		} else if f.SectionType != nil && *f.SectionType == "OTHER_COST" {
			valueForFormula = grossTotal
		}
		keyValues[f.FieldKey] = valueForFormula
		taxTypeByKey[f.FieldKey] = string(taxType)
		out = append(out, &FormEntryValue{
			ID:          uuid.New(),
			EntryID:     entryID,
			FormFieldID: &fieldID,
			NetAmount:   &netBase,
			GstAmount:   gstAmount,
			GrossAmount: &grossTotal,
		})
	}

	if s.formulaSvc != nil && len(rq) > 0 {
		var firstFieldID uuid.UUID
		var firstField *field.FormField
		for _, v := range rq {
			if v.FormFieldID != nil && *v.FormFieldID != "" {
				var err error
				firstFieldID, err = uuid.Parse(*v.FormFieldID)
				if err != nil {
					return nil, err
				}
				firstField, err = s.fieldRepo.GetByID(ctx, firstFieldID)
				if err != nil {
					return nil, err
				}
				break
			}
		}

		if firstField == nil {
			return out, nil
		}

		allFields, err := s.fieldRepo.ListByFormVersionID(ctx, firstField.FormVersionID)
		if err != nil {
			return nil, err
		}

		fieldByID := make(map[uuid.UUID]*field.FormField, len(allFields))
		for _, af := range allFields {
			fieldByID[af.ID] = af
		}

		sectionTotals := make(map[string]float64)
		for _, entryVal := range out {
			if entryVal.FormFieldID == nil {
				continue
			}

			f, ok := fieldByID[*entryVal.FormFieldID]
			if ok && f.SectionType != nil && *f.SectionType != "" && entryVal.NetAmount != nil {
				sectionKey := "SECTION:" + *f.SectionType
				sectionTotals[sectionKey] += *entryVal.NetAmount
			}
		}

		maps.Copy(keyValues, sectionTotals)

		for _, f := range allFields {
			if f.IsComputed && f.TaxType != nil && *f.TaxType != "" {
				taxTypeByKey[f.FieldKey] = *f.TaxType
			}
		}

		manualGSTByKey := make(map[string]float64)
		for _, v := range rq {
			if v.GstAmount == nil || v.FormFieldID == nil || *v.FormFieldID == "" {
				continue
			}
			fieldID, err := uuid.Parse(*v.FormFieldID)
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

		alreadyAdded := make(map[uuid.UUID]bool, len(out))
		for _, v := range out {
			if v.FormFieldID != nil {
				alreadyAdded[*v.FormFieldID] = true
			}
		}

		for fieldID, val := range computed {
			f, ok := fieldByID[fieldID]
			if !ok {
				continue
			}
			if alreadyAdded[fieldID] {
				continue
			}

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
					netBase = val          // Keep as NET
					grossTotal = val + gst // NET + GST = GROSS
				case method.TaxTreatmentExclusive:
					// Formula returns NET, calculate GST and GROSS from NET
					gst := val * 0.10
					gstAmount = &gst
					netBase = val          // Keep as NET
					grossTotal = val + gst // NET + GST = GROSS
				case method.TaxTreatmentManual:
					// For MANUAL tax type on computed fields:
					// Formula returns GROSS amount, extract NET by subtracting GST
					var entryGST *float64
					for _, v := range rq {
						if v.FormFieldID == nil || *v.FormFieldID == "" {
							continue
						}
						entryFieldID, err := uuid.Parse(*v.FormFieldID)
						if err != nil {
							continue
						}
						if entryFieldID == fieldID && v.GstAmount != nil {
							entryGST = v.GstAmount
							break
						}
					}

					// val is GROSS from formula
					grossTotal = val
					if entryGST == nil {
						// No GST provided
						gst := 0.0
						gstAmount = &gst
						netBase = val
					} else {
						// GST provided: net = gross - gst
						gstAmount = entryGST
						netBase = val - *entryGST
					}
				case method.TaxTreatmentZero:
					gstAmount = nil
					netBase = val
					grossTotal = val
				}
			}

			out = append(out, &FormEntryValue{
				ID:          uuid.New(),
				EntryID:     entryID,
				FormFieldID: &fieldID,
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
		if v.FormFieldID == nil {
			continue
		}

		f, err := s.fieldRepo.GetByID(ctx, *v.FormFieldID)
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

	meta := auditctx.GetMetadata(ctx)

	// Only act if the user is an Accountant
	if meta.UserType == nil || !strings.EqualFold(*meta.UserType, util.RoleAccountant) || meta.UserID == nil {
		return
	}

	actorUserID, _ := uuid.Parse(*meta.UserID)

	ver, err := s.versionSvc.GetByID(ctx, rs.FormVersionID)
	if err != nil {
		return
	}

	form, err := s.detailSvc.GetByID(ctx, ver.FormId, actorUserID, *meta.UserType)
	if err != nil || form.Method != "INDEPENDENT_CONTRACTOR" {
		return
	}

	fieldMap := make(map[uuid.UUID]*field.FormField, len(rs.Values))
	for _, v := range rs.Values {
		if v.FormFieldID == nil {
			continue
		}

		f, err := s.fieldRepo.GetByID(ctx, *v.FormFieldID)
		if err != nil {
			return
		}
		fieldMap[*v.FormFieldID] = f
	}

	var incomeSum, expenseSum, otherCostSum float64
	for _, v := range rs.Values {
		if v.FormFieldID == nil {
			continue
		}

		f, ok := fieldMap[*v.FormFieldID]
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

	ownerShare := 0.0
	if form.OwnerShare != nil {
		ownerShare = float64(*form.OwnerShare)
	}

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

// Helper to record shared events
func (s *Service) recordSharedEvent(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, formVersionID uuid.UUID, action string, entryID uuid.UUID, descriptionTemplate string, metadata events.JSONBMap) {
	meta := auditctx.GetMetadata(ctx)

	// Only act if the user is an Accountant
	if meta.UserType == nil || !strings.EqualFold(*meta.UserType, util.RoleAccountant) || meta.UserID == nil {
		return
	}

	actorUserID, err := uuid.Parse(*meta.UserID)
	if err != nil {
		return
	}

	formName := "Form"
	ver, err := s.versionSvc.GetByID(ctx, formVersionID)
	if err == nil && ver != nil {
		form, err := s.detailRepo.GetByID(ctx, ver.FormId)
		if err == nil && form != nil {
			formName = form.Name
		}
	}

	clinic, err := s.clinicRepo.GetClinicByID(ctx, tx, clinicID)
	if err != nil || clinic == nil {
		return
	}

	var accountantID uuid.UUID
	var fullName string

	accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
	if err == nil && accProfile != nil {
		accountantID = accProfile.ID
	} else {
		accountantID = actorUserID
	}

	user, err := s.authRepo.FindByID(ctx, actorUserID)
	if err == nil && user != nil {
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

func (s *Service) ListCoaEntries(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*util.RsList, error) {
	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActitionTransactionsGenerated,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityTransactions),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Transaction Report",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event
	if role == util.RoleAccountant {
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		var pID uuid.UUID
		if filter.PractitionerID != nil {
			pID = *filter.PractitionerID
		}

		_ = s.eventsSvc.Record(ctx, events.SharedEvent{
			ID:             uuid.New(),
			PractitionerID: pID,
			AccountantID:   actorID,
			ActorID:        userID,
			ActorName:      &fullName,
			ActorType:      role,
			EventType:      "transaction_report.generated",
			EntityType:     "REPORT",
			Description:    fmt.Sprintf("Accountant %s generated Transaction Report", fullName),
			Metadata:       events.JSONBMap{"report_type": "Transaction Report"},
			CreatedAt:      time.Now(),
		})
	}

	f := filter.ToCommonFilter()

	items, err := s.repo.ListCoaEntries(ctx, f, actorID, role)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCoaEntries(ctx, f, actorID, role)
	if err != nil {
		return nil, err
	}

	var rs util.RsList
	rs.MapToList(items, total, *f.Offset, *f.Limit)
	return &rs, nil
}

func (s *Service) ListCoaEntryDetails(ctx context.Context, coaID string, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error) {
	coaUUID, err := uuid.Parse(coaID)
	if err != nil {
		return nil, fmt.Errorf("invalid coa_id: %w", err)
	}

	coaName, err := s.repo.GetCoaNameByID(ctx, coaUUID)
	if err != nil {
		return nil, fmt.Errorf("find coa name: %w", err)
	}

	f := filter.ToCommonFilter()

	items, err := s.repo.ListCoaEntryDetails(ctx, coaName, f, actorID, role)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCoaEntryDetails(ctx, coaName, f, actorID, role)
	if err != nil {
		return nil, err
	}

	var rs util.RsList
	rs.MapToList(items, total, *f.Offset, *f.Limit)
	return &rs, nil
}

func (s *Service) ExportTransactionReport(ctx context.Context, f TransactionFilter, actorID uuid.UUID, role string, exportType string, userID uuid.UUID, PracIDs []uuid.UUID) (interface{}, string, error) {
	var result interface{}
	var contentType string
	var fullName string
	var targetNotifIDs []uuid.UUID

	err := util.RunInTransaction(ctx, s.repo.(*Repository).db, func(ctx context.Context, tx *sqlx.Tx) error {
		groups, err := s.repo.ListCoaEntries(ctx, f.ToCommonFilter(), actorID, role)
		if err != nil {
			return err
		}

		for _, g := range groups {
			if g == nil {
				continue
			}
			details, err := s.repo.ListCoaEntryDetails(ctx, g.CoaName, f.ToCommonFilter(), actorID, role)
			if err != nil {
				continue
			}
			g.Details = details
		}

		user, _ := s.authRepo.FindByID(ctx, userID)
		if user != nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		var entityName string
		var practitionerABN string
		if f.PractitionerID != nil && *f.PractitionerID != uuid.Nil {
			prac, err := s.practitionerSvc.GetPractitioner(ctx, *f.PractitionerID)
			if err == nil && prac != nil {
				if prac.EntityName != nil {
					entityName = *prac.EntityName
				} else {
					entityName = fullName
				}
				if prac.ABN != nil {
					practitionerABN = *prac.ABN
				}
			}
		} else {
			if role == util.RolePractitioner {
				prac, err := s.practitionerSvc.GetPractitioner(ctx, *f.PractitionerID)
				entityName = fullName
				if err == nil && prac != nil {
					if prac.ABN != nil {
						practitionerABN = *prac.ABN
					}
				}
			} else {
				acc, err := s.accountantRepo.GetAccountantByUserID(ctx, userID.String())
				if err == nil && acc != nil {
					if acc.EntityName != nil {
						entityName = *acc.EntityName
					} else {
						entityName = fullName
					}
					if acc.ABN != nil {
						practitionerABN = *acc.ABN
					}
				}
			}
		}

		formatDateHelper := func(dateStr string) string {
			if dateStr == "" || dateStr == "<nil>" {
				return "-"
			}
			input := dateStr
			if len(dateStr) >= 10 {
				input = dateStr[:10]
			}
			t, err := time.Parse("2006-01-02", input)
			if err != nil {
				return dateStr
			}
			return t.Format("02-01-2006")
		}

		var period string
		if f.StartDate != nil && *f.StartDate != "" && f.EndDate != nil && *f.EndDate != "" {
			period = fmt.Sprintf("%s to %s", formatDateHelper(*f.StartDate), formatDateHelper(*f.EndDate))
		} else if f.EndDate != nil && *f.EndDate != "" {
			period = fmt.Sprintf("As of %s", formatDateHelper(*f.EndDate))
		}

		// Handle Excel Export
		if strings.ToLower(exportType) == "excel" {
			buf, err := s.generateExcelReport(ctx, f, actorID, role, entityName, practitionerABN, period)
			if err != nil {
				return fmt.Errorf("failed to generate excel: %w", err)
			}
			result = buf
			contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}

		// Resolve target practitioners for Notifications
		targetNotifIDs = PracIDs

		// If clinic_id is provided, look up the owner
		if f.ClinicID != nil {
			clinicUUID, err := uuid.Parse(*f.ClinicID)
			if err == nil {
				clinic, err := s.clinicRepo.GetClinicByID(ctx, tx, clinicUUID)
				if err == nil && clinic != nil {
					targetNotifIDs = []uuid.UUID{clinic.PractitionerID}
				}
			}
		}

		if role == util.RoleAccountant {
			for _, pID := range targetNotifIDs {
				err := s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: pID,
					AccountantID:   actorID,
					ActorID:        userID,
					ActorName:      &fullName,
					ActorType:      role,
					EventType:      "transaction_report.exported",
					EntityType:     "REPORT",
					Description:    fmt.Sprintf("Accountant %s exported Transaction Report", fullName),
					Metadata:       events.JSONBMap{"report_type": "Transaction Report", "export_type": exportType},
					CreatedAt:      time.Now(),
				})
				if err != nil {
					return fmt.Errorf("failed to log shared transaction export event: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, "", err
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	userIDStr := userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActitionTransactionsExported,
		Module:     auditctx.ModuleReport,
		EntityType: lo.ToPtr(auditctx.EntityTransactions),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Transaction Report",
			"export_type": exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	return result, contentType, nil
}

func (s *Service) generateExcelReport(ctx context.Context, f TransactionFilter, actorID uuid.UUID, role string, entityName string, practitionerABN string, period string) (*bytes.Buffer, error) {
	groups, err := s.repo.ListCoaEntries(ctx, f.ToCommonFilter(), actorID, role)
	if err != nil {
		return nil, err
	}

	formatDate := func(dateStr string) string {
		if len(dateStr) < 10 {
			return dateStr
		}
		t, err := time.Parse("2006-01-02", dateStr[:10])
		if err != nil {
			return dateStr
		}
		return t.Format("02-01-2006")
	}

	// --- FETCH METADATA ---

	xl := excelize.NewFile()
	defer xl.Close()
	sheet := "Transactions"
	xl.SetSheetName("Sheet1", sheet)

	// Define the width of the report (Columns A through I)
	lastCol := "I"

	// 1. Define Styles
	styleHeaderBlue, _ := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
	})

	headerStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4EA7B3"}, Pattern: 1},
	})
	groupHeaderStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DAEEF3"}, Pattern: 1},
	})

	normalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		CustomNumFmt: lo.ToPtr("$#,##0.00"),
	})

	// Bold style for the bottom total row
	totalRowStyle, _ := xl.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E1E1E1"}, Pattern: 1},
	})
	totalCurrencyStyle, _ := xl.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1},
		CustomNumFmt: lo.ToPtr("$#,##0.00"),
	})

	// Helpers to handle Pointers and Nils (Fixes 0xc000 and <nil> issues)
	getFloat := func(f *float64) float64 {
		if f == nil {
			return 0.0
		}
		return *f
	}
	getString := func(s *string) string {
		// If the pointer is nil, or the dereferenced string is empty or "<nil>"
		if s == nil || *s == "" || *s == "<nil>" {
			return "-"
		}
		return *s
	}

	// --- 1. RENDER METADATA ---
	// Row 1: Title
	xl.MergeCell(sheet, "A1", lastCol+"1")
	xl.SetCellValue(sheet, "A1", "Transaction Report")
	xl.SetCellStyle(sheet, "A1", "A1", styleHeaderBlue)

	setRichMeta := func(row int, label, value string) {
		cell := fmt.Sprintf("A%d", row)
		xl.MergeCell(sheet, cell, lastCol+strconv.Itoa(row))
		xl.SetCellRichText(sheet, cell, []excelize.RichTextRun{
			{Text: label, Font: &excelize.Font{Bold: true, Family: "Calibri", Size: 10}},
			{Text: " " + value, Font: &excelize.Font{Bold: false, Family: "Calibri", Size: 10}},
		})
	}

	metaRow := 2

	// Exported By (Always show)
	setRichMeta(metaRow, "Exported by:", entityName)
	metaRow++

	// ABN (Skip if empty)
	if practitionerABN != "" {
		setRichMeta(metaRow, "ABN:", practitionerABN)
		metaRow++
	}

	// Period (Skip if nil/empty)
	if period != "" {
		setRichMeta(metaRow, "Period:", period)
		metaRow++
	}

	currentTimeStr := time.Now().Format("02/01/2006, 3:04:05 pm")
	setRichMeta(metaRow, "Generated:", currentTimeStr)
	metaRow++

	// 2. Set Headers
	headerRow := metaRow + 1
	headers := []string{"Date", "Account / Field", "Tax Type", "Form", "Clinic", "Net Amount", "GST Amount", "Gross Amount", "Type"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		xl.SetCellValue(sheet, cell, h)
	}
	xl.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("I%d", headerRow), headerStyle)

	currRow := headerRow + 1
	for _, g := range groups {
		// --- 4. GROUP HEADER ---
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), g.CoaName)
		xl.MergeCell(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow))
		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow), groupHeaderStyle)
		currRow++

		details, err := s.repo.ListCoaEntryDetails(ctx, g.CoaName, f.ToCommonFilter(), actorID, role)
		if err != nil {
			continue
		}

		// --- 5. INDIVIDUAL TRANSACTIONS ---
		for _, d := range details {
			xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), formatDate(d.CreatedAt))
			xl.SetCellValue(sheet, fmt.Sprintf("B%d", currRow), "  "+d.FormFieldName)
			xl.SetCellValue(sheet, fmt.Sprintf("C%d", currRow), getString(d.TaxTypeName))
			xl.SetCellValue(sheet, fmt.Sprintf("D%d", currRow), getString(d.FormName))
			xl.SetCellValue(sheet, fmt.Sprintf("E%d", currRow), getString(d.ClinicName))

			xl.SetCellValue(sheet, fmt.Sprintf("F%d", currRow), getFloat(d.NetAmount))
			xl.SetCellValue(sheet, fmt.Sprintf("G%d", currRow), getFloat(d.GstAmount))
			xl.SetCellValue(sheet, fmt.Sprintf("H%d", currRow), getFloat(d.GrossAmount))

			// Apply currency formatting to F, G, H columns
			xl.SetCellStyle(sheet, fmt.Sprintf("F%d", currRow), fmt.Sprintf("H%d", currRow), normalCurrencyStyle)

			entryType := "Entry"
			if d.IsExpense {
				entryType = "Expense"
			}
			xl.SetCellValue(sheet, fmt.Sprintf("I%d", currRow), entryType)
			currRow++
		}

		// --- 6. TOTAL ROW ---
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), "Total "+g.CoaName)
		xl.SetCellValue(sheet, fmt.Sprintf("F%d", currRow), g.TotalNetAmount)
		xl.SetCellValue(sheet, fmt.Sprintf("H%d", currRow), g.TotalGrossAmount)

		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow), totalRowStyle)
		xl.SetCellStyle(sheet, fmt.Sprintf("F%d", currRow), fmt.Sprintf("F%d", currRow), totalCurrencyStyle)
		xl.SetCellStyle(sheet, fmt.Sprintf("H%d", currRow), fmt.Sprintf("H%d", currRow), totalCurrencyStyle)

		currRow += 2 // Gap between groups
	}

	// Add AutoFilter to the header row (A to I)
	if err := xl.AutoFilter(sheet, fmt.Sprintf("A%d:I%d", headerRow, headerRow), nil); err != nil {
		return nil, err
	}

	// Column Widths
	xl.SetColWidth(sheet, "A", "A", 15) // Date
	xl.SetColWidth(sheet, "B", "B", 35) // Account
	xl.SetColWidth(sheet, "C", "E", 20) // Tax, Form, Clinic
	xl.SetColWidth(sheet, "F", "H", 15) // Amounts
	xl.SetColWidth(sheet, "I", "I", 12) // Type

	return xl.WriteToBuffer()
}

func (s *Service) validateLockDate(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, entryDate *string, createdAt *string) error {

	var dateString string
	if entryDate != nil && *entryDate != "" {
		dateString = *entryDate
	} else if createdAt != nil && *createdAt != "" {
		// Guard against short string index out of range panics
		if len(*createdAt) < 10 {
			return fmt.Errorf("malformed createdAt timestamp format: %s", *createdAt)
		}
		dateString = (*createdAt)[:10]
	} else {
		return nil
	}

	financialSettings, err := s.clinicRepo.GetFinancialSettings(ctx, tx, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to get practitioner financial settings: %w", err)
	}

	if financialSettings == nil || financialSettings.LockDate == nil {
		return nil
	}

	parsedEntryDate, err := time.Parse("2006-01-02", dateString)
	if err != nil {
		return fmt.Errorf("invalid entry date format: %w", err)
	}

	lockDate := financialSettings.LockDate.UTC().Truncate(24 * time.Hour)
	entryDateOnly := parsedEntryDate.UTC().Truncate(24 * time.Hour)

	if entryDateOnly.Before(lockDate) || entryDateOnly.Equal(lockDate) {
		return fmt.Errorf("cannot modify entries on or before the lock date (%s)",
			lockDate.Format("2006-01-02"))
	}

	return nil
}

type CoaGroup struct {
	CoaID            string       `json:"coa_id"`
	CoaName          string       `json:"coa_name"`
	TotalNetAmount   float64      `json:"total_net_amount"`
	TotalGrossAmount float64      `json:"total_gross_amount"`
	Details          []*CoaDetail `json:"details"`
}

type CoaDetail struct {
	FormFieldName string    `json:"form_field_name"`
	TaxTypeName   *string   `json:"tax_type_name"`
	FormName      string    `json:"form_name"`
	ClinicName    string    `json:"clinic_name"`
	NetAmount     *float64  `json:"net_amount"`
	GstAmount     *float64  `json:"gst_amount"`
	GrossAmount   *float64  `json:"gross_amount"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) ExportTransactionData(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*RsExportData, error) {
	f := filter.ToCommonFilter()

	f.Limit = lo.ToPtr(1000)
	coaSummaries, err := s.repo.ListCoaEntries(ctx, f, actorID, role)
	if err != nil {
		return nil, err
	}

	exportItems := make([]*RsCoaExportItem, 0, len(coaSummaries))

	for _, summary := range coaSummaries {
		details, err := s.repo.ListCoaEntryDetails(ctx, summary.CoaName, f, actorID, role)
		if err != nil {
			log.Printf("Error fetching details for COA %s: %v", summary.CoaName, err)
			continue
		}

		exportItems = append(exportItems, &RsCoaExportItem{
			CoaID:            summary.CoaID,
			CoaName:          summary.CoaName,
			TotalNetAmount:   summary.TotalNetAmount,
			TotalGstAmount:   summary.TotalGSTAmount,
			TotalGrossAmount: summary.TotalGrossAmount,
			EntryCount:       summary.EntryCount,
			Entries:          details,
		})
	}

	return &RsExportData{Items: exportItems}, nil
}

// enrichResponseMetadata attaches field metadata and IC calculations to the response
func (s *Service) enrichResponseMetadata(ctx context.Context, rs *RsFormEntry) {
	s.attachFieldMetadata(ctx, rs)
	s.attachICCalculation(ctx, rs)
}

// attachDocuments fetches and attaches document details to a response entry
func (s *Service) attachDocuments(ctx context.Context, entryID uuid.UUID, rs *RsFormEntry) {
	docs, err := s.repo.GetDocumentsByEntryID(ctx, entryID)
	if err != nil || docs == nil {
		return
	}
	rs.Documents = make([]RsEntryDocument, 0, len(docs))
	for _, d := range docs {
		if d != nil {
			rs.Documents = append(rs.Documents, *d)
		}
	}
}

// handleDocumentLinks processes and links documents for an entry
func (s *Service) handleDocumentLinks(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, docs *RqDocument) error {
	if docs == nil || len(docs.Create) == 0 {
		return nil
	}
	docIDs, err := util.ParseUUIDs(docs.Create)
	if err != nil {
		return fmt.Errorf("invalid document id: %w", err)
	}
	if err := s.repo.LinkDocuments(ctx, tx, entryID, docIDs); err != nil {
		return fmt.Errorf("link documents: %w", err)
	}
	return nil
}

// handleDocumentUnlinks processes and unlinks documents from an entry
func (s *Service) handleDocumentUnlinks(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, docs *RqDocument) error {
	if docs == nil || len(docs.Delete) == 0 {
		return nil
	}
	docIDs, err := util.ParseUUIDs(docs.Delete)
	if err != nil {
		return fmt.Errorf("invalid document id: %w", err)
	}
	if err := s.repo.UnlinkDocuments(ctx, tx, entryID, docIDs); err != nil {
		return fmt.Errorf("unlink documents: %w", err)
	}
	return nil
}
