package bs

import (
	"errors"
	"fmt"
	"log"
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

type Handler interface {
	GetBalanceSheet(c *gin.Context)
	ExportBalanceSheet(c *gin.Context)
}

type handler struct {
	svc           Service
	invitationSvc invitation.Service
}

func NewHandler(svc Service, invitationSvc invitation.Service) Handler {
	return &handler{svc: svc, invitationSvc: invitationSvc}
}

// GetBalanceSheet godoc
// @Summary      Get Balance Sheet
// @Description  Get balance sheet showing assets, liabilities, and equity (including owner fund accounts)
// @Tags         Balance Sheet
// @Accept       json
// @Produce      json
// @Param        start_date query string false "Start Date (YYYY-MM-DD)"
// @Param        end_date   query string false "End Date (YYYY-MM-DD)"
// @Param        user_id    query string false "Submitted-by User UUID"
// @Success      200 {object} RsBalanceSheet
// @Failure      400 {object} response.RsError "Bad Request"
// @Failure      401 {object} response.RsError "Unauthorized"
// @Failure      403 {object} response.RsError "Forbidden"
// @Failure      500 {object} response.RsError "Internal Server Error"
// @Security     BearerToken
// @Router       /balance-sheet [get]
func (h *handler) GetBalanceSheet(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, _ := util.GetUserID(c)
	if !ok {
		return
	}

	var filter BSFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.GetBalanceSheet(c.Request.Context(), &filter, *actorID, role, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Balance sheet fetched successfully")
}

// ExportBalanceSheet godoc
// @Summary      Export Balance Sheet to Excel/PDF
// @Description  Generates and downloads a Balance Sheet report. Accountants can export data for linked practitioners.
// @Tags         Balance Sheet
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, text/html
// @Param        practitioner_id  query  string   false  "Practitioner UUID (Required for Accountants to filter)"
// @Param        user_id          query  string   false  "Submitted-by User UUID"
// @Param        start_date       query  string   false  "Start Date (YYYY-MM-DD)"
// @Param        end_date         query  string   false  "End Date (YYYY-MM-DD)"
// @Param        financial_year_id query     string  false  "Anchor Financial Year snapshot UUID"
// @Param        comparisons       query     int     false  "Number of historical years to compare back (0 to 4)"
// @Param        export_type 	  query  string   true   "Export Type: pdf | excel"
// @Success      200              {file}   binary  "Balance_Sheet_2026-04-30.xlsx"
// @Failure      400              {object} response.RsError "Bad Request"
// @Failure      401              {object} response.RsError "Unauthorized"
// @Failure      403 {object} response.RsError "Forbidden"
// @Failure      500              {object} response.RsError "Internal Server Error"
// @Security     BearerToken
// @Router       /balance-sheet/export [get]
func (h *handler) ExportBalanceSheet(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f BSFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	baselineData, err := h.svc.GetBalanceSheet(c.Request.Context(), &f, *actorID, role, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	allYearsReportData := []*RsBalanceSheet{baselineData}

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

	// Fetch historical years for comparison if requested
	// numComparisons: 0 = current year only, 1 = current + 1 past year, etc.
	if numComparisons >= 1 && f.EndDate != nil && *f.EndDate != "" {
		baseTime, err := time.Parse("2006-01-02", *f.EndDate)
		if err == nil {
			for i := 1; i <= numComparisons; i++ {
				pastEndDate := baseTime.AddDate(-i, 0, 0).Format("2006-01-02")

				historicalFilter := BSFilter{
					PractitionerID: f.PractitionerID,
					UserID:         f.UserID,
					EndDate:        &pastEndDate,
				}

				pastYearData, err := h.svc.GetBalanceSheet(c.Request.Context(), &historicalFilter, *actorID, role, userID)
				if err == nil && pastYearData != nil {
					allYearsReportData = append(allYearsReportData, pastYearData)
				} else {
					log.Printf("[WARN] Failed to fetch historical data for year %d: %v", i, err)
				}
			}
		}
	}

	var notifIDs []uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		if f.PractitionerID != nil && *f.PractitionerID != "" {
			pID, _ := uuid.Parse(*f.PractitionerID)
			notifIDs = []uuid.UUID{pID}
		} else {
			linked, _ := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			notifIDs = linked
		}
	}

	pracIDStr := ""
	if f.PractitionerID != nil {
		pracIDStr = *f.PractitionerID
	}

	exportedFileResp, err := h.svc.ExportBalanceSheet(c.Request.Context(), allYearsReportData, exportType, *actorID, role, userID, notifIDs, pracIDStr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	switch v := exportedFileResp.Result.(type) {
	case *excelize.File:
		fileName := fmt.Sprintf("Balance_Sheet_%s.xlsx", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename="+fileName)
		if err := v.Write(c.Writer); err != nil {
			log.Printf("Error writing excel: %v", err)
		}

	case string:
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Header("Content-Disposition", "inline")
		c.String(http.StatusOK, v)

	default:
		response.Error(c, http.StatusInternalServerError, errors.New("unexpected export format"))
	}
}
