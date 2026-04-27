package equity

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type Handler interface {
	GetOwnerEquityCalculation(c *gin.Context)
	GetRetainedEarnings(c *gin.Context)
	GetEquityMovements(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) Handler {
	return &handler{svc: svc}
}

type EquityFilter struct {
	ClinicID *string `form:"clinic_id"`
	AsOfDate *string `form:"as_of_date"`
}

// GetOwnerEquityCalculation godoc
// @Summary Get Owner Equity Calculation
// @Description Automatically calculates all owner fund balances (drawings, funds introduced, retained earnings)
// @Tags Owner Equity
// @Accept json
// @Produce json
// @Param clinic_id query string false "Filter by clinic UUID"
// @Param as_of_date query string false "Calculate as of date (YYYY-MM-DD), defaults to today"
// @Success 200 {object} OwnerEquityCalculation
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /api/v1/equity/calculation [get]
func (h *handler) GetOwnerEquityCalculation(c *gin.Context) {
	practitionerID, exists := c.Get("practitioner_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, errors.New("practitioner_id not found in context"))
		return
	}

	var filter EquityFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Default to today if no date specified
	asOfDate := time.Now().Format("2006-01-02")
	if filter.AsOfDate != nil && *filter.AsOfDate != "" {
		asOfDate = *filter.AsOfDate
	}

	// Parse clinic ID if provided
	var clinicID *uuid.UUID
	if filter.ClinicID != nil && *filter.ClinicID != "" {
		id, err := uuid.Parse(*filter.ClinicID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid clinic_id"))
			return
		}
		clinicID = &id
	}

	result, err := h.svc.CalculateOwnerEquity(c.Request.Context(), practitionerID.(uuid.UUID), clinicID, asOfDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Owner equity calculated successfully")
}

// GetRetainedEarnings godoc
// @Summary Get Retained Earnings
// @Description Calculates retained earnings from all prior years' profits
// @Tags Owner Equity
// @Accept json
// @Produce json
// @Param clinic_id query string false "Filter by clinic UUID"
// @Param as_of_date query string false "Calculate as of date (YYYY-MM-DD), defaults to today"
// @Success 200 {object} map[string]float64
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /api/v1/equity/retained-earnings [get]
func (h *handler) GetRetainedEarnings(c *gin.Context) {
	practitionerID, exists := c.Get("practitioner_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, errors.New("practitioner_id not found in context"))
		return
	}

	var filter EquityFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Default to today if no date specified
	asOfDate := time.Now().Format("2006-01-02")
	if filter.AsOfDate != nil && *filter.AsOfDate != "" {
		asOfDate = *filter.AsOfDate
	}

	// Parse clinic ID if provided
	var clinicID *uuid.UUID
	if filter.ClinicID != nil && *filter.ClinicID != "" {
		id, err := uuid.Parse(*filter.ClinicID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid clinic_id"))
			return
		}
		clinicID = &id
	}

	retainedEarnings, err := h.svc.GetRetainedEarnings(c.Request.Context(), practitionerID.(uuid.UUID), clinicID, asOfDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, gin.H{
		"as_of_date":        asOfDate,
		"retained_earnings": retainedEarnings,
	}, "Retained earnings calculated successfully")
}

// GetEquityMovements godoc
// @Summary Get Equity Movements
// @Description Calculates detailed equity movements for current year (funds introduced, drawings, etc.)
// @Tags Owner Equity
// @Accept json
// @Produce json
// @Param clinic_id query string false "Filter by clinic UUID"
// @Param as_of_date query string false "Calculate as of date (YYYY-MM-DD), defaults to today"
// @Success 200 {object} EquityMovements
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /api/v1/equity/movements [get]
func (h *handler) GetEquityMovements(c *gin.Context) {
	practitionerID, exists := c.Get("practitioner_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, errors.New("practitioner_id not found in context"))
		return
	}

	var filter EquityFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Default to today if no date specified
	asOfDate := time.Now().Format("2006-01-02")
	if filter.AsOfDate != nil && *filter.AsOfDate != "" {
		asOfDate = *filter.AsOfDate
	}

	// Parse clinic ID if provided
	var clinicID *uuid.UUID
	if filter.ClinicID != nil && *filter.ClinicID != "" {
		id, err := uuid.Parse(*filter.ClinicID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, errors.New("invalid clinic_id"))
			return
		}
		clinicID = &id
	}

	movements, err := h.svc.CalculateCurrentYearEquityMovements(c.Request.Context(), practitionerID.(uuid.UUID), clinicID, asOfDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, movements, "Equity movements calculated successfully")
}
