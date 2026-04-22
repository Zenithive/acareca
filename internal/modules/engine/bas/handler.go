package bas

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/xuri/excelize/v2"
)

// IHandler declares all HTTP entry points for the BAS module.
type IHandler interface {
	GetQuarterlySummary(c *gin.Context)
	GetByAccount(c *gin.Context)
	GetMonthly(c *gin.Context)
	GetReport(c *gin.Context)
	GetBASPreparation(c *gin.Context)
	ExportBASReport(c *gin.Context)
	ExportBASPreparation(c *gin.Context)
}

type handler struct {
	svc           Service
	invitationSvc invitation.Service
}

func NewHandler(svc Service, invitationSvc invitation.Service) IHandler {
	return &handler{svc: svc, invitationSvc: invitationSvc}
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

// GetReport godoc
// @Summary      BAS totals report
// @Description  Returns G1, 1A, G11, 1B totals scoped to the authenticated practitioner, filtered by quarter_id or month name.
// @Tags         engine/bas
// @Produce      json
// @Param        quarter_id  query  string  false  "Financial quarter UUID"
// @Param        month       query  string  false  "Month name e.g. January"
// @Success      200  {object}  RsBASReport
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/report [get]
func (h *handler) GetReport(c *gin.Context) {
	role := c.GetString("role")
	var actorID uuid.UUID
	var pracID uuid.UUID
	var ok bool

	if role == util.RoleAccountant {
		actorID, ok = util.GetAccountantID(c)
		if !ok {
			return
		}

		// Resolve which Practitioner this Accountant is working for
		resolvedID, err := h.invitationSvc.GetFirstPractitionerLinkedToAccountant(c.Request.Context(), actorID)
		if err != nil {
			response.Error(c, http.StatusForbidden, fmt.Errorf("accountant not linked to a practitioner: %w", err))
			return
		}
		pracID = resolvedID
	} else {
		// If they are a Practitioner, the actorID IS the pracID
		actorID, ok = util.GetPractitionerID(c)
		if !ok {
			return
		}
		pracID = actorID
	}

	var f BASReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	f.PractitionerID = pracID.String()

	result, err := h.svc.GetReport(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS report fetched successfully")
}

// GetBASPreparation godoc
// @Summary      Full BAS Preparation Report
// @Description  Returns a side-by-side comparison of BAS figures across selected quarters/months, plus a calculated Grand Total column. If clinicId is not provided in query params, aggregates data across all clinics. Multiple clinicId values can be provided to aggregate specific clinics.
// @Tags         engine/bas
// @Produce      json
// @Param   clinic_ids         query    string  false  "Comma-separated Clinic UUIDs"
// @Param        quarter_ids       query  string true "Comma-separated Quarter UUIDs (e.g. uuid1,uuid2)"
// @Param        financial_year_id query  string  true "Restrict to a financial year by UUID"
// @Success      200  {object}  RsBASPreparation
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/bas-preparation [get]
func (h *handler) GetBASPreparation(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		return
	}

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	_ = f.MapToFilter()

	result, err := h.svc.GetBASPreparation(c.Request.Context(), *actorID, role, &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS preparation data fetched")
}

// ExportBASReport godoc
// @Summary Export Business Activity Statement to Excel
// @Description Generates a formatted Excel BAS report.
// @Tags BAS
// @Security BearerToken
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param export_type query string false "Export format: 'pdf' or 'excel' (default: excel)" Enums(pdf, excel)
// @Param financial_year_id query string true "Financial Year UUID"
// @Param quarter_id query []string false "Quarter UUIDs (can pass multiple)" collectionFormat(multi)
// @Param month query string false "Full month name"
// @Success 200 {file} binary
// @Failure 401 {object} map[string]string "Unauthorized"
// @Router /bas/activity-statement/report/export [get]
func (h *handler) ExportBASReport(c *gin.Context) {
	ctx := c.Request.Context()

	actorID, role, ok := util.GetRoleBasedID(c)
	userID, _ := util.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// 1. Get Export Type from query
	exportType := c.DefaultQuery("export_type", "excel")

	var f BASExportFilter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	// NEW VALIDATION: Ensure either Quarter or Month is provided
	if len(f.QuarterIDs) == 0 && (f.Month == nil || *f.Month == "") {
		response.Error(c, http.StatusBadRequest, errors.New("either quarter_id or month must be provided"))
		return
	}

	f.PractitionerID = actorID.String()

	// 2. Fetch ALL 4 quarters for the selected Financial Year
	fyID, _ := uuid.Parse(*f.FinancialYearID)
	allQuarters, err := h.svc.GetAllQuartersInYear(ctx, fyID)
	if err != nil || len(allQuarters) == 0 {
		response.Error(c, http.StatusNotFound, errors.New("financial year quarters not found"))
		return
	}

	// 3. REORDER: Move the requested QuarterID to the front
	if len(f.QuarterIDs) > 0 {
		targetID := f.QuarterIDs[0]
		for i, q := range allQuarters {
			if q.ID == targetID {
				allQuarters[0], allQuarters[i] = allQuarters[i], allQuarters[0]
				break
			}
		}
	}

	var allQuartersData []QuarterData
	var basePrevDates PeriodInfo

	// 4. Populate Data Loop
	for i, qInfo := range allQuarters {
		tempID := qInfo.ID
		origFilter := BASReportFilter{
			PractitionerID: f.PractitionerID,
			QuarterID:      &tempID,
			Month:          f.Month,
		}

		report, _ := h.svc.GetReport(ctx, &origFilter)
		if report == nil {
			report = &RsBASReport{}
		}

		currD, prevD, err := h.svc.GetPeriodDates(ctx, &origFilter)
		if err != nil {
			continue
		}

		allQuartersData = append(allQuartersData, QuarterData{
			Period: currD,
			Report: report,
		})

		if i == 0 {
			basePrevDates = prevD
		}
	}

	// 5. Call Service with exportType (Expect 3 return values)
	buffer, contentType, err := h.svc.ExportActivityStatement(ctx, allQuartersData, basePrevDates, exportType, *actorID, role, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to generate export: %w", err))
		return
	}

	// 6. Set Dynamic Filename and Headers
	ext := ".xlsx"
	if strings.ToLower(exportType) == "pdf" {
		ext = ".pdf"
	}
	fileName := fmt.Sprintf("BAS_Statement_%s%s", time.Now().Format("2006-01-02"), ext)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", contentType)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Cache-Control", "no-cache")

	// 7. Write Data to Response
	c.Data(http.StatusOK, contentType, buffer.Bytes())
}

// ExportBASPreparation godoc
// @Summary      Export Quarterly BAS Preparation
// @Description  Generates an Excel file matching the shared template using GetBASPreparation data.
// @Tags         engine/bas
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        clinic_ids        query    string  false  "Clinic UUIDs"
// @Param        quarter_ids       query    string  true   "Quarter UUIDs"
// @Param        financial_year_id query    string  true   "FY UUID"
// @Param        export_type 	   query    string  true   "Export Type: PDF | Excel"
// @Success      200 {file} binary
// @Router       /bas/bas-preparation/export [get]
// @Security     BearerToken
func (h *handler) ExportBASPreparation(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	UserID, ok := util.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// Get the export type from query params (default to excel)
	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	_ = f.MapToFilter()

	// Get the exact data structure you shared
	data, err := h.svc.GetBASPreparation(c.Request.Context(), *actorID, role, &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	file, err := h.svc.ExportBASPreparation(c.Request.Context(), data, *actorID, role, UserID, &f, exportType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	// fileName := fmt.Sprintf("Quarterly_BAS_Preparation_%s.xlsx", time.Now().Format("2006-01-02"))
	// c.Header("Content-Disposition", "attachment; filename="+fileName)
	// c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	// file.Write(c.Writer)

	switch v := file.(type) {
	case *excelize.File:
		// Standard Excel Response
		fileName := fmt.Sprintf("Quarterly_BAS_Preparation_%s.xlsx", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		v.Write(c.Writer)
	case []byte:
		// PDF Response
		fileName := fmt.Sprintf("Quarterly_BAS_Preparation_%s.pdf", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		c.Writer.Write(v)

	default:
		response.Error(c, http.StatusInternalServerError, errors.New("unexpected export format"))
	}
}
