package setting

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreatePractitioner(c *gin.Context)
	GetPractitioner(c *gin.Context)
	GetPractitionerByUserID(c *gin.Context)
	ListPractitioners(c *gin.Context)
	UpdatePractitioner(c *gin.Context)
	DeletePractitioner(c *gin.Context)
	GetSetting(c *gin.Context)
	UpsertSetting(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

func (h *handler) CreatePractitioner(c *gin.Context) {
	var req RqCreatePractitioner
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreatePractitioner(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

func (h *handler) GetPractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	t, err := h.svc.GetPractitioner(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, t)
}

func (h *handler) GetPractitionerByUserID(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		response.Error(c, http.StatusBadRequest, errors.New("user_id required"))
		return
	}
	t, err := h.svc.GetPractitionerByUserID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, t)
}

func (h *handler) ListPractitioners(c *gin.Context) {
	list, err := h.svc.ListPractitioners(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

func (h *handler) UpdatePractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpdatePractitioner
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdatePractitioner(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated)
}

func (h *handler) DeletePractitioner(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	if err := h.svc.DeletePractitioner(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, gin.H{"message": "deleted"})
}

func (h *handler) GetSetting(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	setting, err := h.svc.GetSetting(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, setting)
}

func (h *handler) UpsertSetting(c *gin.Context) {
	id, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqUpsertPractitionerSetting
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	setting, err := h.svc.UpsertSetting(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, setting)
}
