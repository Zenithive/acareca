package clinic

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreateClinic(c *gin.Context)
	GetClinics(c *gin.Context)
	GetClinicByID(c *gin.Context)
	UpdateClinic(c *gin.Context)
	BulkUpdateClinics(c *gin.Context)
	DeleteClinic(c *gin.Context)
	BulkDeleteClinics(c *gin.Context)
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
// @Router /clinic/create [post]
func (h *handler) CreateClinic(c *gin.Context) {
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
		// Log the detailed error for debugging
		fmt.Printf("CreateClinic error: %v\n", err)
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, clinic)
}

// @Summary Get all clinics for practitioner
// @Tags clinic
// @Produce json
// @Success 200 {array} RsClinic
// @Failure 500 {object} response.RsError
// @Router /clinic/all [get]
func (h *handler) GetClinics(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	clinics, err := h.svc.GetClinics(c.Request.Context(), PractID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, clinics)
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
// @Router /clinic/{id} [get]
func (h *handler) GetClinicByID(c *gin.Context) {
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

	response.JSON(c, http.StatusOK, clinic)
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
// @Router /clinic/{id} [put]
func (h *handler) UpdateClinic(c *gin.Context) {
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

	response.JSON(c, http.StatusOK, clinic)
}

// @Summary Delete a clinic
// @Tags clinic
// @Produce json
// @Param id path string true "Clinic UUID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /clinic/{id} [delete]
func (h *handler) DeleteClinic(c *gin.Context) {
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

	response.JSON(c, http.StatusOK, gin.H{"message": "clinic deleted successfully"})
}
func (h *handler) BulkUpdateClinics(c *gin.Context) {
	// Get user ID from JWT token context
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

	response.JSON(c, http.StatusOK, gin.H{"clinics": clinics})
}

func (h *handler) BulkDeleteClinics(c *gin.Context) {
	// Get user ID from JWT token context
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

	response.JSON(c, http.StatusOK, gin.H{"message": "clinics deleted successfully"})
}
