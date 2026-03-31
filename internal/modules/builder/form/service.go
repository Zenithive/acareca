package form

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error)
	CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, d *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
	GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error)
	List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	UpdateFormStatus(ctx context.Context, formID uuid.UUID, status string) (*detail.RsFormDetail, error)
}

type service struct {
	db             *sqlx.DB
	detailSvc      detail.IService
	versionSvc     version.IService
	fieldSvc       field.IService
	formulaSvc     formula.IService
	entryRepo      entry.IRepository
	coaSvc         coa.Service
	auditSvc       audit.Service
	clinicSvc      interface{}
	eventsSvc      events.Service
	accountantRepo accountant.Repository
	authRepo       auth.Repository
	formClinic     clinic.Service
}

func NewService(db *sqlx.DB, detailSvc detail.IService, versionSvc version.IService, fieldSvc field.IService, formulaSvc formula.IService, entryRepo entry.IRepository, coaSvc coa.Service, auditSvc audit.Service, eventsSvc events.Service, accountantRepo accountant.Repository, authRepo auth.Repository, clinicSvc clinic.Service) IService {
	return &service{db: db, detailSvc: detailSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, formulaSvc: formulaSvc, entryRepo: entryRepo, coaSvc: coaSvc, auditSvc: auditSvc, eventsSvc: eventsSvc, accountantRepo: accountantRepo, authRepo: authRepo, formClinic: clinicSvc}
}

func (s *service) CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)

	// 1. Resolve the REAL owner at the start of THIS function
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, d.ClinicID)
	if err != nil {
		return nil, nil, err
	}
	// This is the 8dd760ab... ID you saw in the logs
	realOwnerID := clinic.PractitionerID
	if err := d.ValidateShares(); err != nil {
		return nil, nil, err
	}

	formReq := &detail.RqFormDetail{
		Name:           d.Name,
		Description:    d.Description,
		Status:         d.Status,
		Method:         d.Method,
		OwnerShare:     d.OwnerShare,
		ClinicShare:    d.ClinicShare,
		SuperComponent: d.SuperComponent,
	}
	if formReq.Status == "" {
		formReq.Status = StatusDraft
	}

	created, err := s.detailSvc.Create(ctx, formReq, d.ClinicID, realOwnerID)
	if err != nil {
		return nil, nil, err
	}

	syncResult := &RsFormWithFieldsSyncResult{ClinicID: created.ClinicID}

	if len(d.Fields) == 0 {
		return created, syncResult, nil
	}

	versions, err := s.versionSvc.List(ctx, created.ID, d.ClinicID)
	if err != nil {
		return nil, nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}
	if activeVersionID == uuid.Nil {
		return created, syncResult, nil
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {

		keyToFieldID := make(map[string]uuid.UUID, len(d.Fields))

		for _, f := range d.Fields {
			f.Sanitize()
			if err := f.Validate(); err != nil {
				return err
			}
			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, d.ClinicID, realOwnerID, f.ToRqFormField())
			if err != nil {
				return err
			}
			keyToFieldID[f.FieldKey] = created.ID
			syncResult.CreatedCount++
		}

		if len(d.Formulas) > 0 {
			if err := s.formulaSvc.SyncTx(ctx, tx, activeVersionID, d.Formulas, keyToFieldID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {

		actorUserID, err := uuid.Parse(*meta.UserID)
		if err != nil {

		} else {
			var finalAccountantID uuid.UUID
			accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
			if err == nil {
				finalAccountantID = accProfile.ID
			} else {
				finalAccountantID = actorUserID
			}

			// Fetching user details exactly like your Clinic implementation
			user, err := s.authRepo.FindByID(ctx, actorUserID)
			if err == nil {
				fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

				// Record the Event
				err = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: realOwnerID,
					AccountantID:   finalAccountantID,
					ActorID:        actorUserID,
					ActorName:      &fullName,
					ActorType:      "ACCOUNTANT",
					EventType:      "form.created",
					EntityType:     "FORM",
					EntityID:       created.ID, // Use 'created' from s.detailSvc.Create
					Description:    fmt.Sprintf("Accountant %s created a new form: %s", fullName, created.Name),
					Metadata:       events.JSONBMap{"form_name": created.Name},
					CreatedAt:      time.Now(),
				})

			}
		}
	}

	idStr := created.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionFormCreated,
		Module:     auditctx.ModuleForms,
		EntityType: strPtr(auditctx.EntityForm),
		EntityID:   &idStr,
		AfterState: created,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return created, syncResult, nil
}

func (s *service) UpdateWithFields(ctx context.Context, req *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)
	if err := req.ValidateShares(); err != nil {
		return nil, nil, err
	}

	existing, err := s.detailSvc.GetByID(ctx, *req.ID)
	if err != nil {
		return nil, nil, err
	}
	beforeState := *existing

	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, existing.ClinicID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve clinic owner: %w", err)
	}
	realOwnerID := clinic.PractitionerID

	var updated *detail.RsFormDetail
	var syncResult *RsFormWithFieldsSyncResult

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		updateReq := &detail.RqUpdateFormDetail{
			ID:             *req.ID,
			Name:           req.Name,
			Description:    req.Description,
			Status:         req.Status,
			Method:         req.Method,
			OwnerShare:     req.OwnerShare,
			ClinicShare:    req.ClinicShare,
			SuperComponent: req.SuperComponent,
		}

		upd, err := s.detailSvc.UpdateMetadata(ctx, updateReq)
		if err != nil {
			return err
		}
		updated = upd
		syncResult = &RsFormWithFieldsSyncResult{ClinicID: updated.ClinicID}

		versions, err := s.versionSvc.List(ctx, existing.ID, existing.ClinicID)
		if err != nil {
			return err
		}
		var activeVersionID uuid.UUID
		for _, v := range versions {
			if v.IsActive {
				activeVersionID = v.Id
				break
			}
		}
		if activeVersionID == uuid.Nil {
			return errors.New("cannot update fields: no active version found")
		}

		// Delete fields
		for _, id := range req.Fields.Delete {
			if s.entryRepo != nil {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
				if err != nil {
					return err
				}
				if has {
					return errors.New("field has submitted entries")
				}
			}
			if err := s.fieldSvc.DeleteTx(ctx, tx, id); err != nil {
				return err
			}
			syncResult.DeletedCount++
		}

		// Update fields
		for _, item := range req.Fields.Update {
			item.Sanitize()
			_, err = s.fieldSvc.UpdateTx(ctx, tx, item.ID, req.ClinicID, realOwnerID, &item)
			if err != nil {
				return err
			}
			syncResult.UpdatedCount++
		}

		// Create fields — build key→UUID map for formula resolution
		keyToFieldID := make(map[string]uuid.UUID)

		// Seed map with existing fields so formulas can reference them too
		existingFields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return err
		}
		for _, f := range existingFields {
			keyToFieldID[f.FieldKey] = f.ID
		}

		for _, item := range req.Fields.Create {
			item.Sanitize()
			if err := item.Validate(); err != nil {
				return err
			}
			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, req.ClinicID, practitionerID, item.ToRqFormField())
			if err != nil {
				return err
			}
			keyToFieldID[item.FieldKey] = created.ID
			syncResult.CreatedCount++
		}

		// Sync formulas (full replace)
		if len(req.Formulas) > 0 {
			if err := s.formulaSvc.SyncTx(ctx, tx, activeVersionID, req.Formulas, keyToFieldID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// --- TRIGGER SHARED EVENT RECORD (ACCOUNTANTS ONLY) ---
	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {

		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			var finalAccountantID uuid.UUID
			accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
			if err == nil {
				finalAccountantID = accProfile.ID
			} else {
				finalAccountantID = actorUserID
			}

			user, err := s.authRepo.FindByID(ctx, actorUserID)
			if err == nil {
				fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

				// Record the Event
				err = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: realOwnerID,
					AccountantID:   finalAccountantID,
					ActorID:        actorUserID,
					ActorName:      &fullName,
					ActorType:      "ACCOUNTANT",
					EventType:      "form.updated",
					EntityType:     "FORM",
					EntityID:       updated.ID,
					Description:    fmt.Sprintf("Accountant %s updated the form: %s", fullName, updated.Name),
					Metadata: events.JSONBMap{
						"form_name":     updated.Name,
						"updated_count": syncResult.UpdatedCount,
						"created_count": syncResult.CreatedCount,
						"deleted_count": syncResult.DeletedCount,
					},
					CreatedAt: time.Now(),
				})

			}
		}
	}

	//meta := auditctx.GetMetadata(ctx)
	idStr := updated.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionFormUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  strPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  updated,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return updated, syncResult, nil
}

func (s *service) BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error) {
	form, err := s.detailSvc.GetByID(ctx, req.FormID)
	if err != nil {
		return nil, err
	}

	result := &RsBulkSyncFields{
		ClinicID: req.ClinicID,
		Created:  []field.RsFormField{},
		Updated:  []field.RsFormField{},
		Deleted:  []uuid.UUID{},
	}

	versions, err := s.versionSvc.List(ctx, form.ID, req.ClinicID)
	if err != nil {
		return nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, fieldID := range req.Delete {
			if s.entryRepo != nil {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, fieldID)
				if err != nil {
					return err
				}
				if has {
					return errors.New("field has submitted entries")
				}
			}
			if err := s.fieldSvc.DeleteTx(ctx, tx, fieldID); err != nil {
				return err
			}
			result.Deleted = append(result.Deleted, fieldID)
		}

		for _, updateItem := range req.Update {
			updateItem.Sanitize()
			updated, err := s.fieldSvc.UpdateTx(ctx, tx, updateItem.ID, req.ClinicID, practitionerID, &updateItem)
			if err != nil {
				return err
			}
			result.Updated = append(result.Updated, *updated)
		}

		for _, createItem := range req.Create {
			createItem.Sanitize()
			if err := createItem.Validate(); err != nil {
				return err
			}
			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, req.ClinicID, practitionerID, createItem.ToRqFormField())
			if err != nil {
				return err
			}
			result.Created = append(result.Created, *created)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error) {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}
	out := &RsFormWithFields{
		Form:     *formDetail,
		Fields:   []field.RsFormField{},
		Formulas: []formula.RsFormula{},
	}
	versions, err := s.versionSvc.List(ctx, formDetail.ID, formDetail.ClinicID)
	if err != nil {
		return nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}
	if activeVersionID != uuid.Nil {
		out.ActiveVersionID = activeVersionID
		fields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			out.Fields = append(out.Fields, *f)
		}
		formulas, err := s.formulaSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return nil, err
		}
		out.Formulas = formulas
	}
	return out, nil
}

func (s *service) List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error) {
	// Pass the request to the detail service and return the consolidated result
	return s.detailSvc.List(ctx, detail.Filter{
		ClinicID: filter.ClinicID,
		FormName: filter.FormName,
		Status:   filter.Status,
		Method:   filter.Method,
		Filter:   filter.Filter, // Include the embedded common.Filter fields (Search, Limit, Offset, etc.)
	}, practitionerID)
}

func (s *service) Delete(ctx context.Context, formID uuid.UUID) error {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return err
	}
	// 2. Resolve the REAL owner (Practitioner) from the Clinic
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, formDetail.ClinicID)
	if err != nil {
		return fmt.Errorf("failed to resolve clinic owner: %w", err)
	}
	realOwnerID := clinic.PractitionerID
	if err := s.detailSvc.Delete(ctx, formDetail.ID); err != nil {
		return err
	}

	// --- TRIGGER SHARED EVENT RECORD (ACCOUNTANTS ONLY) ---
	meta := auditctx.GetMetadata(ctx)
	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {
		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			var finalAccountantID uuid.UUID
			accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
			if err == nil {
				finalAccountantID = accProfile.ID
			} else {
				finalAccountantID = actorUserID
			}

			user, err := s.authRepo.FindByID(ctx, actorUserID)
			if err == nil {
				fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

				// Record the Shared Event
				_ = s.eventsSvc.Record(ctx, events.SharedEvent{
					ID:             uuid.New(),
					PractitionerID: realOwnerID, // The Clinic Owner
					AccountantID:   finalAccountantID,
					ActorID:        actorUserID,
					ActorName:      &fullName,
					ActorType:      "ACCOUNTANT",
					EventType:      "form.deleted",
					EntityType:     "FORM",
					EntityID:       formID,
					Description:    fmt.Sprintf("Accountant %s deleted form: %s", fullName, formDetail.Name),
					Metadata:       events.JSONBMap{"form_name": formDetail.Name},
					CreatedAt:      time.Now(),
				})
			}
		}
	}
	// Audit log: form deleted
	//meta := auditctx.GetMetadata(ctx)
	idStr := formID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionFormDeleted,
		Module:      auditctx.ModuleForms,
		EntityType:  strPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: formDetail,
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return nil
}

// GetByID implements [IService].
func (s *service) GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error) {

	detail, err := s.detailSvc.GetByID(ctx, formId)
	if err != nil {
		return detail, err
	}

	return detail, err
}

func (s *service) UpdateFormStatus(ctx context.Context, formID uuid.UUID, status string) (*detail.RsFormDetail, error) {
	// Fetch current state for audit log and validation
	existing, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}

	// Call the detail service to perform the update
	err = s.detailSvc.UpdateFormStatus(ctx, &detail.RqUpdateFormStatus{
		ID:     formID,
		Status: status,
	})
	if err != nil {
		return nil, err
	}

	// Fetch updated form to return in response
	updated, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}

	// Audit log: Status Updated
	meta := auditctx.GetMetadata(ctx)
	idStr := formID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID:  meta.PracticeID,
		UserID:      meta.UserID,
		Action:      auditctx.ActionFormUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  strPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: map[string]string{"status": existing.Status},
		AfterState:  map[string]string{"status": status},
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
	})

	return updated, nil
}

func strPtr(s string) *string { return &s }
