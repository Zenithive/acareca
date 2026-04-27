package bs

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type Handler interface {
	GetBalanceSheet(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) Handler {
	return &handler{svc: svc}
}

// GetBalanceSheet godoc
// @Summary Get Balance Sheet
// @Description Get balance sheet showing assets, liabilities, and equity (including owner fund accounts)
// @Tags Balance Sheet
// @Accept json
// @Produce json
// @Param clinic_id query string false "Filter by clinic UUID"
// @Param as_of_date query string false "Balance as of date (YYYY-MM-DD), defaults to today"
// @Success 200 {object} RsBalanceSheet
// @Failure 400 {object} response.RsError
// @Failure 401 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Router /api/v1/balance-sheet [get]
func (h *handler) GetBalanceSheet(c *gin.Context) {
	practitionerID, exists := c.Get("practitioner_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, errors.New("practitioner_id not found in context"))
		return
	}

	var filter BSFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetBalanceSheet(c.Request.Context(), practitionerID.(uuid.UUID), &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Balance sheet fetched successfully")
}
