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
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error)
	CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, ownerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, d *RqUpdateFormWithFields, actorID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
	GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error)
	List(ctx context.Context, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error)
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
	invitationSvc  invitation.Service
}

func NewService(db *sqlx.DB, detailSvc detail.IService, versionSvc version.IService, fieldSvc field.IService, formulaSvc formula.IService, entryRepo entry.IRepository, coaSvc coa.Service, auditSvc audit.Service, eventsSvc events.Service, accountantRepo accountant.Repository, authRepo auth.Repository, clinicSvc clinic.Service, invitationSvc invitation.Service) IService {
	return &service{db: db, detailSvc: detailSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, formulaSvc: formulaSvc, entryRepo: entryRepo, coaSvc: coaSvc, auditSvc: auditSvc, eventsSvc: eventsSvc, accountantRepo: accountantRepo, authRepo: authRepo, formClinic: clinicSvc, invitationSvc: invitationSvc}
}

func (s *service) CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, ownerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)

	// Permission checks are now handled by middleware - no need to check here

	// 1. Resolve the REAL owner at the start of THIS function
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, d.ClinicID)
	if err != nil {
		return nil, nil, err
	}
	realOwnerID := clinic.PractitionerID
	if err := d.ValidateShares(); err != nil {
		return nil, nil, err
	}

	var created *detail.RsFormDetail
	syncResult := &RsFormWithFieldsSyncResult{ClinicID: d.ClinicID}

	// Create form and fields within transaction (atomic operation)
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {

		// Create form and Version
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

		var createErr error
		// Create form via detail service
		created, createErr = s.detailSvc.CreateTx(ctx, tx, formReq, d.ClinicID, realOwnerID)
		if createErr != nil {
			return createErr
		}

		if len(d.Fields) == 0 {
			return nil
		}

		// Get active version
		versions, err := s.versionSvc.ListTx(ctx, tx, created.ID, d.ClinicID)
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
			// If we just created the form, we expect a version. If not found, fail the TX.
			return fmt.Errorf("active version not found for form %s", created.ID)
		}

		// Create form fields
		keyToFieldID := make(map[string]uuid.UUID, len(d.Fields))
		for _, f := range d.Fields {
			f.Sanitize()
			if err := f.Validate(); err != nil {
				return err
			}
			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, d.ClinicID, realOwnerID, f.ToRqFormField())
			if err != nil {
				return err // Rollback everything including the Form
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
	// If transaction failed, exit before touching 'created'
	if err != nil {
		return nil, nil, err
	}
	// --- EVERYTHING BELOW ONLY RUNS ON SUCCESS ---
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

func (s *service) UpdateWithFields(ctx context.Context, req *RqUpdateFormWithFields, actorID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)
	// Permission checks are handled by middleware

	req.Normalize()

	if err := req.ValidateShares(); err != nil {
		return nil, nil, err
	}

	existing, err := s.detailSvc.GetByID(ctx, *req.ID, uuid.Nil, "")
	if err != nil {
		return nil, nil, err
	}
	beforeState := *existing

	// // PERMISSION CHECK (Accountant Only)
	// if isAccountant {
	// 	// Check if they have 'update' or 'all' permission for this FORM
	// 	perms, err := s.invitationSvc.GetPermissionsForAccountant(ctx, actorID, existing.ID)
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("Authentication error: %w", err)
	// 	}

	// 	// Deny if no direct mapping exists OR if permissions don't allow 'update'/'all'
	// 	if perms == nil || (!perms.HasAccess("update") && !perms.HasAccess("all")) {
	// 		return nil, nil, errors.New("Access denied: you do not have permission to update this form")
	// 	}
	// }

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
		forceDelete := req.ForceDelete != nil && *req.ForceDelete
		for _, id := range req.Fields.Delete {
			existingField, err := s.fieldSvc.GetByID(ctx, id)
			if err != nil {
				return fmt.Errorf("field %s not found for deletion: %w", id, err)
			}

			if s.entryRepo != nil && !forceDelete {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
				if err != nil {
					return err
				}
				if has {
					return fmt.Errorf("cannot delete field %q (key: %s): field has submitted entries. Use force_delete=true to override (warning: this will orphan entry data)", existingField.Label, existingField.FieldKey)
				}
			}

			if err := s.fieldSvc.DeleteTx(ctx, tx, id); err != nil {
				return fmt.Errorf("failed to delete field %s: %w", id, err)
			}
			syncResult.DeletedCount++
		}

		// Build key→UUID map for formula resolution
		keyToFieldID := make(map[string]uuid.UUID)

		// Create a set of deleted field IDs for quick lookup
		deletedFieldIDs := make(map[uuid.UUID]bool, len(req.Fields.Delete))
		for _, id := range req.Fields.Delete {
			deletedFieldIDs[id] = true
		}

		// Seed map with existing fields (excluding deleted ones) so formulas can reference them
		// Note: If multiple fields have the same key, the last one wins (map behavior)
		existingFields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return err
		}
		for _, f := range existingFields {
			// Skip fields that are being deleted in this transaction
			if !deletedFieldIDs[f.ID] {
				keyToFieldID[f.FieldKey] = f.ID
			}
		}

		// Update fields
		for _, item := range req.Fields.Update {
			item.Sanitize()

			// Verify the field exists before updating
			existingField, err := s.fieldSvc.GetByID(ctx, item.ID)
			if err != nil {
				return fmt.Errorf("field %s not found for update: %w", item.ID, err)
			}

			updated, err := s.fieldSvc.UpdateTx(ctx, tx, item.ID, req.ClinicID, realOwnerID, &item)
			if err != nil {
				return err
			}
			// Use the existing field key (field_key is immutable)
			// If duplicate keys exist, the last one wins
			keyToFieldID[existingField.FieldKey] = updated.ID
			syncResult.UpdatedCount++
		}

		// Create fields
		for _, item := range req.Fields.Create {
			item.Sanitize()
			if err := item.Validate(); err != nil {
				return fmt.Errorf("validation failed for field %s: %w", item.FieldKey, err)
			}

			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, req.ClinicID, actorID, item.ToRqFormField())
			if err != nil {
				return fmt.Errorf("failed to create field %s: %w", item.FieldKey, err)
			}
			// If duplicate keys exist, the last one wins
			keyToFieldID[item.FieldKey] = created.ID
			syncResult.CreatedCount++
		}

		// Sync formulas (full replace)
		// At this point, keyToFieldID contains all fields: existing (not deleted), updated, and newly created
		// This ensures formulas can reference any field, including newly created calculated fields
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
	form, err := s.detailSvc.GetByID(ctx, req.FormID, uuid.Nil, "")
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
	formDetail, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, "")
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

func (s *service) List(ctx context.Context, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error) {
	// Pass actor info to detail service for data filtering based on ownership/permissions
	return s.detailSvc.List(ctx, detail.Filter{
		ClinicIDs: filter.ClinicIDs,
		FormName:  filter.FormName,
		Status:    filter.Status,
		Method:    filter.Method,
		Filter:    filter.Filter,
	}, actorID, role)
}

func (s *service) Delete(ctx context.Context, formID uuid.UUID) error {
	formDetail, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, "")
	if err != nil {
		return err
	}

	// 2. Resolve the REAL owner (Practitioner) from the Clinic
	clinic, err := s.formClinic.GetClinicByIDInternal(ctx, formDetail.ClinicID)
	if err != nil {
		return fmt.Errorf("failed to resolve clinic owner: %w", err)
	}
	realOwnerID := clinic.PractitionerID

	// TRANSACTIONAL DELETION
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete the Form
		if err := s.detailSvc.Delete(ctx, tx, formDetail.ID); err != nil {
			return err
		}

		// Delete associated permissions for this Form
		// if err := s.invitationSvc.DeletePermission(ctx, tx, formID); err != nil {
		// 	return err
		// }

		return nil
	})

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
	// Permission checks are handled by middleware
	detail, err := s.detailSvc.GetByID(ctx, formId, uuid.Nil, "")
	if err != nil {
		return detail, err
	}
	return detail, err
}

func (s *service) UpdateFormStatus(ctx context.Context, formID uuid.UUID, status string) (*detail.RsFormDetail, error) {
	// Fetch current state for audit log and validation
	// Permission checks are handled by middleware
	existing, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, "")
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
	updated, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, "")
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
