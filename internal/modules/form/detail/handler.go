package detail

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	formpkg "github.com/iamarpitzala/acareca/internal/modules/form"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// FormWithFieldsOrchestrator creates/updates a form and its fields in one call. Implemented by form/orchestrate.
type FormWithFieldsOrchestrator interface {
	CreateWithFields(ctx context.Context, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqCreateFormWithFields) (*RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, formID uuid.UUID, clinicID uuid.UUID, practitionerID uuid.UUID, req *RqUpdateFormWithFields) (*RsFormDetail, *RsFormWithFieldsSyncResult, error)
}

type IHandler interface {
	CreateForm(c *gin.Context)
	CreateFormWithFields(c *gin.Context)
	GetForm(c *gin.Context)
	ListForm(c *gin.Context)
	UpdateForm(c *gin.Context)
	UpdateFormWithFields(c *gin.Context)
	DeleteForm(c *gin.Context)
}

type handler struct {
	svc  IService
	orch FormWithFieldsOrchestrator
}

func NewHandler(svc IService, orch FormWithFieldsOrchestrator) IHandler {
	return &handler{svc: svc, orch: orch}
}

// CreateForm implements [IHandler].
func (h *handler) CreateForm(c *gin.Context) {
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqFormDetail
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	created, err := h.svc.Create(c.Request.Context(), &req, clinicID, practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

// CreateFormWithFields implements [IHandler]. One-shot create form (DRAFT), version 1, and fields.
func (h *handler) CreateFormWithFields(c *gin.Context) {
	if h.orch == nil {
		response.Error(c, http.StatusNotImplemented, errors.New("create form with fields not configured"))
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqCreateFormWithFields
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	if req.Status == "" {
		req.Status = StatusDraft
	}
	form, syncResult, err := h.orch.CreateWithFields(c.Request.Context(), clinicID, practitionerID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, formpkg.ErrCoaNotFound) || errors.Is(err, formpkg.ErrTooManyFields) || errors.Is(err, formpkg.ErrFieldWrongVersion) {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, formpkg.ErrFormNotDraftForFields) || errors.Is(err, formpkg.ErrFieldHasSubmittedEntries) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"form": form, "fields_sync": syncResult})
}

// DeleteForm implements [IHandler].
func (h *handler) DeleteForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusNoContent, nil)
}

// GetForm implements [IHandler].
func (h *handler) GetForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}

	form, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, form)
}

// ListForm implements [IHandler].
func (h *handler) ListForm(c *gin.Context) {
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	filter := Filter{ClinicID: clinicID}
	forms, err := h.svc.ListForm(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, forms)
}

// UpdateForm implements [IHandler].
func (h *handler) UpdateForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpdateFormDetail
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	req.ID = id
	updated, err := h.svc.Update(c.Request.Context(), &req, practitionerID)
	if err != nil {
		if errors.Is(err, ErrFormArchived) || errors.Is(err, ErrFormPublishedRestricted) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated)
}

// UpdateFormWithFields implements [IHandler]. Update form metadata and sync fields (DRAFT only for field changes).
func (h *handler) UpdateFormWithFields(c *gin.Context) {
	if h.orch == nil {
		response.Error(c, http.StatusNotImplemented, errors.New("update form with fields not configured"))
		return
	}
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpdateFormWithFields
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	form, syncResult, err := h.orch.UpdateWithFields(c.Request.Context(), formID, clinicID, practitionerID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrFormArchived) || errors.Is(err, ErrFormPublishedRestricted) || errors.Is(err, ErrFormNotDraftForFields) || errors.Is(err, formpkg.ErrFormNotDraftForFields) || errors.Is(err, formpkg.ErrFieldHasSubmittedEntries) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, formpkg.ErrCoaNotFound) || errors.Is(err, formpkg.ErrTooManyFields) || errors.Is(err, formpkg.ErrFieldWrongVersion) {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"form": form, "fields_sync": syncResult})
}
