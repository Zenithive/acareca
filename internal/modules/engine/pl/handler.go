package pl

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	GetMonthlySummary(c *gin.Context)
	GetByAccount(c *gin.Context)
	GetByResponsibility(c *gin.Context)
	GetFYSummary(c *gin.Context)
	GetReport(c *gin.Context)
	ExportReport(c *gin.Context)
}

type handler struct {
	svc            Service
	invitationSvc  invitation.Service
	accountantRepo accountant.Repository
}

func NewHandler(svc Service, invitationSvc invitation.Service, accountantRepo accountant.Repository) IHandler {
	return &handler{svc: svc, invitationSvc: invitationSvc, accountantRepo: accountantRepo}
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
// @Failure      400  {object}  response.RsError "Bad Request"
// @Failure      401  {object}  response.RsError "Unauthorized"
// @Failure      500  {object}  response.RsError "Internal Server Error"
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
// @Failure      400  {object}  response.RsError "Bad Request"
// @Failure      401  {object}  response.RsError "Unauthorized"
// @Failure      500  {object}  response.RsError "Internal Server Error"
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
// @Failure      400  {object}  response.RsError "Bad Request"
// @Failure      401  {object}  response.RsError "Unauthorized"
// @Failure      500  {object}  response.RsError "Internal Server Error"
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
// @Failure      400  {object}  response.RsError "Bad Request"
// @Failure      401  {object}  response.RsError "Unauthorized"
// @Failure      500  {object}  response.RsError "Internal Server Error"
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
// @Failure      400  {object}  response.RsError "Bad Request"
// @Failure      401  {object}  response.RsError "Unauthorized"
// @Failure      403  {object}  response.RsError "Forbidden"
// @Failure      500  {object}  response.RsError "Internal Server Error"
// @Security     BearerToken
// @Router       /pl/report [get]
func (h *handler) GetReport(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		return
	}
	var f PLReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if f.PractitionerID != "" {
		f.PractitionerID = cleanUUIDString(f.PractitionerID)
	}
	if f.ClinicID != nil {
		cleaned := cleanUUIDString(*f.ClinicID)
		f.ClinicID = &cleaned
	}

	var targetNotifIDs []uuid.UUID

	if strings.EqualFold(role, util.RoleAccountant) {
		if f.PractitionerID != "" {
			if pID, err := uuid.Parse(f.PractitionerID); err == nil {
				targetNotifIDs = []uuid.UUID{pID}
			}
		} else if f.ClinicID == nil || *f.ClinicID == "" {
			linked, err := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			if err != nil || len(linked) == 0 {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("no linked practitioners found for aggregation"))
				return
			}
			targetNotifIDs = linked
		}
	} else {
		targetNotifIDs = []uuid.UUID{*actorID}
		f.PractitionerID = actorID.String()
	}

	result, err := h.svc.GetReport(c.Request.Context(), *actorID, &f, role, targetNotifIDs, userID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Profit and Loss report fetched successfully")
}

// ExportReport godoc
// @Summary      Export P&L report to Excel
// @Description  Generates and downloads a professional Excel file of the P&L report using the specified filters.
// @Tags         engine/pl
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, text/html
// @Param        clinic_id    query    string  false  "Clinic UUID"
// @Param        date_from    query    string  false  "Start date (YYYY-MM-DD)"
// @Param        date_until   query    string  false  "End date (YYYY-MM-DD)"
// @Param        coa_id       query    string  false  "Filter by COA UUID"
// @Param        tax_type_id  query    string  false  "Filter by tax type"
// @Param        form_id      query    string  false  "Filter by form UUID"
// @Param        comparisons     query    int     false  "Number of comparative periods to include (0-4)"
// @Param        export_type 	   query    string  true   "Export Type: PDF | Excel"
// @Success      200          {file}   binary  "Profit_and_Loss_Report.xlsx"
// @Failure      400          {object} response.RsError "Bad Request"
// @Failure      401          {object} response.RsError "Unauthorized"
// @Failure      403          {object} response.RsError "Forbidden"
// @Failure      500          {object} response.RsError "Internal Server Error"
// @Security     BearerToken
// @Router       /pl/export [get]
func (h *handler) ExportReport(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		return
	}

	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f PLReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	var notifIDs []uuid.UUID
	pracIDParam := c.Query("practitioner_id")

	if strings.EqualFold(role, util.RoleAccountant) {
		if pracIDParam != "" {
			pracUUID := uuid.MustParse(pracIDParam)
			notifIDs = []uuid.UUID{pracUUID}
			f.PractitionerID = pracIDParam
		} else {
			linked, _ := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			notifIDs = linked
		}
	}

	numComparisons := 0
	if f.Comparisons != nil {
		numComparisons = *f.Comparisons
		if numComparisons > 4 {
			numComparisons = 4
		}
		if numComparisons < 0 {
			numComparisons = 0
		}
	}

	baselineReport, err := h.svc.GetReport(c.Request.Context(), *actorID, &f, role, notifIDs, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	allYearsPLData := []*RsReport{baselineReport}

	if numComparisons > 1 && f.DateFrom != nil && *f.DateFrom != "" && f.DateUntil != nil && *f.DateUntil != "" {
		startBase, errStart := time.Parse("2006-01-02", *f.DateFrom)
		endBase, errEnd := time.Parse("2006-01-02", *f.DateUntil)

		if errStart == nil && errEnd == nil {
			for i := 1; i <= numComparisons-1; i++ {
				pastFrom := startBase.AddDate(-i, 0, 0).Format("2006-01-02")
				pastUntil := endBase.AddDate(-i, 0, 0).Format("2006-01-02")

				historicalFilter := f
				historicalFilter.DateFrom = &pastFrom
				historicalFilter.DateUntil = &pastUntil

				pastReport, err := h.svc.GetReport(c.Request.Context(), *actorID, &historicalFilter, role, notifIDs, userID)
				if err == nil && pastReport != nil {
					allYearsPLData = append(allYearsPLData, pastReport)
				}
			}
		}
	}

	excelFile, err := h.svc.ExportPLReport(c.Request.Context(), allYearsPLData, exportType, *actorID, role, userID, notifIDs, pracIDParam)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	fileName := fmt.Sprintf("Profit_and_Loss_%s.xlsx", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	if err := excelFile.Write(c.Writer); err != nil {
		log.Printf("Error writing excel: %v", err)
	}
}

func cleanUUIDString(s string) string {
	if s == "" {
		return ""
	}
	res := strings.NewReplacer("[", "", "]", "", "\"", "", " ", "").Replace(s)
	if strings.Contains(res, ",") {
		res = strings.Split(res, ",")[0]
	}
	return res
}
