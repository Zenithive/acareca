package bas

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// IHandler declares all HTTP entry points for the BAS module.
type IHandler interface {
	GetQuarterlySummary(c *gin.Context)
	GetByAccount(c *gin.Context)
	GetMonthly(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// GetQuarterlySummary godoc
// @Summary      Quarterly BAS summary (ATO labels)
// @Description  Returns G1, G3, G8, 1A, G11, G14, G15, 1B and Net GST Payable per quarter for a clinic. Mirrors the Australian ATO BAS form labels. Only SUBMITTED entries are included. BAS Excluded accounts are omitted.
// @Tags         engine/bas
// @Produce      json
// @Param        clinic_id         path   string  true   "Clinic UUID"
// @Param        from_date         query  string  false  "Start date filter (YYYY-MM-DD) — rounded to quarter start"
// @Param        to_date           query  string  false  "End date filter (YYYY-MM-DD) — rounded to quarter end"
// @Param        financial_year_id query  string  false  "Restrict to a financial year by UUID"
// @Success      200  {array}   RsBASSummary
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/clinic/{clinic_id}/summary [get]
func (h *handler) GetQuarterlySummary(c *gin.Context) {
	clinicID, ok := parseClinicID(c)
	if !ok {
		return
	}

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetQuarterlySummary(c.Request.Context(), clinicID, &f)
	if err != nil {
		if errors.Is(err, ErrClinicNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS quarterly summary fetched successfully")
}

// GetByAccount godoc
// @Summary      BAS breakdown by COA account
// @Description  Returns quarterly GST totals broken down per Chart of Accounts entry and BAS category (TAXABLE / GST_FREE). Useful for reconciliation and identifying which accounts drive your 1A / 1B figures.
// @Tags         engine/bas
// @Produce      json
// @Param        clinic_id         path   string  true   "Clinic UUID"
// @Param        from_date         query  string  false  "Start date filter (YYYY-MM-DD)"
// @Param        to_date           query  string  false  "End date filter (YYYY-MM-DD)"
// @Param        financial_year_id query  string  false  "Restrict to a financial year by UUID"
// @Success      200  {array}   RsBASByAccount
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/clinic/{clinic_id}/by-account [get]
func (h *handler) GetByAccount(c *gin.Context) {
	clinicID, ok := parseClinicID(c)
	if !ok {
		return
	}

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetByAccount(c.Request.Context(), clinicID, &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS by account fetched successfully")
}

// GetMonthly godoc
// @Summary      Monthly BAS data
// @Description  Returns BAS figures grouped by calendar month. Useful for dashboards and tracking GST accrual within a quarter. Does not include G8 / G15 subtotals (use the quarterly summary for those).
// @Tags         engine/bas
// @Produce      json
// @Param        clinic_id  path   string  true   "Clinic UUID"
// @Param        from_date  query  string  false  "Start date filter (YYYY-MM-DD)"
// @Param        to_date    query  string  false  "End date filter (YYYY-MM-DD)"
// @Success      200  {array}   RsBASMonthly
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/clinic/{clinic_id}/monthly [get]
func (h *handler) GetMonthly(c *gin.Context) {
	clinicID, ok := parseClinicID(c)
	if !ok {
		return
	}

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetMonthly(c.Request.Context(), clinicID, &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS monthly data fetched successfully")
}

// ─── shared helpers ───────────────────────────────────────────────────────────

// parseClinicID validates JWT presence then parses the :clinic_id path param.
func parseClinicID(c *gin.Context) (uuid.UUID, bool) {
	if _, ok := util.GetPractitionerID(c); !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("clinic_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid clinic_id"))
		return uuid.Nil, false
	}
	return id, true
}
