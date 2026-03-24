package coa

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	ListAccountTypes(c *gin.Context)
	GetAccountType(c *gin.Context)
	ListAccountTaxes(c *gin.Context)
	GetAccountTax(c *gin.Context)

	ListChartOfAccount(c *gin.Context)
	GetChartOfAccount(c *gin.Context)
	CreateChartOfAccount(c *gin.Context)
	UpdateCharOfAccount(c *gin.Context)
	DeleteChartOfAccount(c *gin.Context)
	CheckCodeUnique(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary List all account types
// @Tags coa
// @Produce json
// @Param id query int false "Filter by id"
// @Param name query string false "Filter by name"
// @Param search query string false "Search name"
// @Success 200 {object} util.RsList
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/account-types [get]
func (h *handler) ListAccountTypes(c *gin.Context) {
	var f Filter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	list, err := h.svc.ListAccountTypes(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Account types fetched successfully")
}

// @Summary Get account type by ID
// @Tags coa
// @Produce json
// @Param id path int true "Account Type ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/account-types/{id} [get]
func (h *handler) GetAccountType(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 16)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	one, err := h.svc.GetAccountType(c.Request.Context(), int16(id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, one, "Account tax fetched successfully")
}

// @Summary List all account tax types
// @Tags coa
// @Produce json
// @Param id query int false "Filter by id"
// @Param name query string false "Filter by name"
// @Param rate query number false "Filter by rate"
// @Param search query string false "Search name or is_taxable"
// @Success 200 {object} util.RsList
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/account-taxes [get]
func (h *handler) ListAccountTaxes(c *gin.Context) {
	var f Filter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	list, err := h.svc.ListAccountTaxes(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Account taxes fetched successfully")
}

// @Summary Get account tax by ID
// @Tags coa
// @Produce json
// @Param id path int true "Account Tax ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/account-taxes/{id} [get]
func (h *handler) GetAccountTax(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 16)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	one, err := h.svc.GetAccountTax(c.Request.Context(), int16(id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, one, "Account tax fetched successfully")
}

// @Summary List chart of accounts for practitioner
// @Tags coa
// @Produce json
// @Param name query string false "Filter by name"
// @Param code query int false "Filter by code"
// @Param account_type query string false "Filter by account type name"
// @Param search query string false "Search keyword"
// @Param sort_by query string false "Sort field"
// @Param order_by query string false "Order direction (ASC/DESC)"
// @Param limit query int false "Page size (default 20, max 100)"
// @Param offset query int false "Offset"
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account [get]
func (h *handler) ListChartOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.ListChartOfAccount(c.Request.Context(), practitionerID, &filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Chart of accounts fetched successfully")
}

// @Summary Get chart of account by ID
// @Tags coa
// @Produce json
// @Param id path string true "Chart of Account UUID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account/{id} [get]
func (h *handler) GetChartOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	chart, err := h.svc.GetChartOfAccount(c.Request.Context(), id, practitionerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, chart, "Chart of account fetched successfully")
}

// @Summary Create a new chart of account
// @Tags coa
// @Accept json
// @Produce json
// @Param request body RqCreateChartOfAccountOfAccount true "COA Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account [post]
func (h *handler) CreateChartOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqCreateChartOfAccountOfAccount
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreateChartOfAccount(c.Request.Context(), practitionerID, &req)
	if err != nil {
		if errors.Is(err, ErrCodeExists) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created, "Chart of account created successfully")
}

// @Summary Update an existing chart of account
// @Tags coa
// @Accept json
// @Produce json
// @Param id path string true "Chart of Account UUID"
// @Param request body RqUpdateCharOfAccountOfAccount true "Updated COA Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 409 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account/{id} [put]
func (h *handler) UpdateCharOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	var req RqUpdateCharOfAccountOfAccount
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdateCharOfAccount(c.Request.Context(), id, practitionerID, &req)
	if err != nil {
		if errors.Is(err, ErrCodeExists) {
			response.Error(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, ErrSystemAccountProtected) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Chart of account updated successfully")
}

// @Summary Delete chart of account
// @Tags coa
// @Produce json
// @Param id path string true "Chart of Account UUID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 403 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account/{id} [delete]
func (h *handler) DeleteChartOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	if err := h.svc.DeleteChartOfAccount(c.Request.Context(), id, practitionerID); err != nil {
		if errors.Is(err, ErrSystemAccountProtected) {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"message": "deleted"}, "Chart of account deleted successfully")
}

// @Summary Check if a chart of account code is unique for the practitioner
// @Tags coa
// @Accept json
// @Produce json
// @Param request body RqCheckCodeUnique true "Code to check"
// @Success 200 {object} RsCodeUnique
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /coa/chart-of-account/check-code [post]
func (h *handler) CheckCodeUnique(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}

	var req RqCheckCodeUnique
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.CheckCodeUnique(c.Request.Context(), practitionerID, req.Code, req.ExcludeID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Code uniqueness checked successfully")
}
