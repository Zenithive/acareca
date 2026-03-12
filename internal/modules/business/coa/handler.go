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
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

func (h *handler) ListAccountTypes(c *gin.Context) {
	list, err := h.svc.ListAccountTypes(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

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
	response.JSON(c, http.StatusOK, one)
}

func (h *handler) ListAccountTaxes(c *gin.Context) {
	list, err := h.svc.ListAccountTaxes(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

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
	response.JSON(c, http.StatusOK, one)
}

func (h *handler) ListChartOfAccount(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	list, err := h.svc.ListChartOfAccount(c.Request.Context(), practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

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
	response.JSON(c, http.StatusOK, chart)
}

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
	response.JSON(c, http.StatusCreated, created)
}

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
	response.JSON(c, http.StatusOK, updated)
}

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
	response.JSON(c, http.StatusOK, gin.H{"message": "deleted"})
}
