package bas

import (
	"bytes"
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
	var ok bool

	userID, okUser := util.GetUserID(c)
	if !okUser {
		return
	}

	// Parse the query filters
	var f BASReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	var PracIDs []uuid.UUID
	if role == util.RoleAccountant {
		actorID, ok = util.GetAccountantID(c)
		if !ok {
			return
		}

		if pracIDStr := c.Query("practitioner_id"); pracIDStr != "" {
			// Scenario A: specific practitioner_id provided
			pracUUID, err := uuid.Parse(pracIDStr)
			if err != nil {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid practitioner_id: must be a valid UUID"))
				return
			}
			f.PractitionerID = pracIDStr
			PracIDs = []uuid.UUID{pracUUID}
		} else {
			// Scenario B: fetch all linked practitioners
			linkedIDs, err := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), actorID)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to fetch linked practitioners: %w", err))
				return
			}
			if len(linkedIDs) == 0 {
				response.Error(c, http.StatusForbidden, fmt.Errorf("accountant is not linked to any practitioners"))
				return
			}
			PracIDs = linkedIDs
			f.PractitionerID = linkedIDs[0].String()
		}
	} else {
		actorID, ok = util.GetPractitionerID(c)
		if !ok {
			return
		}
		PracIDs = []uuid.UUID{actorID}
		f.PractitionerID = actorID.String()
	}

	result, err := h.svc.GetReport(c.Request.Context(), &f, PracIDs, userID, actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS report fetched successfully")
}

// GetBASPreparation godoc
// @Summary      Full BAS Preparation Report
// @Description  Returns a side-by-side comparison of BAS figures across selected quarters/months, plus a calculated Grand Total column. Aggregates data across all practitioner's data.
// @Tags         engine/bas
// @Produce      json
// @Param        quarter_ids       query  string true "Comma-separated Quarter UUIDs (e.g. uuid1,uuid2)"
// @Param        financial_year_id query  string  true "Restrict to a financial year by UUID"
// @Success      200  {object}  RsBASPreparation
// @Failure      400  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /bas/bas-preparation [get]
func (h *handler) GetBASPreparation(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		return
	}

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	_ = f.MapToFilter()

	result, err := h.svc.GetBASPreparation(c.Request.Context(), *actorID, role, &f, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "BAS preparation data fetched")
}

// ExportBASReport godoc
// @Summary Export Business Activity Statement to Excel
// @Description Generates a formatted Excel BAS report.
// @Tags engine/bas
// @Security BearerToken
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, text/html
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
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// Resolve practitionerIDs (data scope) and notifIDs (Shared Events).
	// Scenario A: practitioner_id in query → scope + notify only that one.
	// Scenario B: no practitioner_id → all linked practitioners.
	// Practitioner: self only, no shared events.
	var practitionerIDs []uuid.UUID
	var notifIDs []uuid.UUID

	if role == util.RoleAccountant {
		if pracIDStr := c.Query("practitioner_id"); pracIDStr != "" {
			// Scenario A
			pracUUID, err := uuid.Parse(pracIDStr)
			if err != nil {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid practitioner_id: must be a valid UUID"))
				return
			}
			practitionerIDs = []uuid.UUID{pracUUID}
			notifIDs = []uuid.UUID{pracUUID}
		} else {
			// Scenario B
			linkedIDs, err := h.invitationSvc.GetPractitionersLinkedToAccountant(ctx, *actorID)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to fetch linked practitioners: %w", err))
				return
			}
			if len(linkedIDs) == 0 {
				response.Error(c, http.StatusForbidden, fmt.Errorf("accountant is not linked to any practitioners"))
				return
			}
			practitionerIDs = linkedIDs
			notifIDs = linkedIDs
		}
	} else {
		practitionerIDs = []uuid.UUID{*actorID}
		notifIDs = nil // practitioners never receive their own shared events
	}

	// Get Export Type from query
	exportType := c.DefaultQuery("export_type", "excel")

	var f BASExportFilter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// Ensure either Quarter or Month is provided
	if len(f.QuarterIDs) == 0 && (f.Month == nil || *f.Month == "") {
		response.Error(c, http.StatusBadRequest, errors.New("either quarter_id or month must be provided"))
		return
	}

	// For accountants: use the first linked practitioner ID.
	// For practitioners: use their own ID.
	// This ensures f.PractitionerID is always a valid practitioner UUID.
	f.PractitionerID = practitionerIDs[0].String()

	// Fetch ALL 4 quarters for the selected Financial Year
	fyID, _ := uuid.Parse(*f.FinancialYearID)
	allQuarters, err := h.svc.GetAllQuartersInYear(ctx, fyID)
	if err != nil || len(allQuarters) == 0 {
		response.Error(c, http.StatusNotFound, errors.New("financial year quarters not found"))
		return
	}

	// REORDER: Move the requested QuarterID to the front
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

	// Populate Data Loop
	for i, qInfo := range allQuarters {
		tempID := qInfo.ID
		origFilter := BASReportFilter{
			PractitionerID: f.PractitionerID,
			QuarterID:      &tempID,
			Month:          f.Month,
		}

		report, _ := h.svc.GetReport(ctx, &origFilter, nil, userID, *actorID, role)
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

	// Call Service with exportType
	result, contentType, err := h.svc.ExportActivityStatement(ctx, allQuartersData, basePrevDates, exportType, *actorID, role, userID, notifIDs)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to generate export: %w", err))
		return
	}

	if contentType == "text/html" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", "inline")
		c.String(http.StatusOK, result.(string))
		return
	}

	//  Default Excel handling
	buf := result.(*bytes.Buffer)
	fileName := fmt.Sprintf("BAS_Statement_%s.xlsx", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, contentType, buf.Bytes())
}

// ExportBASPreparation godoc
// @Summary      Export Quarterly BAS Preparation
// @Description  Generates an Excel file matching the shared template using GetBASPreparation data.
// @Tags         engine/bas
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, text/html
// @Param        quarter_ids       query    string  true   "Quarter UUIDs"
// @Param        financial_year_id query    string  true   "FY UUID"
// @Param        export_type 	   query    string  true   "Export Type: PDF | Excel"
// @Success      200 {file} binary
// @Router       /bas/bas-preparation/export [get]
// @Security     BearerToken
func (h *handler) ExportBASPreparation(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	UserID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// Resolve PracIDs (data scope) and notifIDs (Shared Events).
	// var PracIDs []uuid.UUID
	var notifIDs []uuid.UUID

	if role == util.RoleAccountant {
		if pracIDStr := c.Query("practitioner_id"); pracIDStr != "" {
			// Scenario A
			pracUUID, err := uuid.Parse(pracIDStr)
			if err != nil {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid practitioner_id: must be a valid UUID"))
				return
			}
			// PracIDs = []uuid.UUID{pracUUID}
			notifIDs = []uuid.UUID{pracUUID}
		} else {
			// Scenario B
			linkedIDs, err := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to fetch linked practitioners: %w", err))
				return
			}
			if len(linkedIDs) == 0 {
				response.Error(c, http.StatusForbidden, fmt.Errorf("accountant is not linked to any practitioners"))
				return
			}
			// PracIDs = linkedIDs
			notifIDs = linkedIDs
		}
	} else {
		// PracIDs = []uuid.UUID{*actorID}
		notifIDs = nil // practitioners never receive their own shared events
	}

	// Get the export type from query params (default to excel)
	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f BASFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	_ = f.MapToFilter()

	data, err := h.svc.GetBASPreparation(c.Request.Context(), *actorID, role, &f, UserID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	file, err := h.svc.ExportBASPreparation(c.Request.Context(), data, *actorID, role, UserID, &f, exportType, notifIDs)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	switch v := file.(type) {
	case *excelize.File:
		// Standard Excel Response
		fileName := fmt.Sprintf("Quarterly_BAS_Preparation_%s.xlsx", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		v.Write(c.Writer)

	case string:
		// HTML Response for PDF
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", "inline") // opens in new tab
		c.String(http.StatusOK, v)

	default:
		response.Error(c, http.StatusInternalServerError, errors.New("unexpected export format"))
	}
}
