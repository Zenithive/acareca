package pl

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// IHandler declares all HTTP entry points for the P&L module.
type IHandler interface {
	GetMonthlySummary(c *gin.Context)
	GetByAccount(c *gin.Context)
	GetByResponsibility(c *gin.Context)
	GetFYSummary(c *gin.Context)
	GetReport(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// GetMonthlySummary godoc
// @Summary      Monthly P&L summary
// @Description  Returns Income, COGS, Gross Profit, Other Expenses and Net Profit grouped by calendar month, filtered by clinic_id.
// @Tags         engine/pl
// @Produce      json
// @Param        clinic_id  query  string  true   "Clinic UUID"
// @Param        from_date  query  string  false  "Start date filter (YYYY-MM-DD)"
// @Param        to_date    query  string  false  "End date filter (YYYY-MM-DD)"
// @Success      200  {array}   RsPLSummary
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /pl/summary [get]
func (h *handler) GetMonthlySummary(c *gin.Context) {
	if _, ok := util.GetPractitionerID(c); !ok {
		return
	}

	var f PLFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetMonthlySummary(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "P&L monthly summary fetched successfully")
}

// GetByAccount godoc
// @Summary      P&L by COA account
// @Description  Returns monthly totals broken down per Chart of Accounts entry, filtered by clinic_id.
// @Tags         engine/pl
// @Produce      json
// @Param        clinic_id  query  string  true   "Clinic UUID"
// @Param        from_date  query  string  false  "Start date filter (YYYY-MM-DD)"
// @Param        to_date    query  string  false  "End date filter (YYYY-MM-DD)"
// @Success      200  {array}   RsPLAccount
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /pl/by-account [get]
func (h *handler) GetByAccount(c *gin.Context) {
	if _, ok := util.GetPractitionerID(c); !ok {
		return
	}

	var f PLFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetByAccount(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "P&L by account fetched successfully")
}

// GetByResponsibility godoc
// @Summary      P&L split by payment responsibility
// @Description  Returns monthly totals split by OWNER vs CLINIC, filtered by clinic_id.
// @Tags         engine/pl
// @Produce      json
// @Param        clinic_id  query  string  true   "Clinic UUID"
// @Param        from_date  query  string  false  "Start date filter (YYYY-MM-DD)"
// @Param        to_date    query  string  false  "End date filter (YYYY-MM-DD)"
// @Success      200  {array}   RsPLResponsibility
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /pl/by-responsibility [get]
func (h *handler) GetByResponsibility(c *gin.Context) {
	if _, ok := util.GetPractitionerID(c); !ok {
		return
	}

	var f PLFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetByResponsibility(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "P&L by responsibility fetched successfully")
}

// GetFYSummary godoc
// @Summary      Quarterly P&L by financial year
// @Description  Returns P&L summarised by financial year and quarter (Q1–Q4), filtered by clinic_id.
// @Tags         engine/pl
// @Produce      json
// @Param        clinic_id          query  string  true   "Clinic UUID"
// @Param        financial_year_id  query  string  false  "Filter to a single financial year (UUID)"
// @Success      200  {array}   RsPLFYSummary
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /pl/fy-summary [get]
func (h *handler) GetFYSummary(c *gin.Context) {
	if _, ok := util.GetPractitionerID(c); !ok {
		return
	}

	var f PLFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetFYSummary(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "P&L FY summary fetched successfully")
}

// GetReport godoc
// @Summary      Structured P&L report
// @Description  Returns a nested P&L report grouped by clinic → form → section → field, filtered by date range, COA, tax type, and form.
// @Tags         engine/pl
// @Produce      json
// @Param        clinic_id   query  string  false  "Clinic UUID (omit for all clinics)"
// @Param        date_from   query  string  false  "Start date (YYYY-MM-DD)"
// @Param        date_until  query  string  false  "End date (YYYY-MM-DD)"
// @Param        coa_id      query  string  false  "Filter by COA UUID"
// @Param        tax_type_id query  string  false  "Filter by tax type name (e.g. GST on Income)"
// @Param        form_id     query  string  false  "Filter by form UUID"
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /pl/report [get]
func (h *handler) GetReport(c *gin.Context) {
	actorID, ok := util.GetUserID(c)
	if !ok {
		return
	}

	var f PLReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	//f.PractitionerID = pracID.String()

	result, err := h.svc.GetReport(c.Request.Context(), actorID, &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Profit and Loss report fetched successfully")
}
