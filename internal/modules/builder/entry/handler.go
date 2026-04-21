package entry

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	List(c *gin.Context)
	ListTransactions(c *gin.Context)

	// COA-grouped endpoints
	ListCoaEntries(c *gin.Context)
	ListCoaEntryDetails(c *gin.Context)
	// GetFieldSummary(c *gin.Context)
	HandleExport(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// @Summary Create a new form entry
// @Description Create a new entry for a specific form version
// @Tags entry
// @Accept json
// @Produce json
// @Param version_id path string true "Version ID"
// @Param request body RqFormEntry true "Entry details"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/version/{version_id} [post]
func (h *handler) Create(c *gin.Context) {
	versionID, ok := util.ParseUuidID(c, "version_id")
	if !ok {
		return
	}

	role := c.GetString("role")
	var actorID uuid.UUID

	// Get ID based on who is logged in
	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	var req RqFormEntry
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	created, err := h.svc.Create(c.Request.Context(), versionID, &req, &actorID, actorID)
	if err != nil {
		if errors.Is(err, limits.ErrLimitReached) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created, "Form entry created successfully")
}

// @Summary Get a form entry by ID
// @Description Fetch details of a specific entry
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {object} response.RsBase
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/{id} [get]
func (h *handler) Get(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	e, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, e, "Form entry fetched successfully")
}

// @Summary Update a form entry
// @Description Update data for an existing entry
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Param request body RqUpdateFormEntry true "Updated details"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/{id} [patch]
func (h *handler) Update(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	role := c.GetString("role")
	var actorID uuid.UUID
	// Get ID based on who is logged in
	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}
	if !ok {
		return
	}

	var req RqUpdateFormEntry
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), id, &req, &actorID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Form entry updated successfully")
}

// @Summary Delete a form entry
// @Description Remove an entry from the system
// @Tags entry
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil, "Form entry deleted successfully")
}

// @Summary List form entries
// @Description List all entries for a specific version
// @Tags entry
// @Accept json
// @Produce json
// @Param version_id path string true "Version ID"
// @Param clinic_id query string false "Filter by clinic ID"
// @Param search query string false "Search keyword"
// @Param sort_by query string false "Sort field"
// @Param order_by query string false "Order direction (ASC/DESC)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/version/{version_id} [get]
func (h *handler) List(c *gin.Context) {
	versionID, ok := util.ParseUuidID(c, "version_id")
	if !ok {
		return
	}

	role := c.GetString("role")
	var actorID uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		id, ok := util.GetAccountantID(c)
		if !ok {
			return
		}
		actorID = id
	} else {
		id, ok := util.GetPractitionerID(c)
		if !ok {
			return
		}
		actorID = id
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	list, err := h.svc.List(c.Request.Context(), versionID, filter, actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Form entries fetched successfully")
}

// @Summary List all transactions
// @Description Returns flat rows (one per entry value) enriched with clinic, form, COA, and tax data
// @Tags entry
// @Produce json
// @Param clinic_id query string false "Filter by clinic ID"
// @Param form_id query string false "Filter by form ID"
// @Param coa_id query string false "Filter by COA ID"
// @Param tax_type_id query int false "Filter by account tax ID"
// @Param date_from query string false "Filter entries created after this date (RFC3339)"
// @Param date_to query string false "Filter entries created before this date (RFC3339)"
// @Param status query string false "Filter by status (DRAFT, SUBMITTED)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/transactions [get]
func (h *handler) ListTransactions(c *gin.Context) {
	role := c.GetString("role")
	var actorID uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		id, ok := util.GetAccountantID(c)
		if !ok {
			return
		}
		actorID = id
	} else {
		id, ok := util.GetPractitionerID(c)
		if !ok {
			return
		}
		actorID = id
	}

	var filter TransactionFilter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	// pracIDStr := practitionerID.String()
	// filter.PractitionerID = &pracIDStr

	list, err := h.svc.ListTransactions(c.Request.Context(), filter, actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Form entries fetched successfully")
}

// // @Summary Get summed values for a specific field
// // @Description Returns the total net, gst, and gross amounts for all active entries of a field
// // @Tags entry
// // @Produce json
// // @Param field_id path string true "Form Field ID"
// // @Success 200 {object} RsFieldSummary
// // @Failure 400 {object} response.RsError
// // @Failure 404 {object} response.RsError
// // @Failure 500 {object} response.RsError
// // @Security BearerToken
// // @Router /entry/{field_id}/summary [get]
// func (h *handler) GetFieldSummary(c *gin.Context) {
// 	fieldID, ok := util.ParseUuidID(c, "field_id")
// 	if !ok {
// 		return
// 	}

// 	summary, err := h.svc.GetFieldSummary(c.Request.Context(), fieldID)
// 	if err != nil {
// 		if errors.Is(err, ErrNotFound) {
// 			response.Error(c, http.StatusNotFound, err)
// 			return
// 		}
// 		response.Error(c, http.StatusInternalServerError, err)
// 		return
// 	}

// 	response.JSON(c, http.StatusOK, summary, "Field summary calculated successfully")
// }

// @Summary List grouped COA entries (parent grid)
// @Description Returns one row per COA with aggregated amounts and entry counts
// @Tags entry
// @Produce json
// @Param page query int false "Zero-based page index"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param practitioner_id query string false "Filter by practitioner ID"
// @Param clinic_id query string false "Filter by clinic ID"
// @Param form_id query string false "Filter by form ID"
// @Param coa_id query string false "Filter by COA ID"
// @Param tax_type_id query int false "Filter by tax type ID"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/coa-entries [get]
func (h *handler) ListCoaEntries(c *gin.Context) {
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	var filter TransactionFilter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	filter.Role = role

	result, err := h.svc.ListCoaEntries(c.Request.Context(), filter, *actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "COA entries fetched successfully")
}

// @Summary List entries for a specific COA (child grid)
// @Description Returns detailed entry rows for the expanded COA
// @Tags entry
// @Produce json
// @Param coa_id path string true "COA ID"
// @Param page query int false "Zero-based page index"
// @Param limit query int false "Page size (default 10, max 100)"
// @Param practitioner_id query string false "Filter by practitioner ID"
// @Param clinic_id query string false "Filter by clinic ID"
// @Param form_id query string false "Filter by form ID"
// @Param tax_type_id query int false "Filter by tax type ID"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /entry/coa-entries/{coa_id}/entries [get]
func (h *handler) ListCoaEntryDetails(c *gin.Context) {
	coaID := c.Param("coa_id")
	if coaID == "" {
		response.Error(c, http.StatusBadRequest, errors.New("coa_id is required"))
		return
	}

	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	var filter TransactionFilter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	filter.Role = role

	result, err := h.svc.ListCoaEntryDetails(c.Request.Context(), coaID, filter, *actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "COA entry details fetched successfully")
}

// @Summary Export transaction report to Excel
// @Description Generates an Excel file (.xlsx) containing grouped transaction records based on filters.
// @Tags reporting
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param clinic_id query string false "Filter by clinic ID"
// @Param form_id query string false "Filter by form ID"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Param search query string false "Search by account, field, or clinic name"
// @Success 200 {file} binary "Excel file containing transaction report"
// @Failure 400 {object} response.RsError "Invalid request parameters"
// @Failure 500 {object} response.RsError "Internal server error during generation"
// @Security BearerToken
// @Router /entry/coa-entries/export [get]
func (h *handler) HandleExport(c *gin.Context) {
	// 1. Get ActorID and Role using your established helper
	// This solves the 'user_id does not exist' panic
	actorID, role, ok := util.GetRoleBasedID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	// 2. Bind the filters (Use the same TransactionFilter struct as ListCoaEntries)
	var filter TransactionFilter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	filter.Role = role

	// 3. Call the service to generate the Excel buffer
	// Pass *actorID (dereferenced) and the role
	buffer, err := h.svc.ExportTransactionReport(c.Request.Context(), filter, *actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Errorf("failed to generate export: %w", err))
		return
	}

	// 4. Set Download Headers
	fileName := fmt.Sprintf("Transaction_Report_%s.xlsx", time.Now().Format("2006-01-02_1504"))

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Cache-Control", "no-cache")

	// 5. Write the binary data to the response
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}
