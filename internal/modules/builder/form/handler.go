package form

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
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
	response.JSON(c, http.StatusOK, result)
}

func (h *handler) CreateFormWithFields(c *gin.Context) {
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
	req.ClinicID = clinicID
	if req.Status == "" {
		req.Status = detail.StatusDraft
	}
	form, syncResult, err := h.svc.CreateWithFields(c.Request.Context(), &req, practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, gin.H{"form": form, "fields_sync": syncResult})
}

func (h *handler) UpdateFormWithFields(c *gin.Context) {

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
	req.ClinicID = clinicID
	form, syncResult, err := h.svc.UpdateWithFields(c.Request.Context(), &req, practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"form": form, "fields_sync": syncResult})
}

func (h *handler) GetFormWithFields(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	out, err := h.svc.GetFormWithFields(c.Request.Context(), formID, clinicID)
	if err != nil {
		if errors.Is(err, detail.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, out)
}

func (h *handler) List(c *gin.Context) {
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	filter := detail.Filter{ClinicID: clinicID}
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	filter.ClinicID = clinicID
	list, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

func (h *handler) Delete(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), formID, clinicID); err != nil {
		if errors.Is(err, detail.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil)
}
