package setting

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreateTentant(c *gin.Context)
	GetTentant(c *gin.Context)
	GetTentantByUserID(c *gin.Context)
	ListTentants(c *gin.Context)
	UpdateTentant(c *gin.Context)
	DeleteTentant(c *gin.Context)
	GetSetting(c *gin.Context)
	UpsertSetting(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

func parseTentantID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid practitioner id"))
		return 0, false
	}
	return id, true
}

func (h *handler) CreateTentant(c *gin.Context) {
	var req RqCreateTentant
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreateTentant(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

func (h *handler) GetTentant(c *gin.Context) {
	id, ok := parseTentantID(c)
	if !ok {
		return
	}
	t, err := h.svc.GetTentant(c.Request.Context(), id)
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

func (h *handler) GetTentantByUserID(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		response.Error(c, http.StatusBadRequest, errors.New("user_id required"))
		return
	}
	t, err := h.svc.GetTentantByUserID(c.Request.Context(), userID)
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

func (h *handler) ListTentants(c *gin.Context) {
	list, err := h.svc.ListTentants(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

func (h *handler) UpdateTentant(c *gin.Context) {
	id, ok := parseTentantID(c)
	if !ok {
		return
	}
	var req RqUpdateTentant
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdateTentant(c.Request.Context(), id, &req)
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

func (h *handler) DeleteTentant(c *gin.Context) {
	id, ok := parseTentantID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTentant(c.Request.Context(), id); err != nil {
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
	id, ok := parseTentantID(c)
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
	id, ok := parseTentantID(c)
	if !ok {
		return
	}
	var req RqUpsertTentantSetting
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
