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

// @Summary Create a new financial year
// @Tags fy
// @Accept json
// @Produce json
// @Param request body RqCreateFY true "Financial Year Data"
// @Success 201 {object} RsFinancialYear
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /fy [post]
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

// @Summary Update the label of a financial year
// @Tags fy
// @Accept json
// @Produce json
// @Param financial_year_id path string true "Financial Year UUID"
// @Param request body RqUpdateFYLabel true "Updated Label Data"
// @Success 200 {object} RsFinancialYear
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /fy/{financial_year_id}/label [put]
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

// @Summary Get all financial years
// @Tags fy
// @Produce json
// @Success 200 {array} RsFinancialYear
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /fy [get]
func (h *handler) GetFinancialYears(c *gin.Context) {
	years, err := h.svc.GetFinancialYears(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, years)
}

// @Summary Get all quarters for a specific financial year
// @Tags fy
// @Produce json
// @Param financial_year_id path string true "Financial Year UUID"
// @Success 200 {array} RsFinancialQuarter
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /fy/{financial_year_id}/quarters [get]
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
