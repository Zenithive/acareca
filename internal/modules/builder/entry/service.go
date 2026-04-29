package entry

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
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
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
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

	//ExportTransactionReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*bytes.Buffer, error)
	ExportTransactionReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, exportType string, userID uuid.UUID, PracIDs []uuid.UUID) (interface{}, string, error)
	generateExcelReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*bytes.Buffer, error)
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
	invitationSvc  invitation.Service
	detailRepo     detail.IRepository
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService, detailSvc detail.IService, versionSvc version.IService, auditSvc audit.Service, eventsSvc events.Service, accRepo accountant.Repository, authRepo auth.Repository, clinicRepo clinic.Repository, clinicSvc clinic.Service, formulaSvc formula.IService, fieldSvc field.IService, invitationSvc invitation.Service, detailRepo detail.IRepository) IService {
	return &Service{repo: repo, fieldRepo: fieldRepo, methodSvc: methodSvc, limitsSvc: limits.NewService(db), detailSvc: detailSvc, versionSvc: versionSvc, auditSvc: auditSvc, formulaSvc: formulaSvc, eventsSvc: eventsSvc, accountantRepo: accRepo, authRepo: authRepo, clinicRepo: clinicRepo, formClinic: clinicSvc, fieldSvc: fieldSvc, invitationSvc: invitationSvc, detailRepo: detailRepo}
}

// Create implements [IService].
func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID) (*RsFormEntry, error) {
	meta := auditctx.GetMetadata(ctx)
	// Permission checks are handled by middleware

	// Resolve the REAL owner at the start of THIS function
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, req.ClinicID)
	if err != nil {
		return nil, err
	}

	realOwnerID := clinic.PractitionerID

	// Validate lock date before creating entry
	if err := s.validateLockDate(ctx, realOwnerID, req.Date, nil); err != nil {
		return nil, err
	}

	if err := s.limitsSvc.Check(ctx, realOwnerID, limits.KeyTransactionCreate); err != nil {
		return nil, err
	}

	// // Resolve the FormID to check permissions
	// version, err := s.versionSvc.GetByID(ctx, formVersionID)
	// if err != nil {
	// 	return nil, fmt.Errorf("invalid version: %w", err)
	// }

	// // PERMISSION CHECK (Accountant Only)
	// if strings.EqualFold(role, util.RoleAccountant) {
	// 	perms, err := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, version.FormId)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	// Must have 'create' or 'all'
	// 	if perms == nil || (!perms.HasAccess("create") && !perms.HasAccess("all")) {
	// 		return nil, errors.New("Access denied: you do not have permission to create entries for this form")
	// 	}
	// }

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
	// Permission checks are handled by middleware
	e, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Resolve the Form ID via Version ID
	// formVersion, err := s.versionSvc.GetByID(ctx, e.FormVersionID)
	// if err != nil {
	// 	return nil, err
	// }
	// if strings.EqualFold(role, util.RoleAccountant) {
	// 	// First, check if there's a specific permission for this ENTRY ID
	// 	entryPerms, err := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, id)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("auth error: %w", err)
	// 	}

	// 	// Fallback: If no entry perms, check the PARENT FORM permissions
	// 	if entryPerms == nil {
	// 		formPerms, err := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, formVersion.FormId)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("auth error: %w", err)
	// 		}

	// 		// If no form perms either, block access entirely
	// 		if formPerms == nil || (!formPerms.HasAccess("read") && !formPerms.HasAccess("all")) {
	// 			return nil, errors.New("Access denied: no permission found for this entry or its parent form")
	// 		}
	// 		// SUCCESS: No specific entry perms, but has form-level read access. Allow read-only access.
	// 	} else {
	// 		// SUCCESS: Found specific Entry perms. Check for read access.
	// 		if !entryPerms.HasAccess("read") && !entryPerms.HasAccess("all") {
	// 			return nil, errors.New("Access denied: you do not have permission to view this entry")
	// 		}
	// 	}
	// }

	rs := e.ToRs(values)
	s.attachFieldMetadata(ctx, rs)
	s.attachICCalculation(ctx, rs)
	return rs, nil
}

// Update implements [IService].
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID) (*RsFormEntry, error) {
	// Permission checks are handled by middleware

	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	beforeState := existing.ToRs(values)

	// Validate lock date - check both existing date and new date if provided
	dateToCheck := existing.Date
	if req.Date != nil {
		dateToCheck = req.Date
	}
	if err := s.validateLockDate(ctx, existing.PractitionerID, dateToCheck, &existing.CreatedAt); err != nil {
		return nil, err
	}

	// PERMISSION CHECK (Accountant Only)
	// if strings.EqualFold(role, util.RoleAccountant) {
	// 	entryPerms, _ := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, id)

	// 	// Must have 'update' OR 'all'
	// 	if entryPerms == nil || (!entryPerms.HasAccess("update") && !entryPerms.HasAccess("all")) {
	// 		return nil, errors.New("Access denied: you do not have permission to update this entry")
	// 	}
	// }

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
	// Permission checks are handled by middleware

	// Get entry details before deletion for audit log
	existing, values, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	beforeState := existing.ToRs(values)

	// Validate lock date before deleting entry
	if err := s.validateLockDate(ctx, existing.ClinicID, existing.Date, &existing.CreatedAt); err != nil {
		return err
	}

	// PERMISSION CHECK (Accountant Only)
	// if strings.EqualFold(role, util.RoleAccountant) {
	// 	entryPerms, _ := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, id)

	// 	// Must have 'delete' OR 'all'
	// 	if entryPerms == nil || (!entryPerms.HasAccess("delete") && !entryPerms.HasAccess("all")) {
	// 		return errors.New("Access denied: you do not have permission to delete this entry")
	// 	}
	// }

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
func (s *Service) List(ctx context.Context, formVersionID uuid.UUID, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error) {
	// Permission checks are handled by middleware
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

// GetByVersionID implements [IService].
func (s *Service) GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error) {
	e, values, err := s.repo.GetByVersionID(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.ToRs(values), nil
}

// ListTransactions implements [IService].
func (s *Service) ListTransactions(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error) {
	// Permission checks are handled by middleware
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
		sectionTotals := make(map[string]float64)
		for _, entryVal := range out {
			f, ok := fieldByID[entryVal.FormFieldID]
			if ok && f.SectionType != nil && *f.SectionType != "" && entryVal.NetAmount != nil {
				sectionKey := "SECTION:" + *f.SectionType
				// Always use NET amount for section totals (matching LiveCalculate logic)
				sectionTotals[sectionKey] += *entryVal.NetAmount
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
					netBase = val          // Keep as NET
					grossTotal = val + gst // NET + GST = GROSS
				case method.TaxTreatmentExclusive:
					// Formula returns NET, calculate GST and GROSS from NET
					gst := val * 0.10
					gstAmount = &gst
					netBase = val          // Keep as NET
					grossTotal = val + gst // NET + GST = GROSS
				case method.TaxTreatmentManual:
					// For MANUAL tax type on computed fields, check if GST was provided in request
					var entryGST *float64
					for _, v := range rq {
						entryFieldID, _ := uuid.Parse(v.FormFieldID)
						if entryFieldID == fieldID && v.GstAmount != nil {
							entryGST = v.GstAmount
							break
						}
					}

					// If GST amount is empty or zero, send net with gst=0, gross=net
					if entryGST == nil {
						gst := 0.0
						gstAmount = &gst
						netBase = val
						grossTotal = val
					} else {
						// If GST provided, send net=net, gst=entry.gst, gross=net+gst
						gstAmount = entryGST
						netBase = val
						grossTotal = val + *entryGST
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
		form, err := s.detailRepo.GetByID(ctx, ver.FormId)
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

// ListCoaEntries implements [IService] - returns grouped COA rows for parent grid
func (s *Service) ListCoaEntries(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*util.RsList, error) {
	var targetPracIDs []uuid.UUID
	if role == util.RolePractitioner {
		// Practitioner logic: Only them
		pracIdStr := actorID.String()
		filter.PractitionerID = &pracIdStr
		targetPracIDs = []uuid.UUID{actorID}
	} else if role == util.RoleAccountant {
		// Accountant logic:
		if filter.PractitionerID != nil && *filter.PractitionerID != "" {
			// Case A: Specific practitioner selected in query
			pID, err := uuid.Parse(*filter.PractitionerID)
			if err == nil {
				targetPracIDs = []uuid.UUID{pID}
			}
		} else {
			// Case B: No practitioner specified - fetch all linked practitioners
			linked, err := s.invitationSvc.GetPractitionersLinkedToAccountant(ctx, actorID)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch linked practitioners: %w", err)
			}
			targetPracIDs = linked
		}
	}

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
		EntityType: strPtr(auditctx.EntityTransactions),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Transaction Report",
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event
	if role == util.RoleAccountant {
		// Fetching user details
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		for _, pID := range targetPracIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorType:      role,
				EventType:      "transaction_report.generated",
				EntityType:     "REPORT",
				Description:    fmt.Sprintf("Accountant %s generated Transaction Report", fullName),
				Metadata:       events.JSONBMap{"report_type": "Transaction Report"},
				CreatedAt:      time.Now(),
			})
		}
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

// ListCoaEntryDetails implements [IService] - returns entry details for a specific COA (child grid)
func (s *Service) ListCoaEntryDetails(ctx context.Context, coaID string, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error) {
	coaUUID, err := uuid.Parse(coaID)
	if err != nil {
		return nil, fmt.Errorf("invalid coa_id: %w", err)
	}

	f := filter.ToCommonFilter()

	items, err := s.repo.ListCoaEntryDetails(ctx, coaUUID, f, actorID, role)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCoaEntryDetails(ctx, coaUUID, f, actorID, role)
	if err != nil {
		return nil, err
	}

	var rs util.RsList
	rs.MapToList(items, total, *f.Offset, *f.Limit)
	return &rs, nil
}

func (s *Service) ExportTransactionReport(ctx context.Context, f TransactionFilter, actorID uuid.UUID, role string, exportType string, userID uuid.UUID, PracIDs []uuid.UUID) (interface{}, string, error) {
	// Fetch Shared Data
	groups, err := s.repo.ListCoaEntries(ctx, f.ToCommonFilter(), actorID, role)
	if err != nil {
		return nil, "", err
	}

	for _, g := range groups {
		coaUUID, _ := uuid.Parse(g.CoaID)
		details, err := s.repo.ListCoaEntryDetails(ctx, coaUUID, f.ToCommonFilter(), actorID, role)
		if err != nil {
			continue
		}
		g.Details = details
	}

	var result interface{}
	var contentType string

	// Handle HTML Export
	if strings.ToLower(exportType) == "pdf" {
		data := struct{ Groups interface{} }{Groups: groups}
		htmlContent, err := s.generateTransactionHTML(data)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate html: %w", err)
		}
		result = htmlContent
		contentType = "text/html"
	} else {
		// Handle Excel Export
		buf, err := s.generateExcelReport(ctx, f, actorID, role)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate excel: %w", err)
		}
		result = buf
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	// Resolve target practitioners for Notifications
	targetNotifIDs := PracIDs

	// If clinic_id is provided, we narrow down the notification to ONLY the owner of that clinic
	if f.ClinicID != nil {
		clinicUUID, err := uuid.Parse(*f.ClinicID)
		if err == nil {
			// Get the clinic to find the practitioner_id
			clinic, err := s.clinicRepo.GetClinicByID(ctx, clinicUUID)
			if err == nil {
				// Set the notification target to just this owner
				targetNotifIDs = []uuid.UUID{clinic.PractitionerID}
			}
		}
	}

	// --- AUDIT LOG ---
	meta := auditctx.GetMetadata(ctx)
	var userIDStr string
	userIDStr = userID.String()
	parsedActorID := actorID.String()

	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: nil,
		UserID:     &userIDStr,
		Action:     auditctx.ActitionTransactionsExported,
		Module:     auditctx.ModuleReport,
		EntityType: strPtr(auditctx.EntityTransactions),
		EntityID:   &parsedActorID,
		AfterState: map[string]interface{}{
			"report_type": "Transaction Report",
			"export_type": exportType,
		},
		IPAddress: meta.IPAddress,
		UserAgent: meta.UserAgent,
	})

	// Record the Shared Event
	if role == util.RoleAccountant {
		// Fetching user details
		var fullName string
		user, err := s.authRepo.FindByID(ctx, userID)
		if err == nil {
			fullName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}

		for _, pID := range targetNotifIDs {
			_ = s.eventsSvc.Record(ctx, events.SharedEvent{
				ID:             uuid.New(),
				PractitionerID: pID,
				AccountantID:   actorID,
				ActorID:        userID,
				ActorType:      role,
				EventType:      "transaction_report.exported",
				EntityType:     "REPORT",
				Description:    fmt.Sprintf("Accountant %s exported Transaction Report", fullName),
				Metadata:       events.JSONBMap{"report_type": "Transaction Report", "export_type": exportType},
				CreatedAt:      time.Now(),
			})
		}
	}

	return result, contentType, nil
}

func (s *Service) generateExcelReport(ctx context.Context, f TransactionFilter, actorID uuid.UUID, role string) (*bytes.Buffer, error) {
	groups, err := s.repo.ListCoaEntries(ctx, f.ToCommonFilter(), actorID, role)
	if err != nil {
		return nil, err
	}

	xl := excelize.NewFile()
	defer xl.Close()
	sheet := "Transactions"
	xl.SetSheetName("Sheet1", sheet)

	// 1. Define Styles
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

	// Format date to YYYY-MM-DD
	formatDate := func(dateStr string) string {
		if dateStr == "" || dateStr == "<nil>" {
			return "-"
		}

		// Try to parse the standard ISO format
		t, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Fallback: If it's already a simple date string "2026-04-27"
			t, err = time.Parse("2006-01-02", strings.Split(dateStr, "T")[0])
			if err != nil {
				return dateStr // Return raw string if parsing fails
			}
		}
		return t.Format("2006-01-02")
	}

	// 2. Set Headers
	headers := []string{"Date", "Account / Field", "Tax Type", "Form", "Clinic", "Net Amount", "GST Amount", "Gross Amount", "Type"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		xl.SetCellValue(sheet, cell, h)
	}
	xl.SetCellStyle(sheet, "A1", "I1", headerStyle)

	currRow := 2
	for _, g := range groups {
		// --- 4. GROUP HEADER ---
		xl.SetCellValue(sheet, fmt.Sprintf("A%d", currRow), g.CoaName)
		xl.MergeCell(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow))
		xl.SetCellStyle(sheet, fmt.Sprintf("A%d", currRow), fmt.Sprintf("I%d", currRow), groupHeaderStyle)
		currRow++

		coaUUID, _ := uuid.Parse(g.CoaID)
		details, err := s.repo.ListCoaEntryDetails(ctx, coaUUID, f.ToCommonFilter(), actorID, role)
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

	// Add AutoFilter to the header row (A1 to I1)
	if err := xl.AutoFilter(sheet, "A1:I1", nil); err != nil {
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

func (s *Service) validateLockDate(ctx context.Context, practitionerID uuid.UUID, entryDate *string, createdAt *string) error {

	var dateString string
	if entryDate != nil && *entryDate != "" {
		dateString = *entryDate
	} else if createdAt != nil && *createdAt != "" {

		dateString = (*createdAt)[:10]
	} else {

		return nil
	}

	// 2. Fetch settings based on PractitionerID instead of ClinicID
	financialSettings, err := s.clinicRepo.GetFinancialSettings(ctx, practitionerID)
	if err != nil {
		return fmt.Errorf("failed to get practitioner financial settings: %w", err)
	}

	// 3. Check if settings or lock date exist
	if financialSettings == nil || financialSettings.LockDate == nil {
		return nil
	}

	// 4. Parse the entry date
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

const reportTemplate = `
<html>
<head>
<style>
    @page {
        size: A4 landscape;
        margin: 1cm;
    }

    body { 
        font-family: 'sans-serif; 
        font-size: 12pt; 
        color: #000;
        margin: 0;
    }

    table { 
        width: 100%; 
        border-collapse: collapse; 
        table-layout: fixed; 
        margin-bottom: 30px;
    }

    th, td { 
        border: 1px solid #d1d1d1; 
        padding: 8px 6px;
        word-wrap: break-word;
        vertical-align: middle;
    }
    
    th { 
        background-color: #4EA7B3; 
        color: white; 
        text-align: center; 
        font-weight: bold;
        font-size: 14pt; 
    }
    
    .group-row { 
        background-color: #DAEEF3; 
        font-weight: bold; 
        color: #2A5D63;
        font-size: 13pt;
    }
    
    /* Total Rows: Bold, Gray Background, Size 12pt */
    .total-row { 
        background-color: #E1E1E1; 
        font-weight: bold;
        font-size: 12pt;
    }
    
    .amount { text-align: right; }
    .date-cell { text-align: center; }

    /* Column Widths */
    .col-date { width: 12%; }
    .col-acct { width: 20%; }
    .col-tax  { width: 10%; }
    .col-form { width: 15%; }
    .col-clinic { width: 15%; }
    .col-amt  { width: 9%; }
    .col-type  { width: 10%; }
</style>
</head>
<body>
    <table>
        <thead>
            <tr>
                <th class="col-date">Date</th>
                <th class="col-acct">Account / Field</th>
                <th class="col-tax">Tax Type</th>
                <th class="col-form">Form</th>
                <th class="col-clinic">Clinic</th>
                <th class="col-amt">Net</th>
                <th class="col-amt">GST</th>
                <th class="col-amt">Gross</th>
                <th class="col-type">Type</th>
            </tr>
        </thead>
        <tbody>
            {{range .Groups}}
                <tr class="group-row">
                    <td colspan="9">{{.CoaName}}</td>
                </tr>
                {{range .Details}}
                <tr>
                    <td class="date-cell">{{formatDate .CreatedAt}}</td>
                    <td style="padding-left: 20px;">{{.FormFieldName}}</td>
                    <td>{{.TaxTypeName}}</td>
                   	<td>{{if .FormName}}{{.FormName}}{{else}}-{{end}}</td>
					<td>{{if .ClinicName}}{{.ClinicName}}{{else}}-{{end}}</td>
                    <td class="amount">${{getFloat .NetAmount | printf "%.2f"}}</td>
                    <td class="amount">${{getFloat .GstAmount | printf "%.2f"}}</td>
                    <td class="amount">${{getFloat .GrossAmount | printf "%.2f"}}</td>
					<td>{{if .IsExpense}}Expense{{else}}Entry{{end}}</td>
                </tr>
                {{end}}
                <tr class="total-row">
                    <td colspan="5" style="text-align: left; padding-left: 10px;">Total {{.CoaName}}</td>
                    <td class="amount">${{.TotalNetAmount | printf "%.2f"}}</td>
                    <td class="amount"></td>
                    <td class="amount">${{.TotalGrossAmount | printf "%.2f"}}</td>
					<td></td>
                </tr>
                <tr style="border: none; height: 20px;"><td colspan="9" style="border: none;"></td></tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>
`

type CoaGroup struct {
	CoaID            string       `json:"coa_id"`
	CoaName          string       `json:"coa_name"`
	TotalNetAmount   float64      `json:"total_net_amount"`
	TotalGrossAmount float64      `json:"total_gross_amount"`
	Details          []*CoaDetail `json:"details"` // We will nest the details here
}

// CoaDetail represents the individual transaction lines
type CoaDetail struct {
	FormFieldName string    `json:"form_field_name"`
	TaxTypeName   *string   `json:"tax_type_name"` // Pointer to handle nulls
	FormName      string    `json:"form_name"`
	ClinicName    string    `json:"clinic_name"`
	NetAmount     *float64  `json:"net_amount"`
	GstAmount     *float64  `json:"gst_amount"`
	GrossAmount   *float64  `json:"gross_amount"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) generateTransactionHTML(data interface{}) (string, error) {
	tmpl, err := template.New("pdf").Funcs(template.FuncMap{
		"getFloat": func(f *float64) float64 {
			if f == nil {
				return 0.0
			}
			return *f
		},
		// Helper to format strings or time objects from specific format
		"formatDate": func(t interface{}) string {
			switch v := t.(type) {
			case time.Time:
				return v.Format("2006-01-02")
			case string:
				// If it's a full timestamp like "2026-04-20T10:00:00Z", just take the date part
				if len(v) >= 10 {
					return v[:10]
				}
				return v
			default:
				return ""
			}
		},
	}).Parse(reportTemplate)

	if err != nil {
		return "", err
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, data); err != nil {
		return "", err
	}

	// Print button that only shows on screen, not on the PDF/Printout
	b := `<div class="no-print" style="width:100%;text-align:right;margin-bottom:15px;">
	<button onclick="window.print()" style="padding:10px 20px;background:#DAEEF3;color:#000;border:1.2pt solid #000;border-radius:4px;cursor:pointer;font-weight:bold;font-family:sans-serif;">Print to PDF</button>
	<style>@media print{.no-print{display:none}}</style></div>`

	finalHTML := strings.Replace(htmlBuf.String(), "<body>", b, 1)

	return finalHTML, nil
}
