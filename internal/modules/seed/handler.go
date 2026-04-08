package seed

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type IHandler interface {
	SeedData(c *gin.Context)
	CleanupData(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// SeedData godoc
// @Summary Seed test data
// @Description Create test clinics, forms, and formulas for a practitioner
// @Tags Seed
// @Accept json
// @Produce json
// @Param request body RqSeedData true "Seed configuration"
// @Success 200 {object} RsSeedData "Data seeded successfully"
// @Failure 400 {object} response.RsError "Invalid input"
// @Failure 500 {object} response.RsError "Internal server error"
// @Router /seed [post]
func (h *handler) SeedData(c *gin.Context) {
	var req RqSeedData
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate practitioner ID if provided
	var practitionerID *uuid.UUID
	if req.PractitionerID != nil && *req.PractitionerID != "" {
		id, err := uuid.Parse(*req.PractitionerID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		practitionerID = &id
	}

	result, err := h.svc.SeedData(c.Request.Context(), practitionerID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Data seeded successfully")
}

// CleanupData godoc
// @Summary Cleanup seeded data
// @Description Delete all clinics and forms for a practitioner (preserves COA)
// @Tags Seed
// @Accept json
// @Produce json
// @Param request body RqCleanupData true "Cleanup configuration"
// @Success 200 {object} RsCleanupData "Data cleaned up successfully"
// @Failure 400 {object} response.RsError "Invalid input"
// @Failure 500 {object} response.RsError "Internal server error"
// @Router /seed/cleanup [post]
func (h *handler) CleanupData(c *gin.Context) {
	var req RqCleanupData
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Validate practitioner ID
	practitionerID, err := uuid.Parse(req.PractitionerID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.CleanupData(c.Request.Context(), practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Data cleaned up successfully")
}
