package clinic

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	List(c *gin.Context)
	GetByID(c *gin.Context)
	Update(c *gin.Context)
	BulkUpdate(c *gin.Context)
	Delete(c *gin.Context)
	BulkDelete(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary Create a new clinic
// @Tags clinic
// @Accept json
// @Produce json
// @Param request body RqCreateClinic true "Clinic Data"
// @Success 201 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic [post]
func (h *handler) Create(c *gin.Context) {
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqCreateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.CreateClinic(c.Request.Context(), PractID, &req)
	if err != nil {
		if errors.Is(err, limits.ErrLimitReached) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		fmt.Printf("CreateClinic error: %v\n", err)
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, clinic, "Clinic created successfully")
}

// @Summary Get all clinics for practitioner
// @Tags clinic
// @Produce json
// @Param name query string false "Filter by clinic name"
// @Param id query string false "Filter by clinic ID"
// @Param is_active query boolean false "Filter by active status"
// @Param search query string false "Search across name, abn, description"
// @Param sort_by query string false "Sort field (name, is_active, created_at)"
// @Param order_by query string false "Sort direction (ASC, DESC)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic [get]
func (h *handler) List(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	clinics, err := h.svc.ListClinic(c.Request.Context(), PractID, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinics, "Clinics fetched successfully")
}

// GetClinicByID
// @Summary Get clinic by ID
// @Tags clinic
// @Produce json
// @Param id path string true "Clinic UUID"
// @Success 200 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/{id} [get]
func (h *handler) GetByID(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic id"))
		return
	}

	clinic, err := h.svc.GetClinicByID(c.Request.Context(), PractID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic fetched successfully")
}

// @Summary Update clinic details
// @Tags clinic
// @Accept json
// @Produce json
// @Param id path string true "Clinic UUID"
// @Param request body RqUpdateClinic true "Updated Clinic Data"
// @Success 200 {object} RsClinic
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/{id} [put]
func (h *handler) Update(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic id"))
		return
	}

	var req RqUpdateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.UpdateClinic(c.Request.Context(), PractID, id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinic, "Clinic updated successfully")
}

// @Summary Delete a clinic
// @Tags clinic
// @Produce json
// @Param id path string true "Clinic UUID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic id"))
		return
	}

	if err := h.svc.DeleteClinic(c.Request.Context(), PractID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"message": "clinic deleted successfully"}, "Clinic deleted successfully")
}

// @Summary Bulk update clinics
// @Tags clinic
// @Accept json
// @Produce json
// @Param request body RqBulkUpdateClinic true "Bulk Update Data"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/bulk-update [put]
func (h *handler) BulkUpdate(c *gin.Context) {
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqBulkUpdateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinics, err := h.svc.BulkUpdateClinics(c.Request.Context(), PractID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, util.RsList{Items: clinics, Total: len(clinics)}, "Clinics updated successfully")
}

// @Summary Bulk delete clinics
// @Tags clinic
// @Accept json
// @Produce json
// @Param request body RqBulkDeleteClinic true "Bulk Delete Data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/bulk-delete [delete]
func (h *handler) BulkDelete(c *gin.Context) {
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqBulkDeleteClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.BulkDeleteClinics(c.Request.Context(), PractID, &req); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{"message": "clinics deleted successfully"}, "Clinics deleted successfully")
}
