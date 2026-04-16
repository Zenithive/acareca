package calculation

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	LiveCalculate(c *gin.Context)

	// Legacy Code
	Calculation(c *gin.Context)
	CalculateFromEntries(c *gin.Context)
	FormulaCalculate(c *gin.Context)

	GetFormSummary(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{
		svc: svc,
	}
}

// LiveCalculate godoc
// @Summary Live calculation based on form version ID
// @Description Evaluates all is_computed=true fields for a form version using provided field entries.
// @Description Returns net amount for each computed field; net/gst/gross when the field has a tax_type.
// @Description Pass form_field_id with net_amount, gst_amount, and gross_amount for each field.
// @Tags calculation
// @Accept json
// @Produce json
// @Param request body RqLiveCalculate true "Form version ID and field entries"
// @Success 200 {object} RsLiveCalculate
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculate/live [post]
func (h *handler) LiveCalculate(c *gin.Context) {
	var req RqLiveCalculate
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.LiveCalculate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Live calculation completed successfully")
}

//Legacy code

// Calculation godoc
// @Summary Run calculation for a form
// @Description Calculate results for a specific form by ID
// @Tags calculation
// @Produce json
// @Param id path string true "Form ID"
// @Param super_component query number false "Super component value override"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculate/{id} [get]
// Calculation implements [IHandler].
func (h *handler) Calculation(c *gin.Context) {
	ctx := c.Request.Context()
	var actorID, formID uuid.UUID
	var ok bool

	formID, ok = util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	// Get Role and appropriate ID
	role := c.GetString("role")

	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}

	var filter NetFilter

	if superComponent := c.Query("super_component"); superComponent != "" {
		val, err := strconv.ParseFloat(superComponent, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("super_component must be a number"))
			return
		}
		if val < 0 || val > 100 {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("super_component must be between 0 and 100 (e.g. 11.5 for 11.5%%)"))
			return
		}
		filter.SuperComponent = &val
	}

	result, err := h.svc.Calculate(ctx, formID, &filter, actorID, role)
	if err != nil {
		if errors.Is(err, entry.ErrNotFound) || errors.Is(err, version.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Calculation completed successfully")
}

// CalculateFromEntries godoc
// @Summary Calculate from supplied entries
// @Description Run GrossMethod or NetMethod using entries provided in the request body.
// @Description No database lookup of entries is performed — suitable for previewing
// @Description calculations before an entry is submitted.
// @Tags calculation
// @Accept  json
// @Produce json
// @Param request body RqCalculateFromEntries true "Form ID, entries, and optional super component"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculate [post]
func (h *handler) CalculateFromEntries(c *gin.Context) {
	// Get Role and appropriate ID
	role := c.GetString("role")
	var actorID uuid.UUID
	var ok bool

	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	if !ok {
		response.Error(c, http.StatusUnauthorized, nil)
		return
	}
	var req RqCalculateFromEntries
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.CalculateFromEntries(c.Request.Context(), &req, actorID, role)
	if err != nil {
		if errors.Is(err, entry.ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Calculation completed successfully")
}

// FormulaCalculate godoc
// @Summary Calculate computed fields for a form
// @Description Evaluates all is_computed=true fields using manual field key→amount query params.
// @Description Returns net amount for each computed field; net/gst/gross when the field has a tax_type.
// @Description Pass each non-computed field key as a query param, e.g. ?A=5000&B=300&C=55&D=20
// @Tags calculation
// @Produce json
// @Param form_id path string true "Form ID"
// @Param A query number false "Value for field A"
// @Param B query number false "Value for field B"
// @Param C query number false "Value for field C"
// @Param D query number false "Value for field D"
// @Success 200 {object} RsFormulaCalculate
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculate/formula/{form_id} [get]
func (h *handler) FormulaCalculate(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}

	// Parse all query params as field key → float64 amount.
	values := make(map[string]float64)
	for key, vals := range c.Request.URL.Query() {
		if len(vals) == 0 {
			continue
		}
		v, err := strconv.ParseFloat(vals[0], 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid value for field %q: must be a number", key))
			return
		}
		values[key] = v
	}

	if len(values) == 0 {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("at least one field value is required as a query param (e.g. ?A=5000&B=300)"))
		return
	}

	req := &RqFormulaCalculate{Values: values}

	result, err := h.svc.FormulaCalculate(c.Request.Context(), formID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, result, "Formula calculation completed successfully")
}

// GetFormSummary godoc
// @Summary      Get form summary by form ID
// @Description  Fetches all individual entries/transactions for a specific form ID, including field names, COA details, and tax information.
// @Tags         calculation
// @Produce      json
// @Param        id   path      string  true  "Form ID (UUID)"
// @Success      200  {array}  RsTransactionRow "List of transactions"
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /summary/{id} [get]
func (h *handler) GetFormSummary(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	// Extract security context
	role := c.GetString("role")
	var actorID uuid.UUID

	if strings.EqualFold(role, util.RoleAccountant) {
		actorID, ok = util.GetAccountantID(c)
	} else {
		actorID, ok = util.GetPractitionerID(c)
	}

	if !ok {
		response.Error(c, http.StatusUnauthorized, fmt.Errorf("unauthorized access"))
		return
	}

	// Call service and return the raw util.RsList
	data, err := h.svc.GetFormSummary(c.Request.Context(), id.String(), actorID, role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, data, "Form summary fetched successfully")
}
