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
	DeleteClinic(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

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

func (h *handler) UpdateClinic(c *gin.Context) {
	// Get user ID from JWT token context
	PractID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	fmt.Printf("UpdateClinic Handler - Extracted userID from JWT: %s\n", PractID)

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic id"))
		return
	}

	fmt.Printf("UpdateClinic Handler - Clinic ID to update: %s\n", id.String())

	var req RqUpdateClinic
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	clinic, err := h.svc.UpdateClinic(c.Request.Context(), PractID, id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			fmt.Printf("UpdateClinic Handler - Clinic not found or access denied\n")
			response.Error(c, http.StatusNotFound, err)
			return
		}
		fmt.Printf("UpdateClinic Handler - Internal server error: %v\n", err)
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	fmt.Printf("UpdateClinic Handler - Successfully updated clinic\n")
	response.JSON(c, http.StatusOK, clinic)
}

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
