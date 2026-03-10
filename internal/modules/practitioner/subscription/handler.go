package subscription

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	ListByTentantID(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// tentantIDFromCtx is set by routes when mounting under /tentants/:id/subscriptions
const tentantIDKey = "tentant_id"

func getTentantID(c *gin.Context) (int, bool) {
	idStr, exists := c.Get(tentantIDKey)
	if !exists {
		response.Error(c, http.StatusBadRequest, errors.New("practitioner id not in context"))
		return 0, false
	}
	id, ok := idStr.(int)
	if !ok {
		response.Error(c, http.StatusInternalServerError, errors.New("invalid practitioner id type"))
		return 0, false
	}
	return id, true
}

func parseID(c *gin.Context, param string) (int, bool) {
	id, err := strconv.Atoi(c.Param(param))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return 0, false
	}
	return id, true
}

func (h *handler) Create(c *gin.Context) {
	tentantID, ok := getTentantID(c)
	if !ok {
		return
	}
	var req RqCreateTentantSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.Create(c.Request.Context(), tentantID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

func (h *handler) GetByID(c *gin.Context) {
	id, ok := parseID(c, "sub_id")
	if !ok {
		return
	}
	sub, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, sub)
}

func (h *handler) ListByTentantID(c *gin.Context) {
	tentantID, ok := getTentantID(c)
	if !ok {
		return
	}
	list, err := h.svc.ListByTentantID(c.Request.Context(), tentantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

func (h *handler) Update(c *gin.Context) {
	id, ok := parseID(c, "sub_id")
	if !ok {
		return
	}
	var req RqUpdateTentantSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), id, &req)
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

func (h *handler) Delete(c *gin.Context) {
	id, ok := parseID(c, "sub_id")
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
	response.JSON(c, http.StatusOK, gin.H{"message": "deleted"})
}
