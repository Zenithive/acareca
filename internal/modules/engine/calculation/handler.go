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

// NetAmount implements [IHandler].
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
	response.JSON(c, http.StatusOK, result)
}

// NetResult implements [IHandler].
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
	response.JSON(c, http.StatusOK, result)
}

// GrossResult implements [IHandler].
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
	response.JSON(c, http.StatusOK, result)
}

// OutWorkResult implements [IHandler].
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
	response.JSON(c, http.StatusOK, result)
}
