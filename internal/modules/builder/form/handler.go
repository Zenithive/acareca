package form

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Sync(c *gin.Context)
	CreateFormWithFields(c *gin.Context)
	UpdateFormWithFields(c *gin.Context)
	GetFormWithFields(c *gin.Context)
	List(c *gin.Context)
	Delete(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// @Summary Bulk sync fields
// @Description Synchronize multiple fields for a practitioner
// @Tags form
// @Accept json
// @Produce json
// @Param request body RqBulkSyncFields true "Sync request"
// @Success 200 {object} RsBulkSyncFields
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/sync [post]
func (h *handler) Sync(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqBulkSyncFields
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.BulkSyncFields(c.Request.Context(), practitionerID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Fields synchronized successfully")
}

// @Summary Create form with fields
// @Description Create a new custom form along with its associated fields
// @Tags form
// @Accept json
// @Produce json
// @Param request body RqCreateFormWithFields true "Form creation request"
// @Success 201 {object} RsFormWithFields
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form [post]
func (h *handler) CreateFormWithFields(c *gin.Context) {
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
		req.Status = detail.StatusDraft
	}
	form, syncResult, err := h.svc.CreateWithFields(c.Request.Context(), &req, practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, gin.H{"form": form, "fields_sync": syncResult}, "Form created successfully")
}

// @Summary Update form with fields
// @Description Update an existing form and sync its fields
// @Tags form
// @Accept json
// @Produce json
// @Param request body RqUpdateFormWithFields true "Form update request"
// @Success 200 {object} RsFormWithFields
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form [put]
func (h *handler) UpdateFormWithFields(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if formID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, errors.New("form id is required"))
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
	req.ID = &formID
	form, syncResult, err := h.svc.UpdateWithFields(c.Request.Context(), &req, practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"form": form, "fields_sync": syncResult}, "Form updated successfully")
}

// @Summary Get form by ID
// @Description Retrieve a specific form and its fields by ID
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Success 200 {object} RsFormWithFields
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [get]
func (h *handler) GetFormWithFields(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	out, err := h.svc.GetFormWithFields(c.Request.Context(), formID)
	if err != nil {
		if errors.Is(err, detail.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, out, "Form fetched successfully")
}

// @Summary List forms
// @Description List forms filtered by clinic and query parameters
// @Tags form
// @Accept json
// @Produce json
// @Param clinic_id query string false "Clinic ID"
// @Success 200 {array} detail.RsFormDetail
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form [get]
func (h *handler) List(c *gin.Context) {
	clinicId, err := util.ParseUUID(c.Query("clinic_id"))
	if err != nil {
		return
	}

	var filter Filter
	if clinicId != uuid.Nil {
		filter.ClinicID = &clinicId
	}
	if err := util.BindAndValidate(c, &filter); err != nil {
		fmt.Println(err.Error())
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Forms fetched successfully")
}

// @Summary Delete form
// @Description Remove a form by its ID
// @Tags form
// @Accept json
// @Produce json
// @Param id path string true "Form ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /form/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), formID); err != nil {
		if errors.Is(err, detail.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil, "Form deleted successfully")
}
