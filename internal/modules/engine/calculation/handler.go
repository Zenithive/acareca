package calculation

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	NetAmount(c *gin.Context)
	NetResult(c *gin.Context)
	GrossResult(c *gin.Context)
	OutWorkResult(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary Calculate net amount
// @Tags calculation
// @Accept json
// @Produce json
// @Param request body Entry true "Calculation Entry Data"
// @Success 200 {object} NetAmountResult
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculation/net-amount [post]
func (h *handler) NetAmount(c *gin.Context) {
	var entry Entry
	if err := util.BindAndValidate(c, &entry); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.NetAmount(c.Request.Context(), &entry)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Net amount calculated successfully")
}

// @Summary Calculate net result
// @Tags calculation
// @Accept json
// @Produce json
// @Param request body Entry true "Calculation Entry Data"
// @Success 200 {object} NetResult
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculation/net-result [post]
func (h *handler) NetResult(c *gin.Context) {
	var entry Entry
	if err := util.BindAndValidate(c, &entry); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.NetResult(c.Request.Context(), &entry)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Net result calculated successfully")
}

// @Summary Calculate gross result
// @Tags calculation
// @Accept json
// @Produce json
// @Param request body Entry true "Calculation Entry Data"
// @Success 200 {object} GrossResult
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculation/gross-result [post]
func (h *handler) GrossResult(c *gin.Context) {
	var entry Entry
	if err := util.BindAndValidate(c, &entry); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.GrossResult(c.Request.Context(), &entry)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Gross result calculated successfully")
}

// @Summary Calculate outwork result
// @Tags calculation
// @Accept json
// @Produce json
// @Param request body Entry true "Calculation Entry Data"
// @Success 200 {object} OutWorkResult
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /calculation/outwork-result [post]
func (h *handler) OutWorkResult(c *gin.Context) {
	var entry Entry
	if err := util.BindAndValidate(c, &entry); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.OutWorkResult(c.Request.Context(), &entry)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "Outwork result calculated successfully")
}
