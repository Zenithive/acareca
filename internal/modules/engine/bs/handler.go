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
// @Security     BearerToken
// @Router /balance-sheet [get]
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
// @Param        practitioner_id  query    string  false  "Practitioner UUID (Required for Accountants to filter)"
// @Param        clinic_id        query    string  false  "Clinic UUID"
// @Param        as_of_date       query    string  false  "Balance as of date (YYYY-MM-DD), defaults to today"
// @Param        export_type 	  query    string  true   "Export Type: pdf | excel"
// @Success      200              {file}   binary  "Balance_Sheet_2026-04-30.xlsx"
// @Failure      400              {object} response.RsError
// @Failure      401              {object} response.RsError
// @Failure      500              {object} response.RsError
// @Security     BearerToken
// @Router       /balance-sheet/export [get]
func (h *handler) ExportBalanceSheet(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	userID, okUser := util.GetUserID(c)
	if !ok || !okUser {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// Get the export type from query params (default to excel)
	exportType := strings.ToLower(c.DefaultQuery("export_type", "excel"))

	var f BSFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// 1. Fetch report data
	reportData, err := h.svc.GetBalanceSheet(c.Request.Context(), &f, *actorID, role, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	// Logic for notifications (notifIDs)
	var notifIDs []uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		if f.PractitionerID != nil && *f.PractitionerID != "" {
			pID, _ := uuid.Parse(*f.PractitionerID)
			notifIDs = []uuid.UUID{pID}
		} else {
			// Service already did this work, but for notifications we might need the list again
			linked, _ := h.invitationSvc.GetPractitionersLinkedToAccountant(c.Request.Context(), *actorID)
			notifIDs = linked
		}
	}

	// 2. Generate Export (Excel or PDF HTML)
	clinicIDParam := ""
	if f.ClinicID != nil {
		clinicIDParam = *f.ClinicID
	}

	exportedFile, err := h.svc.ExportBalanceSheet(c.Request.Context(), reportData, exportType, *actorID, role, userID, notifIDs, clinicIDParam)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	// 3. Response handling
	switch v := exportedFile.(type) {
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
