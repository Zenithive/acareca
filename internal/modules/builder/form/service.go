package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/business/practitioner"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

// AuthUserInfo represents user information needed for notifications
type AuthUserInfo struct {
	ID        uuid.UUID
	FirstName string
	LastName  string
}

// AuthService defines the interface for auth operations needed by form service
type AuthService interface {
	GetUserByID(ctx context.Context, entityID uuid.UUID, EntityType util.ActorType) (*AuthUserInfo, error)
}

type IService interface {
	GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error)
	CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, ownerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, d *RqUpdateFormWithFields, actorID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
	GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error)
	List(ctx context.Context, filter Filter, actorID uuid.UUID, role string) (*util.RsList, error)
	Delete(ctx context.Context, formID uuid.UUID) error
	UpdateFormStatus(ctx context.Context, formID uuid.UUID, status string) (*detail.RsFormDetail, error)
	CreateExpense(ctx context.Context, rq RqExpense, actorId uuid.UUID, role string) (*detail.RsFormDetail, error)
	UpdateExpense(ctx context.Context, formID uuid.UUID, rq RqUpdateExpense, actorId uuid.UUID) (*detail.RsFormDetail, error)
	GetExpense(ctx context.Context, formID uuid.UUID, actorId uuid.UUID, role string) (*RsExpense, error)
}

type service struct {
	db              *sqlx.DB
	detailSvc       detail.IService
	versionSvc      version.IService
	fieldSvc        field.IService
	formulaSvc      formula.IService
	entryRepo       entry.IRepository
	coaSvc          coa.Service
	auditSvc        audit.Service
	eventsSvc       events.Service
	accountantRepo  accountant.Repository
	authRepo        auth.Repository
	formClinic      clinic.Service
	invitationSvc   invitation.Service
	practitionerSvc practitioner.IService
	financialRepo   fy.Repository
	notificationPub *sharednotification.Publisher
	invitationRepo  invitation.Repository
	authSvc         AuthService
	adminRepo       admin.Repository
}

func NewService(db *sqlx.DB, detailSvc detail.IService, versionSvc version.IService, fieldSvc field.IService, formulaSvc formula.IService, entryRepo entry.IRepository, coaSvc coa.Service, auditSvc audit.Service, eventsSvc events.Service, accountantRepo accountant.Repository, authRepo auth.Repository, clinicSvc clinic.Service, invitationSvc invitation.Service, practitionerSvc practitioner.IService, financialRepo fy.Repository, notificationSvc notification.Service, invitationRepo invitation.Repository, authSvc AuthService, adminRepo admin.Repository) IService {
	return &service{db: db, detailSvc: detailSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, formulaSvc: formulaSvc, entryRepo: entryRepo, coaSvc: coaSvc, auditSvc: auditSvc, eventsSvc: eventsSvc, accountantRepo: accountantRepo, authRepo: authRepo, formClinic: clinicSvc, invitationSvc: invitationSvc, practitionerSvc: practitionerSvc, financialRepo: financialRepo, notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), adminRepo), invitationRepo: invitationRepo, authSvc: authSvc, adminRepo: adminRepo}
}

func (s *service) CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, ownerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)

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
		created, createErr = s.detailSvc.CreateTx(ctx, tx, formReq, &d.ClinicID, realOwnerID)
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
			return fmt.Errorf("active version not found for form %s", created.ID)
		}

		// Create form fields
		keyToFieldID := make(map[string]uuid.UUID, len(d.Fields))
		for _, f := range d.Fields {
			f.Sanitize()
			if err := f.Validate(); err != nil {
				return err
			}
			created, err := s.fieldSvc.Create(ctx, tx, activeVersionID, &d.ClinicID, realOwnerID, f.ToRqFormField())
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
					EntityID:       created.ID,
					Description:    fmt.Sprintf("Accountant %s created a new form: %s", fullName, created.Name),
					Metadata:       events.JSONBMap{"form_name": created.Name},
					CreatedAt:      time.Now(),
				})

			}
		}
	}

	idStr := created.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionFormCreated,
		Module:     auditctx.ModuleForms,
		EntityType: lo.ToPtr(auditctx.EntityForm),
		EntityID:   &idStr,
		AfterState: created,
	})

	// Send notification
	if meta.UserID != nil && meta.UserType != nil {
		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			actorType := util.ActorPractitioner
			if strings.EqualFold(*meta.UserType, util.RoleAccountant) {
				actorType = util.ActorAccountant
			}

			eventType := util.EventFormSubmitted
			if created.Status != "PUBLISHED" {
				eventType = util.EventFormUpdated
			}

			if err := s.notifyForm(ctx, created.ID, actorUserID, actorType, eventType, created.Name); err != nil {
				log.Printf("[WARN] failed to send form creation notification: %v", err)
			}
		}
	}

	return created, syncResult, nil
}

func (s *service) UpdateWithFields(ctx context.Context, req *RqUpdateFormWithFields, actorID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	meta := auditctx.GetMetadata(ctx)
	req.Normalize()

	if err := req.ValidateShares(); err != nil {
		return nil, nil, err
	}

	// Get existing state for "BeforeState" and ownership resolution
	existing, err := s.detailSvc.GetByID(ctx, *req.ID, uuid.Nil, "")
	if err != nil {
		return nil, nil, err
	}
	beforeState := *existing

	// Skip clinic resolution for expense forms
	var realOwnerID uuid.UUID
	if existing.ClinicID != nil && *existing.ClinicID != uuid.Nil {
		clinic, err := s.formClinic.GetClinicByIDInternal(ctx, *existing.ClinicID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve clinic owner: %w", err)
		}
		realOwnerID = clinic.PractitionerID
	} else {
		// For expense forms, use the actorID as the owner
		realOwnerID = actorID
	}

	var updated *detail.RsFormDetail
	var syncResult *RsFormWithFieldsSyncResult

	// Execute Updates in Transaction
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Update Metadata
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
		clinicID := uuid.Nil
		if updated.ClinicID != nil {
			clinicID = *updated.ClinicID
		}
		syncResult = &RsFormWithFieldsSyncResult{ClinicID: clinicID}

		// Resolve Active Version
		versions, err := s.versionSvc.List(ctx, existing.ID, clinicID)
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

		// --- FIELD DELETION ---
		forceDelete := req.ForceDelete != nil && *req.ForceDelete
		for _, id := range req.Delete {
			if s.entryRepo != nil && !forceDelete {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
				if err != nil {
					return err
				}
				if has {
					return fmt.Errorf("field %s has submitted entries; use force_delete to override", id)
				}
			}
			if err := s.fieldSvc.Delete(ctx, tx, id); err != nil {
				return err
			}
			syncResult.DeletedCount++
		}

		// Build key mapping for Formulas
		keyToFieldID := make(map[string]uuid.UUID)
		deletedMap := make(map[uuid.UUID]bool)
		for _, id := range req.Delete {
			deletedMap[id] = true
		}

		existingFields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return err
		}
		for _, f := range existingFields {
			if !deletedMap[f.ID] {
				keyToFieldID[f.FieldKey] = f.ID
			}
		}

		// --- FIELD UPDATES ---
		for _, item := range req.Update {
			item.Sanitize()
			fUpd, err := s.fieldSvc.Update(ctx, tx, item.ID, req.ClinicID, realOwnerID, &item)
			if err != nil {
				return fmt.Errorf("update failed for field %s: %w", item.ID, err)
			}
			// Use the existing field key (field_key is immutable)
			// If duplicate keys exist, the last one wins
			keyToFieldID[fUpd.FieldKey] = fUpd.ID
			syncResult.UpdatedCount++
		}

		// --- FIELD CREATION ---
		for _, item := range req.Create {
			item.Sanitize()
			if err := item.Validate(); err != nil {
				return fmt.Errorf("validation failed for field %s: %w", item.FieldKey, err)
			}
			created, err := s.fieldSvc.Create(ctx, tx, activeVersionID, &req.ClinicID, actorID, item.ToRqFormField())
			if err != nil {
				return fmt.Errorf("failed to create field %s: %w", item.FieldKey, err)
			}
			// If duplicate keys exist, the last one wins
			keyToFieldID[item.FieldKey] = created.ID
			syncResult.CreatedCount++
		}

		// --- FORMULA SYNC ---
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

	// Shared Event Recording (Triggered only for successful Accountant actions)
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

	// Audit Logging
	idStr := updated.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionFormUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  lo.ToPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  updated,
	})

	// Send notification
	if meta.UserID != nil && meta.UserType != nil {
		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			actorType := util.ActorPractitioner
			if strings.EqualFold(*meta.UserType, util.RoleAccountant) {
				actorType = util.ActorAccountant
			}

			if err := s.notifyForm(ctx, updated.ID, actorUserID, actorType, util.EventFormUpdated, updated.Name); err != nil {
				log.Printf("[WARN] failed to send form update notification: %v", err)
			}
		}
	}

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
			if err := s.fieldSvc.Delete(ctx, tx, fieldID); err != nil {
				return err
			}
			result.Deleted = append(result.Deleted, fieldID)
		}

		for _, updateItem := range req.Update {
			updateItem.Sanitize()
			updated, err := s.fieldSvc.Update(ctx, tx, updateItem.ID, req.ClinicID, practitionerID, &updateItem)
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
			created, err := s.fieldSvc.Create(ctx, tx, activeVersionID, &req.ClinicID, practitionerID, createItem.ToRqFormField())
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
	clinicID := uuid.Nil
	if formDetail.ClinicID != nil {
		clinicID = *formDetail.ClinicID
	}
	versions, err := s.versionSvc.List(ctx, formDetail.ID, clinicID)
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

	// Get clinic info and owner ID only for non-expense forms
	var realOwnerID uuid.UUID
	if formDetail.Method != "EXPENSE_ENTRY" {
		clinicID := uuid.Nil
		if formDetail.ClinicID != nil {
			clinicID = *formDetail.ClinicID
		}
		clinic, err := s.formClinic.GetClinicByIDInternal(ctx, clinicID)
		if err != nil {
			return fmt.Errorf("failed to resolve clinic owner: %w", err)
		}
		realOwnerID = clinic.PractitionerID
	}

	// TRANSACTIONAL DELETION
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete associated permissions for this Form
		if err := s.invitationSvc.DeletePermission(ctx, tx, formID); err != nil {
			return err
		}

		// Delete the Form
		if err := s.detailSvc.Delete(ctx, tx, formDetail.ID); err != nil {
			return err
		}

		return nil
	})

	// --- TRIGGER SHARED EVENT RECORD ---
	meta := auditctx.GetMetadata(ctx)
	if formDetail.Method != "EXPENSE_ENTRY" {
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
	}
	// Audit log: form deleted
	idStr := formID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionFormDeleted,
		Module:      auditctx.ModuleForms,
		EntityType:  lo.ToPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: formDetail,
	})

	// Send notification (form deletion is communicated as EventFormUpdated)
	if formDetail.Method != "EXPENSE_ENTRY" && meta.UserID != nil && meta.UserType != nil {
		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			actorType := util.ActorPractitioner
			if strings.EqualFold(*meta.UserType, util.RoleAccountant) {
				actorType = util.ActorAccountant
			}

			if err := s.notifyForm(ctx, formID, actorUserID, actorType, util.EventFormUpdated, formDetail.Name); err != nil {
				log.Printf("[WARN] failed to send form deletion notification: %v", err)
			}
		}
	}

	return nil
}

func (s *service) GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error) {
	detail, err := s.detailSvc.GetByID(ctx, formId, uuid.Nil, "")
	if err != nil {
		return detail, err
	}
	return detail, err
}

func (s *service) UpdateFormStatus(ctx context.Context, formID uuid.UUID, status string) (*detail.RsFormDetail, error) {
	// Fetch current state for audit log and validation
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
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionFormUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  lo.ToPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: map[string]string{"status": existing.Status},
		AfterState:  map[string]string{"status": status},
	})

	// Send notification - use EventFormSubmitted if publishing, otherwise EventFormUpdated
	if meta.UserID != nil && meta.UserType != nil {
		actorUserID, err := uuid.Parse(*meta.UserID)
		if err == nil {
			actorType := util.ActorPractitioner
			if strings.EqualFold(*meta.UserType, util.RoleAccountant) {
				actorType = util.ActorAccountant
			}

			eventType := util.EventFormUpdated
			if strings.EqualFold(status, "PUBLISHED") {
				eventType = util.EventFormSubmitted
			}

			if err := s.notifyForm(ctx, formID, actorUserID, actorType, eventType, updated.Name); err != nil {
				log.Printf("[WARN] failed to send form status update notification: %v", err)
			}
		}
	}

	return updated, nil
}

// calculateExpenseAmounts calculates net, GST, and gross amounts based on tax type
func calculateExpenseAmounts(amount, businessUse, taxRate float64, taxType string) (net, gst, gross float64) {
	businessPercent := businessUse / 100.0

	if strings.EqualFold(taxType, "INCLUSIVE") {
		// GST Inclusive: Tax is already inside the amount
		businessGross := amount * businessPercent
		net = businessGross / (1 + taxRate)
		gst = businessGross - net
		gross = businessGross
	} else {
		// GST Exclusive: Tax is added on top
		net = amount * businessPercent
		gst = net * taxRate
		gross = net + gst
	}

	return util.Round(net, 2), util.Round(gst, 2), util.Round(gross, 2)
}

// coaSectionType maps a COA account type name to its section type string.
// Revenue/Income COAs → "COLLECTION"; everything else → "OTHER_COST".
func coaSectionType(accountTypeName string) string {
	t := strings.ToLower(accountTypeName)
	if strings.Contains(t, "revenue") || strings.Contains(t, "income") {
		return "COLLECTION"
	}
	return "OTHER_COST"
}

// resolveTaxRate returns the tax rate (as a decimal) for a COA entry.
// Returns 0.0 when no tax applies.
func resolveTaxRate(ctx context.Context, coaSvc coa.Service, coaDetail *coa.RsChartOfAccount) float64 {
	if !coaDetail.IsTaxable || coaDetail.AccountTaxID <= 0 {
		return 0.0
	}
	taxDetail, err := coaSvc.GetAccountTax(ctx, coaDetail.AccountTaxID)
	if err != nil || taxDetail == nil {
		return 0.0
	}
	return taxDetail.Rate / 100.0
}

func (s *service) CreateExpense(ctx context.Context, rq RqExpense, actorId uuid.UUID, role string) (*detail.RsFormDetail, error) {
	var OwnerID uuid.UUID

	switch role {
	case util.RoleAccountant:
		if len(rq.Items) == 0 {
			return nil, errors.New("at least one expense item is required")
		}
		firstCoa, err := s.coaSvc.GetByIDInternal(ctx, rq.Items[0].CoaID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve expense owner (COA: %s, Actor: %s): %w",
				rq.Items[0].CoaID, actorId, err)
		}
		OwnerID = firstCoa.PractitionerID
	default:
		OwnerID = actorId
	}

	// Validate each item's date against FY and lock date
	for idx, item := range rq.Items {
		itemDate, err := util.ParseFlexibleDate(item.Date)
		if err != nil {
			return nil, fmt.Errorf("item %d: invalid date format: %w", idx, err)
		}

		fy, err := s.financialRepo.GetFinancialYearByDate(ctx, itemDate)
		if err != nil {
			return nil, fmt.Errorf("item %d: the date %s does not fall within an active financial year", idx, itemDate.Format("02-01-2006"))
		}

		lockDateStr, err := s.practitionerSvc.GetLockDate(ctx, OwnerID, fy.ID)
		if err != nil {
			return nil, fmt.Errorf("item %d: failed to verify lock date: %w", idx, err)
		}

		if lockDateStr != nil && *lockDateStr != "" {
			lockDate, err := util.ParseFlexibleDate(*lockDateStr)
			if err != nil {
				return nil, fmt.Errorf("item %d: invalid lock date format: %w", idx, err)
			}
			if !itemDate.After(lockDate) {
				return nil, fmt.Errorf("item %d: cannot create expense: the financial period for %s is locked", idx, *lockDateStr)
			}
		}
	}

	var createdForm *detail.RsFormDetail

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		rqDetail := detail.RqFormDetail{
			Name:        rq.Name,
			ClinicShare: 0,
			Method:      "EXPENSE_ENTRY",
			OwnerShare:  100,
			Status:      "PUBLISHED",
		}

		form, err := s.detailSvc.CreateTx(ctx, tx, &rqDetail, nil, OwnerID)
		if err != nil {
			return fmt.Errorf("failed to create expense form: %w", err)
		}
		createdForm = form

		if form.ActiveVersionID == nil {
			return errors.New("active version not found for expense form")
		}

		status := entry.EntryStatusSubmitted

		var submittedAt *string
		if status == EntryStatusSubmitted {
			nowStr := time.Now().UTC().Format(time.RFC3339)
			submittedAt = &nowStr
		}
		entryID := uuid.New()
		firstItemDate := rq.Items[0].Date
		formEntry := &entry.FormEntry{
			ID:            entryID,
			FormVersionID: *form.ActiveVersionID,
			ClinicID:      uuid.Nil,
			SubmittedBy:   &actorId,
			Status:        status,
			SubmittedAt:   submittedAt,
			Date:          &firstItemDate,
		}

		var entryValues []*entry.FormEntryValue
		var allDocIDs []uuid.UUID
		coaDetails := make([]*coa.RsChartOfAccount, len(rq.Items))

		for idx, item := range rq.Items {
			coaDetail, err := s.coaSvc.GetChartOfAccount(ctx, item.CoaID, OwnerID)
			if err != nil {
				return fmt.Errorf("failed to get COA details for item %d: %w", idx, err)
			}
			coaDetails[idx] = coaDetail

			taxType := "EXCLUSIVE"
			if item.TaxType != nil && *item.TaxType != "" {
				taxType = strings.ToUpper(*item.TaxType)
			}

			localBusinessUse := item.BusinessUse
			netAmount, gstAmount, grossAmount := calculateExpenseAmounts(
				item.Amount,
				localBusinessUse,
				resolveTaxRate(ctx, s.coaSvc, coaDetail),
				taxType,
			)

			sectionType := coaSectionType(coaDetail.AccountTypeName)

			rsField, err := s.fieldSvc.Create(ctx, tx, *form.ActiveVersionID, nil, OwnerID, &field.RqFormField{
				FieldKey:    fmt.Sprintf("E%d", idx+1),
				Label:       item.Name,
				CoaID:       item.CoaID.String(),
				IsComputed:  false,
				SortOrder:   idx,
				BusinessUse: &localBusinessUse,
				TaxType:     &taxType,
				SectionType: sectionType,
				Amount:      &item.Amount,
			})
			if err != nil {
				return fmt.Errorf("failed to create field for item %d: %w", idx, err)
			}

			itemDate := item.Date
			entryValues = append(entryValues, &entry.FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            entryID,
				FormFieldID:        &rsField.ID,
				NetAmount:          &netAmount,
				GstAmount:          &gstAmount,
				GrossAmount:        &grossAmount,
				Description:        item.Description,
				BusinessPercentage: &localBusinessUse,
				Date:               &itemDate,
			})

			// Collect document IDs for the current expense item
			if len(item.DocumentIDs) > 0 {
				parsedIDs, parseErr := util.ParseUUIDs(item.DocumentIDs)
				if parseErr != nil {
					return fmt.Errorf("item %d: invalid document id structure: %w", idx, parseErr)
				}
				allDocIDs = append(allDocIDs, parsedIDs...)
			}
		}

		// Create the entry with all values first
		if err := s.entryRepo.Create(ctx, tx, formEntry, entryValues); err != nil {
			return fmt.Errorf("failed to create expense entry: %w", err)
		}

		// Link collected unique document links to this entry ID
		if len(allDocIDs) > 0 {
			allDocIDs = lo.Uniq(allDocIDs) // De-duplicate entries across elements
			if linkErr := s.entryRepo.LinkDocuments(ctx, tx, entryID, allDocIDs); linkErr != nil {
				return fmt.Errorf("failed to link batch documents for expense: %w", linkErr)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit log
	idStr := createdForm.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionFormCreated,
		Module:     auditctx.ModuleForms,
		EntityType: lo.ToPtr(auditctx.EntityForm),
		EntityID:   &idStr,
		AfterState: createdForm,
	})

	return createdForm, nil
}

func (s *service) UpdateExpense(ctx context.Context, formID uuid.UUID, rq RqUpdateExpense, actorId uuid.UUID) (*detail.RsFormDetail, error) {

	// Get existing form first (we need this to find the practitioner)
	existingForm, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get expense form: %w", err)
	}

	// Extract the PractitionerID from the active version
	var practitionerID uuid.UUID
	versions, err := s.versionSvc.List(ctx, formID, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get form versions: %w", err)
	}
	for _, v := range versions {
		if v.IsActive {
			practitionerID = v.PractitionerID
			break
		}
	}

	if practitionerID == uuid.Nil {
		return nil, errors.New("could not determine practitioner for this expense")
	}

	// Extract date from the first item (update takes priority over create)
	var rawDate string
	if len(rq.Update) > 0 && rq.Update[0].Date != nil {
		rawDate = *rq.Update[0].Date
	} else if len(rq.Create) > 0 {
		rawDate = rq.Create[0].Date
	}

	// Validate each update item's date
	for idx, item := range rq.Update {
		if item.Date == nil || *item.Date == "" {
			continue
		}
		itemDate, err := util.ParseFlexibleDate(*item.Date)
		if err != nil {
			return nil, fmt.Errorf("update item %d: invalid date format: %w", idx, err)
		}
		fy, err := s.financialRepo.GetFinancialYearByDate(ctx, itemDate)
		if err != nil {
			return nil, fmt.Errorf("update item %d: the date %s does not fall within an active financial year", idx, itemDate.Format("02-01-2006"))
		}
		lockDateStr, err := s.practitionerSvc.GetLockDate(ctx, practitionerID, fy.ID)
		if err != nil {
			return nil, fmt.Errorf("update item %d: failed to verify lock date: %w", idx, err)
		}
		if lockDateStr != nil && *lockDateStr != "" {
			lockDate, err := util.ParseFlexibleDate(*lockDateStr)
			if err != nil {
				return nil, fmt.Errorf("update item %d: invalid lock date format: %w", idx, err)
			}
			if !itemDate.After(lockDate) {
				return nil, fmt.Errorf("update item %d: cannot update expense: the financial period for %s is locked", idx, *lockDateStr)
			}
		}
	}

	// Validate each create item's date
	for idx, item := range rq.Create {
		if item.Date == "" {
			continue
		}
		itemDate, err := util.ParseFlexibleDate(item.Date)
		if err != nil {
			return nil, fmt.Errorf("create item %d: invalid date format: %w", idx, err)
		}
		fy, err := s.financialRepo.GetFinancialYearByDate(ctx, itemDate)
		if err != nil {
			return nil, fmt.Errorf("create item %d: the date %s does not fall within an active financial year", idx, itemDate.Format("02-01-2006"))
		}
		lockDateStr, err := s.practitionerSvc.GetLockDate(ctx, practitionerID, fy.ID)
		if err != nil {
			return nil, fmt.Errorf("create item %d: failed to verify lock date: %w", idx, err)
		}
		if lockDateStr != nil && *lockDateStr != "" {
			lockDate, err := util.ParseFlexibleDate(*lockDateStr)
			if err != nil {
				return nil, fmt.Errorf("create item %d: invalid lock date format: %w", idx, err)
			}
			if !itemDate.After(lockDate) {
				return nil, fmt.Errorf("create item %d: cannot update expense: the financial period for %s is locked", idx, *lockDateStr)
			}
		}
	}

	_ = rawDate // entry-level date updated below from first item if present

	if existingForm.Method != "EXPENSE_ENTRY" {
		return nil, errors.New("form is not an expense entry")
	}

	beforeState := *existingForm
	var updatedForm *detail.RsFormDetail

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Update form name if changed
		if rq.Name != existingForm.Name {
			updateReq := &detail.RqUpdateFormDetail{
				ID:   formID,
				Name: &rq.Name,
			}
			upd, err := s.detailSvc.UpdateMetadata(ctx, updateReq)
			if err != nil {
				return fmt.Errorf("failed to update form name: %w", err)
			}
			updatedForm = upd
		} else {
			updatedForm = existingForm
		}

		// Get active version ID
		var practitionerID uuid.UUID
		var activeVersionID uuid.UUID
		if existingForm.ActiveVersionID != nil {
			activeVersionID = *existingForm.ActiveVersionID
		} else {
			// Fallback: fetch from version list if not set in form
			versions, err := s.versionSvc.List(ctx, formID, uuid.Nil)
			if err != nil {
				return fmt.Errorf("failed to get form versions: %w", err)
			}
			for _, v := range versions {
				if v.IsActive {
					practitionerID = v.PractitionerID
					activeVersionID = v.Id
					break
				}
			}
		}

		if activeVersionID == uuid.Nil {
			return errors.New("active version not found for expense form")
		}

		// Get existing entry
		existingEntry, existingValues, err := s.entryRepo.GetByVersionID(ctx, activeVersionID)
		if err != nil {
			return fmt.Errorf("failed to get existing entry: %w", err)
		}

		// Handle deletions
		for _, fieldID := range rq.Delete {
			if err := s.fieldSvc.Delete(ctx, tx, fieldID); err != nil {
				return fmt.Errorf("failed to delete field %s: %w", fieldID, err)
			}
		}

		// Handle updates
		for idx, item := range rq.Update {
			// Get existing field
			existingField, err := s.fieldSvc.GetByID(ctx, item.ID)
			if err != nil {
				return fmt.Errorf("failed to get field %s: %w", item.ID, err)
			}

			// Prepare update data
			var coaID uuid.UUID
			if existingField.CoaID != nil {
				coaID = *existingField.CoaID
			}
			if item.CoaID != nil {
				coaID = *item.CoaID
			}

			amount := 0.0
			businessUse := 0.0
			var description *string

			// Find existing entry value for this field
			for _, ev := range existingValues {
				if ev.FormFieldID == &item.ID {
					if ev.GrossAmount != nil {
						amount = *ev.GrossAmount
					}
					description = ev.Description
					break
				}
			}

			if existingField.BusinessUse != nil {
				businessUse = *existingField.BusinessUse
			}

			if item.Amount != nil {
				amount = *item.Amount
			}
			if item.BusinessUse != nil {
				businessUse = *item.BusinessUse
			}
			if item.Description != nil {
				description = item.Description
			}

			// Get COA details
			coaDetail, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID)
			if err != nil {
				return fmt.Errorf("failed to get COA details: %w", err)
			}

			taxType := "EXCLUSIVE"

			// Use tax type from request if provided, otherwise fall back to existing field value
			if item.TaxType != nil && *item.TaxType != "" {
				taxType = strings.ToUpper(*item.TaxType)
			} else if existingField.TaxType != nil && *existingField.TaxType != "" {
				taxType = *existingField.TaxType
			}

			taxRate := resolveTaxRate(ctx, s.coaSvc, coaDetail)

			netAmount, gstAmount, grossAmount := calculateExpenseAmounts(amount, businessUse, taxRate, taxType)

			// Update field
			label := existingField.Label
			if item.Name != nil {
				label = *item.Name
			}

			coaIDStr := coaID.String()
			updateFieldReq := field.RqUpdateFormField{
				ID:          item.ID,
				Label:       &label,
				CoaID:       &coaIDStr,
				BusinessUse: &businessUse,
				TaxType:     &taxType,
				Amount:      &amount,
			}

			if _, err := s.fieldSvc.Update(ctx, tx, item.ID, uuid.Nil, practitionerID, &updateFieldReq); err != nil {
				return fmt.Errorf("failed to update field: %w", err)
			}

			// Find existing entry value for this field to preserve date if not provided
			var existingDate *string
			for _, ev := range existingValues {
				if ev.FormFieldID != nil && *ev.FormFieldID == item.ID && ev.UpdatedAt == nil {
					existingDate = ev.Date
					break
				}
			}

			// Update entry value - mark old as updated and insert new
			if err := s.entryRepo.MarkEntryValueUpdated(ctx, tx, item.ID, existingEntry.ID); err != nil {
				return fmt.Errorf("failed to mark old entry value: %w", err)
			}

			// Use new date if provided, otherwise preserve the existing item-level date
			dateToUse := item.Date
			if dateToUse == nil {
				dateToUse = existingDate
			}

			// Preserve notes if not provided in update

			if err := s.entryRepo.InsertEntryValue(ctx, tx, &entry.FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            existingEntry.ID,
				FormFieldID:        &item.ID,
				NetAmount:          &netAmount,
				GstAmount:          &gstAmount,
				GrossAmount:        &grossAmount,
				Description:        description,
				BusinessPercentage: &businessUse,
				Date:               dateToUse,
			}); err != nil {
				return fmt.Errorf("failed to insert updated entry value: %w", err)
			}

			// Document additions/removals specified on this individual updated item
			if item.DocumentIDs != nil {
				if len(item.DocumentIDs.Create) > 0 {
					docIDs, parseErr := util.ParseUUIDs(item.DocumentIDs.Create)
					if parseErr != nil {
						return fmt.Errorf("update item %d: invalid document link targets: %w", idx, parseErr)
					}
					if linkErr := s.entryRepo.LinkDocuments(ctx, tx, existingEntry.ID, docIDs); linkErr != nil {
						return fmt.Errorf("failed to process update link document ops: %w", linkErr)
					}
				}
				if len(item.DocumentIDs.Delete) > 0 {
					docIDs, parseErr := util.ParseUUIDs(item.DocumentIDs.Delete)
					if parseErr != nil {
						return fmt.Errorf("update item %d: invalid document unlink targets: %w", idx, parseErr)
					}
					if unlinkErr := s.entryRepo.UnlinkDocuments(ctx, tx, existingEntry.ID, docIDs); unlinkErr != nil {
						return fmt.Errorf("failed to process update unlink document ops: %w", unlinkErr)
					}
				}
			}
		}

		// Update the entry-level date from the first updated/created item (for filtering)
		var entryDateStr *string
		if len(rq.Update) > 0 && rq.Update[0].Date != nil && *rq.Update[0].Date != "" {
			entryDateStr = rq.Update[0].Date
		} else if len(rq.Create) > 0 && rq.Create[0].Date != "" {
			entryDateStr = &rq.Create[0].Date
		}
		if entryDateStr != nil {
			if err := s.entryRepo.UpdateEntryDate(ctx, tx, existingEntry.ID, *entryDateStr); err != nil {
				return fmt.Errorf("failed to update entry date: %w", err)
			}
		}

		// Handle creates
		for idx, item := range rq.Create {
			coaDetail, err := s.coaSvc.GetChartOfAccount(ctx, item.CoaID, practitionerID)
			if err != nil {
				return fmt.Errorf("failed to get COA details for new item %d: %w", idx, err)
			}

			taxType := "EXCLUSIVE"
			if item.TaxType != nil && *item.TaxType != "" {
				taxType = strings.ToUpper(*item.TaxType)
			}

			netAmount, gstAmount, grossAmount := calculateExpenseAmounts(
				item.Amount, item.BusinessUse, resolveTaxRate(ctx, s.coaSvc, coaDetail), taxType,
			)

			rsField, err := s.fieldSvc.Create(ctx, tx, activeVersionID, nil, practitionerID, &field.RqFormField{
				FieldKey:    fmt.Sprintf("N%d", idx+1),
				Label:       item.Name,
				CoaID:       item.CoaID.String(),
				IsComputed:  false,
				BusinessUse: &item.BusinessUse,
				TaxType:     &taxType,
				SectionType: coaSectionType(coaDetail.AccountTypeName),
				Amount:      &item.Amount,
			})
			if err != nil {
				return fmt.Errorf("failed to create new field: %w", err)
			}

			if err := s.entryRepo.InsertEntryValue(ctx, tx, &entry.FormEntryValue{
				ID:                 uuid.New(),
				EntryID:            existingEntry.ID,
				FormFieldID:        &rsField.ID,
				NetAmount:          &netAmount,
				GstAmount:          &gstAmount,
				GrossAmount:        &grossAmount,
				Description:        item.Description,
				BusinessPercentage: &item.BusinessUse,
				Date:               &item.Date,
			}); err != nil {
				return fmt.Errorf("failed to insert new entry value for item %d: %w", idx, err)
			}

			if len(item.DocumentIDs) > 0 {
				docIDs, parseErr := util.ParseUUIDs(item.DocumentIDs)
				if parseErr != nil {
					return fmt.Errorf("create item %d: invalid document link configuration: %w", idx, parseErr)
				}
				if linkErr := s.entryRepo.LinkDocuments(ctx, tx, existingEntry.ID, docIDs); linkErr != nil {
					return fmt.Errorf("failed to process creation document links: %w", linkErr)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Audit log
	idStr := formID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionFormUpdated,
		Module:      auditctx.ModuleForms,
		EntityType:  lo.ToPtr(auditctx.EntityForm),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  updatedForm,
	})

	return updatedForm, nil
}

func (s *service) GetExpense(ctx context.Context, formID uuid.UUID, actorId uuid.UUID, role string) (*RsExpense, error) {
	// Get form details
	formDetail, err := s.detailSvc.GetByID(ctx, formID, uuid.Nil, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense form: %w", err)
	}

	// Verify it's an expense form
	if formDetail.Method != "EXPENSE_ENTRY" {
		return nil, errors.New("form is not an expense entry")
	}

	// Verify ownership
	// Get form version to check practitioner_id and get active version ID
	versions, err := s.versionSvc.List(ctx, formID, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get form versions: %w", err)
	}

	var practitionerID uuid.UUID
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			practitionerID = v.PractitionerID
			activeVersionID = v.Id
			break
		}
	}

	if activeVersionID == uuid.Nil {
		return nil, errors.New("active version not found for expense form")
	}

	// Check if actor owns this expense
	if role == util.RolePractitioner {
		if practitionerID != actorId {
			return nil, errors.New("access denied: you do not own this expense")
		}
	} else {
		// For accountants, verify they are linked to the practitioner
		isLinked, err := s.invitationSvc.IsAccountantLinkedToPractitioner(ctx, practitionerID, actorId)
		if err != nil || !isLinked {
			return nil, errors.New("access denied: you do not have access to this expense")
		}
	}

	// Get form fields
	fields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}

	// Get entry and entry values
	existingEntry, entryValues, err := s.entryRepo.GetByVersionID(ctx, activeVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry values: %w", err)
	}

	// Build response
	response := &RsExpense{
		ID:             formDetail.ID,
		Name:           formDetail.Name,
		PractitionerID: practitionerID,
		Items:          []RsExpenseItem{},
		Documents:      []RsExpenseDocument{},
		CreatedAt:      formDetail.CreatedAt,
	}

	if formDetail.UpdatedAt != "" {
		response.UpdatedAt = &formDetail.UpdatedAt
	}

	// Determine amount_type from the first field's tax type
	taxType := "EXCLUSIVE" // Default
	if len(fields) > 0 && fields[0].TaxType != nil {
		taxType = *fields[0].TaxType
	}
	response.TaxType = taxType

	// Build items from fields and entry values
	for _, f := range fields {
		if f.CoaID == nil {
			continue
		}

		// Get COA details for name
		coaDetail, err := s.coaSvc.GetChartOfAccount(ctx, *f.CoaID, practitionerID)
		if err != nil {
			continue
		}

		// Find matching entry value
		var netAmount, gstAmount, grossAmount float64
		var description *string
		var itemDate string
		for _, ev := range entryValues {
			if ev.FormFieldID != nil && *ev.FormFieldID == f.ID && ev.UpdatedAt == nil {
				if ev.NetAmount != nil {
					netAmount = *ev.NetAmount
				}
				if ev.GstAmount != nil {
					gstAmount = *ev.GstAmount
				}
				if ev.GrossAmount != nil {
					grossAmount = *ev.GrossAmount
				}
				description = ev.Description
				if ev.Date != nil && *ev.Date != "" {
					// Use item-level date (primary source for expense entries)
					itemDate = *ev.Date
				}
				break
			}
		}

		businessUse := 0.0
		if f.BusinessUse != nil {
			businessUse = *f.BusinessUse
		}

		// Safe Dereference for Amount
		displayAmount := grossAmount // Default to calculated gross
		if f.Amount != nil {
			displayAmount = *f.Amount
		}

		item := RsExpenseItem{
			ID:          f.ID,
			Name:        f.Label,
			CoaID:       *f.CoaID,
			CoaName:     coaDetail.Name,
			BusinessUse: businessUse,
			Amount:      displayAmount,
			NetAmount:   netAmount,
			GstAmount:   gstAmount,
			GrossAmount: grossAmount,
			Date:        itemDate,
			Description: description,
			Notes:       description,
		}

		response.Items = append(response.Items, item)
	}

	// Fetching Documents
	if existingEntry != nil {
		docs, docErr := s.entryRepo.GetDocumentsByEntryID(ctx, existingEntry.ID)
		if docErr == nil && docs != nil {
			response.Documents = make([]RsExpenseDocument, 0, len(docs))
			for _, d := range docs {
				if d != nil {
					// Use JSON marshaling to safely translate types across package boundaries via common tags
					var targetDoc RsExpenseDocument
					bytes, err := json.Marshal(d)
					if err == nil {
						if err := json.Unmarshal(bytes, &targetDoc); err == nil {
							response.Documents = append(response.Documents, targetDoc)
						}
					}
				}
			}
		}
	}

	return response, nil
}

// notifyForm sends notifications to linked users about form operations
func (s *service) notifyForm(ctx context.Context, formID uuid.UUID, actorID uuid.UUID, actorType util.ActorType, eventType util.EventType, formName string) error {
	if s.notificationPub == nil {
		log.Printf("[WARN] notification publisher is nil, skipping form notification")
		return nil
	}

	// Get sender information
	user, err := s.authSvc.GetUserByID(ctx, actorID, actorType)
	if err != nil {
		log.Printf("[WARN] failed to get user for notification: %v", err)
		return nil
	}
	senderName := user.FirstName + " " + user.LastName

	// Build recipients list
	recipients := []sharednotification.RecipientWithPreferences{}

	switch actorType {
	case util.ActorPractitioner:
		// If the sender is a practitioner, notify all their linked accountants
		accountants, err := s.invitationRepo.GetAccountantsLinkedToPractitioner(ctx, actorID)
		if err != nil {
			log.Printf("[WARN] failed to get linked accountants for practitioner %s: %v", actorID, err)
			return nil
		}

		for _, acc := range accountants {
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   acc.AccountantID,
				RecipientType: util.ActorAccountant,
				UserID:        acc.UserID,
			})
		}

	case util.ActorAccountant:
		// If the sender is an accountant, notify all linked practitioners
		practitionerIDs, err := s.invitationRepo.GetPractitionersLinkedToAccountant(ctx, actorID)
		if err != nil {
			log.Printf("[WARN] failed to get practitioners for accountant %s: %v", actorID, err)
			return nil
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
		log.Printf("[WARN] unsupported actor type for form notification: %s", actorType)
		return nil
	}

	// If no recipients, don't send notification
	if len(recipients) == 0 {
		log.Printf("[INFO] no recipients found for form notification")
		return nil
	}

	// Determine title based on event type
	title := "Form Updated"
	if eventType == util.EventFormSubmitted {
		title = "Form Submitted"
	}

	// Send notifications with preferences using the publisher
	err = s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   actorID,
		SenderType: actorType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityForm,
		EntityID:   formID,
		EntityKey:  "form_id",
		Title:      title,
		Body:       fmt.Sprintf("%s: %s by %s", title, formName, senderName),
		ExtraData:  map[string]any{"form_name": formName},
	})

	if err != nil {
		log.Printf("[WARN] failed to send form notification: %v", err)
	}

	return nil
}
