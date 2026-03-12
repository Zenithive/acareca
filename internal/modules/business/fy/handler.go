package fy

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreateFY(c *gin.Context)
	UpdateFYLabel(c *gin.Context)
	GetFinancialYears(c *gin.Context)
	GetFinancialQuarters(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

func (h *handler) CreateFY(c *gin.Context) {
	var req RqCreateFY
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	fy, err := h.svc.CreateFY(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrInvalidFYYearFormat) {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusCreated, fy)
}

func (h *handler) UpdateFYLabel(c *gin.Context) {
	idParam := c.Param("financial_year_id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid financial year id"))
		return
	}

	var req RqUpdateFYLabel
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	fy, err := h.svc.UpdateFYLabel(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, fy)
}

func (h *handler) GetFinancialYears(c *gin.Context) {
	years, err := h.svc.GetFinancialYears(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, years)
}

func (h *handler) GetFinancialQuarters(c *gin.Context) {
	idParam := c.Param("financial_year_id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid financial year id"))
		return
	}

	quarters, err := h.svc.GetFinancialQuarters(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, quarters)
}
