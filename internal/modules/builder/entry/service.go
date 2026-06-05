package entry

import (
	"context"
	"fmt"
	"log"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/export"
	entryexport "github.com/iamarpitzala/acareca/internal/shared/export/entry"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type IService interface {
	Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID, role string) (*RsFormEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID, role string) (*RsFormEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, formVersionID uuid.UUID, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error)
	GetByVersionID(ctx context.Context, id uuid.UUID) (*RsFormEntry, error)
	ListTransactions(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error)
	// COA-grouped endpoints
	ListCoaEntries(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, userID uuid.UUID) (*util.RsList, error)
	ListCoaEntryDetails(ctx context.Context, coaID string, filter TransactionFilter, actorID uuid.UUID, role string) (*util.RsList, error)
	ExportTransactionReport(ctx context.Context, filter TransactionFilter, actorID uuid.UUID, role string, exportType string, userID uuid.UUID, PracIDs []uuid.UUID) (interface{}, string, error)
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
	invitationRepo  invitation.Repository
	detailRepo      detail.IRepository
	financialRepo   fy.Repository
	practitionerSvc practitioner.IService
	coaRepo         coa.Repository
	notificationSvc notification.Service
	notificationPub *sharednotification.Publisher
	adminRepo       admin.Repository
	authSvc         auth.Service
}

func NewService(db *sqlx.DB, repo IRepository, fieldRepo field.IRepository, methodSvc method.IService, detailSvc detail.IService, versionSvc version.IService, auditSvc audit.Service, eventsSvc events.Service, accRepo accountant.Repository, authRepo auth.Repository, clinicRepo clinic.Repository, clinicSvc clinic.Service, formulaSvc formula.IService, fieldSvc field.IService, invitationSvc invitation.Service, invitationRepo invitation.Repository, detailRepo detail.IRepository, financialRepo fy.Repository, practitionerSvc practitioner.IService, coaRepo coa.Repository, notificationSvc notification.Service, adminRepo admin.Repository, authSvc auth.Service) IService {
	return &Service{
		repo:            repo,
		fieldRepo:       fieldRepo,
		methodSvc:       methodSvc,
		limitsSvc:       limits.NewService(db),
		detailSvc:       detailSvc,
		versionSvc:      versionSvc,
		auditSvc:        auditSvc,
		formulaSvc:      formulaSvc,
		eventsSvc:       eventsSvc,
		accountantRepo:  accRepo,
		authRepo:        authRepo,
		clinicRepo:      clinicRepo,
		formClinic:      clinicSvc,
		fieldSvc:        fieldSvc,
		invitationSvc:   invitationSvc,
		invitationRepo:  invitationRepo,
		detailRepo:      detailRepo,
		financialRepo:   financialRepo,
		practitionerSvc: practitionerSvc,
		coaRepo:         coaRepo,
		notificationSvc: notificationSvc,
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc)),
		adminRepo:       adminRepo,
		authSvc:         authSvc,
	}
}

func (s *Service) Create(ctx context.Context, formVersionID uuid.UUID, req *RqFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID, role string) (*RsFormEntry, error) {
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

		// Verify ledger integrity before commit
		if err := s.repo.AssertLedgerGroupBalances(ctx, tx, result.ID); err != nil {
			return err
		}

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

	if err = s.notifyTransaction(ctx, entityID, util.ActorType(role), util.EventTransactionCreated, "Transaction Created"); err != nil {
		log.Printf("failed to send transaction notification event: %v", err.Error())
	}

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

func (s *Service) Update(ctx context.Context, id uuid.UUID, req *RqUpdateFormEntry, submittedBy *uuid.UUID, entityID uuid.UUID, role string) (*RsFormEntry, error) {
	var result *RsFormEntry
	var beforeState any

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

		// Verify ledger integrity before commit
		if err := s.repo.AssertLedgerGroupBalances(ctx, tx, id); err != nil {
			return err
		}

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

	// Send update notification
	if err = s.notifyTransaction(ctx, entityID, util.ActorType(role), util.EventTransactionUpdated, "Transaction Updated"); err != nil {
		log.Printf("failed to send transaction update notification event: %v", err.Error())
	}

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
	// =========================================================================
	// PASS 1: VALIDATION, TAX CALCULATIONS, AND FORMULA EXECUTION
	// =========================================================================
	out := make([]*FormEntryValue, 0, len(rq))
	keyValues := make(map[string]float64, len(rq))
	taxTypeByKey := make(map[string]string, len(rq))

	// Process direct COA entries
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

			inputAmount = s.roundValue(inputAmount)

			out = append(out, &FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            entryID,
				FormFieldID:        nil,
				CoaID:              &coaID,
				NetAmount:          &inputAmount,
				GstAmount:          nil,
				GrossAmount:        &inputAmount,
				Description:        v.Description,
				BusinessPercentage: v.BusinessPercentage,
				Notes:              v.Notes,
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
			netBase = s.roundValue(inputAmount - *v.GstAmount)
			roundedGross := s.roundValue(grossTotal)

			keyValues[f.FieldKey] = netBase
			out = append(out, &FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            entryID,
				FormFieldID:        &fieldID,
				CoaID:              f.CoaID, // 🚀 FIXED
				NetAmount:          &netBase,
				GstAmount:          gstAmount,
				GrossAmount:        &roundedGross,
				Description:        v.Description,
				BusinessPercentage: v.BusinessPercentage,
				Notes:              v.Notes,
			})
			continue
		}

		if f.TaxType == nil {
			netBase = s.roundValue(netBase)
			grossTotal = s.roundValue(grossTotal)
			keyValues[f.FieldKey] = netBase
			out = append(out, &FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            entryID,
				FormFieldID:        &fieldID,
				CoaID:              f.CoaID, // 🚀 FIXED
				NetAmount:          &netBase,
				GstAmount:          nil,
				GrossAmount:        &grossTotal,
				Description:        v.Description,
				BusinessPercentage: v.BusinessPercentage,
				Notes:              v.Notes,
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
			roundedGst := s.roundValue(result.GstAmount)
			gstAmount = &roundedGst
			netBase = s.roundValue(result.Amount)
			grossTotal = s.roundValue(result.TotalAmount)

		case method.TaxTreatmentExclusive:
			result, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: inputAmount})
			if err != nil {
				return nil, err
			}
			gstAmount = &result.GstAmount
			netBase = s.roundValue(inputAmount)
			grossTotal = s.roundValue(result.TotalAmount)

		case method.TaxTreatmentManual:
			gstAmount = v.GstAmount

			if v.GrossAmount != nil {
				grossTotal = s.roundValue(*v.GrossAmount)
				if v.GstAmount != nil {
					netBase = s.roundValue(*v.GrossAmount - *v.GstAmount)
				} else {
					netBase = s.roundValue(*v.GrossAmount)
				}
			} else {
				grossTotal = s.roundValue(inputAmount)
				if v.GstAmount != nil {
					netBase = s.roundValue(inputAmount - *v.GstAmount)
				} else {
					netBase = s.roundValue(inputAmount)
				}
			}

		case method.TaxTreatmentZero:
			gstAmount = nil
			netBase = s.roundValue(inputAmount)
			grossTotal = s.roundValue(inputAmount)

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
			ID:                 uuid.New(),
			EntryID:            entryID,
			FormFieldID:        &fieldID,
			CoaID:              f.CoaID, // 🚀 FIXED
			NetAmount:          &netBase,
			GstAmount:          gstAmount,
			GrossAmount:        &grossTotal,
			Description:        v.Description,
			BusinessPercentage: v.BusinessPercentage,
			Notes:              v.Notes,
		})
	}

	// Execute formulas if available
	var firstField *field.FormField
	if s.formulaSvc != nil && len(rq) > 0 {
		var firstFieldID uuid.UUID
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

		if firstField != nil {
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

				netBase := s.roundValue(val)
				grossTotal := s.roundValue(val)
				var gstAmount *float64

				if f.TaxType != nil {
					taxType := method.TaxTreatment(*f.TaxType)

					switch taxType {
					case method.TaxTreatmentInclusive:
						gst := s.roundValue(val * 0.10)
						gstAmount = &gst
						netBase = s.roundValue(val)
						grossTotal = s.roundValue(val + gst)
					case method.TaxTreatmentExclusive:
						gst := s.roundValue(val * 0.10)
						gstAmount = &gst
						netBase = s.roundValue(val)
						grossTotal = s.roundValue(val + gst)
					case method.TaxTreatmentManual:
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

						grossTotal = s.roundValue(val)
						if entryGST == nil {
							gst := 0.0
							gstAmount = &gst
							netBase = s.roundValue(val)
						} else {
							gstAmount = entryGST
							netBase = s.roundValue(val - *entryGST)
						}
					case method.TaxTreatmentZero:
						gstAmount = nil
						netBase = s.roundValue(val)
						grossTotal = s.roundValue(val)
					}
				}

				out = append(out, &FormEntryValue{
					ID:                 uuid.New(),
					EntryID:            entryID,
					FormFieldID:        &fieldID,
					CoaID:              f.CoaID,
					NetAmount:          &netBase,
					GstAmount:          gstAmount,
					GrossAmount:        &grossTotal,
					BusinessPercentage: nil, // Formulas don't have business percentage
					Notes:              nil, // Formulas don't have notes
				})
			}
		}
	}

	// =========================================================================
	// PASS 2: DOUBLE-ENTRY LEDGER IMPACT CALCULATION & AUTOMATIC BALANCING
	// =========================================================================
	// Rule: amounts are stored with their natural sign (positive = normal balance).
	// Assets/Expenses are debit-normal  → contribute +amount to the balance equation.
	// Liability/Equity/Revenue/Income are credit-normal → contribute -amount.
	// A balanced entry sums to zero. If not, inject a COA-600 (Bank) offset row.

	if len(out) > 0 {
		var totalLedgerImpact float64
		var practitionerID *uuid.UUID

		for _, ev := range out {
			if ev.NetAmount == nil || *ev.NetAmount == 0 || ev.CoaID == nil {
				continue
			}

			chartAccount, err := s.coaRepo.GetByIDInternal(ctx, *ev.CoaID)
			if err == nil && chartAccount != nil {
				if practitionerID == nil {
					practitionerID = &chartAccount.PractitionerID
				}

				accountType := strings.ToLower(chartAccount.AccountTypeName)
				// Use the raw signed amount — the sign already encodes debit/credit direction.
				if strings.Contains(accountType, "asset") || strings.Contains(accountType, "expense") {
					totalLedgerImpact += *ev.NetAmount
				} else if strings.Contains(accountType, "liability") || strings.Contains(accountType, "equity") || strings.Contains(accountType, "revenue") || strings.Contains(accountType, "income") {
					totalLedgerImpact -= *ev.NetAmount
				}
			}
		}

		totalLedgerImpact = s.roundValue(totalLedgerImpact)

		if math.Abs(totalLedgerImpact) > 0.01 && practitionerID != nil {
			bankAccount, err := s.coaRepo.GetChartByCodeAndPractitionerID(ctx, 600, *practitionerID, nil)
			if err != nil || bankAccount == nil {
				return nil, fmt.Errorf("missing required balancing account: COA code 600 Business Bank Account for practitioner %s", practitionerID.String())
			}

			// COA-600 is an Asset; to absorb the variance we invert it.
			counterBalancingAmount := s.roundValue(-totalLedgerImpact)
			bankCoaID := bankAccount.ID

			var totalGSTInTransaction float64
			for _, ev := range out {
				if ev.CoaID != nil {
					chartAcc, err := s.coaRepo.GetByIDInternal(ctx, *ev.CoaID)
					if err == nil && chartAcc != nil && chartAcc.Code == 820 {
						continue // Skip GST account 820
					}
				}
				if ev.GstAmount != nil {
					totalGSTInTransaction += *ev.GstAmount
				}
			}
			totalGSTInTransaction = s.roundValue(totalGSTInTransaction)

			bankGrossAmount := s.roundValue(counterBalancingAmount + totalGSTInTransaction)

			out = append(out, &FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            entryID,
				FormFieldID:        nil,
				CoaID:              &bankCoaID,
				NetAmount:          &counterBalancingAmount,
				GstAmount:          nil,
				GrossAmount:        &bankGrossAmount,
				BusinessPercentage: nil, // System balancing entries don't have business percentage
				Notes:              nil, // System balancing entries don't have notes
			})
		}

		var finalLedgerBalance float64
		for _, ev := range out {
			if ev.NetAmount == nil || *ev.NetAmount == 0 || ev.CoaID == nil {
				continue
			}

			chartAccount, err := s.coaRepo.GetByIDInternal(ctx, *ev.CoaID)
			if err == nil && chartAccount != nil {
				accountType := strings.ToLower(chartAccount.AccountTypeName)
				if strings.Contains(accountType, "asset") || strings.Contains(accountType, "expense") {
					finalLedgerBalance += *ev.NetAmount
				} else if strings.Contains(accountType, "liability") || strings.Contains(accountType, "equity") || strings.Contains(accountType, "revenue") || strings.Contains(accountType, "income") {
					finalLedgerBalance -= *ev.NetAmount
				}
			}
		}

		finalLedgerBalance = s.roundValue(finalLedgerBalance)
		if math.Abs(finalLedgerBalance) > 0.01 {
			return nil, fmt.Errorf("ledger integrity violation: variance of %.2f exceeds 0.01 threshold after balancing", finalLedgerBalance)
		}
	}

	return out, nil
}

func (s *Service) roundValue(val float64) float64 {
	return math.Round(val*100) / 100
}

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
			config := export.ExportConfig{
				EntityName:     entityName,
				EntityABN:      practitionerABN,
				Period:         period,
				ExportType:     exportType,
				ExportedByName: fullName,
				GeneratedTime:  time.Now().Format("02/01/2006, 3:04:05 pm"),
			}

			exportGroups := make([]*entryexport.CoaGroup, len(groups))
			for i, g := range groups {
				exportDetails := make([]*entryexport.CoaDetail, len(g.Details))
				for j, d := range g.Details {
					var createdAtTime time.Time
					if d.CreatedAt != "" {
						parsedTime, err := time.Parse("2006-01-02", d.CreatedAt)
						if err == nil {
							createdAtTime = parsedTime
						}
					}

					exportDetails[j] = &entryexport.CoaDetail{
						FormFieldName: d.FormFieldName,
						TaxTypeName:   d.TaxTypeName,
						FormName:      lo.FromPtrOr(d.FormName, "-"),
						ClinicName:    lo.FromPtrOr(d.ClinicName, "-"),
						NetAmount:     d.NetAmount,
						GstAmount:     d.GstAmount,
						GrossAmount:   d.GrossAmount,
						CreatedAt:     createdAtTime,
						IsExpense:     d.IsExpense,
					}
				}
				exportGroups[i] = &entryexport.CoaGroup{
					CoaID:            g.CoaID,
					CoaName:          g.CoaName,
					TotalNetAmount:   g.TotalNetAmount,
					TotalGrossAmount: g.TotalGrossAmount,
					Details:          exportDetails,
				}
			}

			buf, err := entryexport.GenerateExcelReport(exportGroups, config, formatDateHelper)
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

func (s *Service) notifyTransaction(ctx context.Context, entityID uuid.UUID, recipientType util.ActorType, eventType util.EventType, title string) error {
	if s.notificationPub == nil {
		return fmt.Errorf("notification publisher is nil")
	}

	user, err := s.authSvc.GetUserByID(ctx, entityID, recipientType)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	senderName := user.FirstName + " " + user.LastName
	senderType := recipientType

	// Build recipients list
	recipients := []sharednotification.RecipientWithPreferences{}

	switch recipientType {
	case util.ActorPractitioner:
		// If the sender is a practitioner, notify their linked accountants
		accountants, err := s.invitationRepo.GetAccountantsLinkedToPractitioner(ctx, entityID)
		if err != nil {
			log.Printf("[WARN] failed to get linked accountants for practitioner %s: %v", entityID, err)
			return nil // Don't fail transaction if notification fails
		}

		for _, acc := range accountants {
			// Check if accountant has notification access permission
			permissions, err := s.invitationSvc.GetPermissionsForAccountant(ctx, acc.AccountantID, entityID)
			if err != nil {
				log.Printf("[WARN] failed to get permissions for accountant %s: %v", acc.AccountantID, err)
				continue
			}

			// Only notify accountants with reports view/download permission
			if permissions != nil && permissions.Has(invitation.PermReportsViewDownload, false) {
				recipients = append(recipients, sharednotification.RecipientWithPreferences{
					RecipientID:   acc.AccountantID,
					RecipientType: util.ActorAccountant,
					UserID:        acc.UserID,
				})
			}
		}

	case util.ActorAccountant:
		// If the sender is an accountant, we need to find which practitioners to notify
		// Get all practitioners linked to this accountant
		practitionerIDs, err := s.invitationRepo.GetPractitionersLinkedToAccountant(ctx, entityID)
		if err != nil {
			log.Printf("[WARN] failed to get practitioners for accountant %s: %v", entityID, err)
			return nil // Don't fail transaction if notification fails
		}

		// Notify each linked practitioner
		for _, practitionerID := range practitionerIDs {
			practitionerUserID, err := s.invitationRepo.GetPractitionerUserIDByID(ctx, practitionerID)
			if err != nil {
				log.Printf("[WARN] failed to get user ID for practitioner %s: %v", practitionerID, err)
				continue
			}

			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   practitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        practitionerUserID,
			})
		}

	default:
		return fmt.Errorf("unsupported recipient type: %s", recipientType)
	}

	// If no recipients, don't send notification
	if len(recipients) == 0 {
		log.Printf("[INFO] no recipients found for transaction notification")
		return nil
	}

	// Send notifications with preferences using the new publisher
	return s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   entityID,
		SenderType: senderType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityTransaction,
		EntityID:   entityID,
		EntityKey:  "transaction_id",
		Title:      title,
		Body:       fmt.Sprintf("%s by %s", title, senderName),
	})
}
