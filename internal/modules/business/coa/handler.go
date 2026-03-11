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
	GetAccountTypeByID(c *gin.Context)
	ListAccountTaxes(c *gin.Context)
	GetAccountTaxByID(c *gin.Context)

	ListChartsBypractice_id(c *gin.Context)
	GetChartByIDAndpractice_id(c *gin.Context)
	CreateChart(c *gin.Context)
	UpdateChart(c *gin.Context)
	DeleteChart(c *gin.Context)
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

func (h *handler) GetAccountTypeByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 16)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	one, err := h.svc.GetAccountTypeByID(c.Request.Context(), int16(id))
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

func (h *handler) GetAccountTaxByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 16)
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	one, err := h.svc.GetAccountTaxByID(c.Request.Context(), int16(id))
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

func (h *handler) parsepractice_idID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("practice_idId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid practice_idId"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *handler) ListChartsBypractice_id(c *gin.Context) {
	practice_id, ok := h.parsepractice_idID(c)
	if !ok {
		return
	}
	list, err := h.svc.ListChartsBypractice_id(c.Request.Context(), practice_id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

func (h *handler) GetChartByIDAndpractice_id(c *gin.Context) {
	practice_id, ok := h.parsepractice_idID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	chart, err := h.svc.GetChartByIDAndpractice_id(c.Request.Context(), id, practice_id)
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

func (h *handler) CreateChart(c *gin.Context) {
	practice_id, ok := h.parsepractice_idID(c)
	if !ok {
		return
	}
	var req RqCreateChartOfAccount
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreateChart(c.Request.Context(), practice_id, &req)
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

func (h *handler) UpdateChart(c *gin.Context) {
	practice_id, ok := h.parsepractice_idID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	var req RqUpdateChartOfAccount
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdateChart(c.Request.Context(), id, practice_id, &req)
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

func (h *handler) DeleteChart(c *gin.Context) {
	practice_id, ok := h.parsepractice_idID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	if err := h.svc.DeleteChart(c.Request.Context(), id, practice_id); err != nil {
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
