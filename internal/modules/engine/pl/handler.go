package pl

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/xuri/excelize/v2"
)

// IHandler declares all HTTP entry points for the P&L module.
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
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	var f PLReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// MANDATORY: Clean the IDs before doing logic
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
			// Case A: Accountant specifically picked one
			if pID, err := uuid.Parse(f.PractitionerID); err == nil {
				targetNotifIDs = []uuid.UUID{pID}
			}
		} else if f.ClinicID == nil || *f.ClinicID == "" {
			// Case B: Aggregation mode - Fetch ALL linked practitioners
			linked, err := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			if err != nil || len(linked) == 0 {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("no linked practitioners found for aggregation"))
				fmt.Printf("\nError fetching linked practitioners:%v:", err)
				return
			}
			targetNotifIDs = linked
		}
	} else {
		// Practitioner: Only notify self
		targetNotifIDs = []uuid.UUID{*actorID}
		f.PractitionerID = actorID.String()
	}

	result, err := h.svc.GetReport(c.Request.Context(), *actorID, &f, role, targetNotifIDs)
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
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        clinic_id    query    string  false  "Clinic UUID"
// @Param        date_from    query    string  false  "Start date (YYYY-MM-DD)"
// @Param        date_until   query    string  false  "End date (YYYY-MM-DD)"
// @Param        coa_id       query    string  false  "Filter by COA UUID"
// @Param        tax_type_id  query    string  false  "Filter by tax type"
// @Param        form_id      query    string  false  "Filter by form UUID"
// @Param        export_type 	   query    string  true   "Export Type: PDF | Excel"
// @Success      200          {file}   binary  "Profit_and_Loss_Report.xlsx"
// @Failure      400          {object} response.RsError
// @Failure      500          {object} response.RsError
// @Security     BearerToken
// @Router       /pl/export [get]
func (h *handler) ExportReport(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		return
	}

	// Get the export type from query params (default to excel)
	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f PLReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Resolve PracIDs (for data scoping) and notifIDs (for Shared Events).
	// Scenario A: practitioner_id in query → scope + notify only that one.
	// Scenario B: no practitioner_id → fetch all linked, notify all.
	// Practitioner: scope to self, no shared events.
	var PracIDs []uuid.UUID
	var notifIDs []uuid.UUID

	if strings.EqualFold(role, util.RoleAccountant) {
		if pracIDStr := c.Query("practitioner_id"); pracIDStr != "" {
			// Scenario A
			pracUUID, err := uuid.Parse(pracIDStr)
			if err != nil {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid practitioner_id: must be a valid UUID"))
				return
			}
			f.PractitionerID = pracIDStr
			PracIDs = []uuid.UUID{pracUUID}
			notifIDs = []uuid.UUID{pracUUID}
		} else {
			// Scenario B
			linked, err := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to fetch linked practitioners: %w", err))
				return
			}
			if len(linked) == 0 {
				response.Error(c, http.StatusForbidden, fmt.Errorf("accountant is not linked to any practitioners"))
				return
			}
			PracIDs = linked
			notifIDs = linked
			// f.PractitionerID left empty — service resolves via clinicRepo
		}
	} else {
		PracIDs = []uuid.UUID{*actorID}
		notifIDs = nil // practitioners never receive their own shared events
	}

	// Safely handle optional ClinicID
	clinicIDParam := ""
	if f.ClinicID != nil {
		clinicIDParam = *f.ClinicID
	}

	// Fetch the structured data (service resolves and sets f.PractitionerID internally)
	reportData, err := h.svc.GetReport(c.Request.Context(), userID, &f, role, notifIDs)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	// Generate the Excel/PDF file
	excelFile, err := h.svc.ExportPLReport(c.Request.Context(), reportData, exportType, *actorID, role, userID, notifIDs, clinicIDParam)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	_ = PracIDs // used for scoping context; notifIDs drives notifications

	switch v := excelFile.(type) {
	case *excelize.File:
		fileName := fmt.Sprintf("Profit_and_Loss_%s.xlsx", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		v.Write(c.Writer)

	case string:
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", "inline")
		c.String(http.StatusOK, v)

	default:
		response.Error(c, http.StatusInternalServerError, errors.New("unexpected export format"))
	}
}

// cleanUUIDString handles the case where the frontend sends UUIDs
// wrapped in JSON-style brackets and quotes like ["uuid"]
func cleanUUIDString(s string) string {
	if s == "" {
		return ""
	}
	// Remove brackets, double quotes, and whitespace
	res := strings.NewReplacer("[", "", "]", "", "\"", "", " ", "").Replace(s)

	// If multiple IDs were sent in a comma-separated string, take the first one
	if strings.Contains(res, ",") {
		res = strings.Split(res, ",")[0]
	}
	return res
}
