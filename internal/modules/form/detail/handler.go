package detail

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	CreateForm(c *gin.Context)
	GetForm(c *gin.Context)
	ListForm(c *gin.Context)
	UpdateForm(c *gin.Context)
	DeleteForm(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// CreateForm implements [IHandler].
func (h *handler) CreateForm(c *gin.Context) {
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	var req RqFormDetail
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.Create(c.Request.Context(), &req, clinicID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

// DeleteForm implements [IHandler].
func (h *handler) DeleteForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusNoContent, nil)
}

// GetForm implements [IHandler].
func (h *handler) GetForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	form, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, form)
}

// ListForm implements [IHandler].
func (h *handler) ListForm(c *gin.Context) {
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	filter := Filter{ClinicID: clinicID}
	forms, err := h.svc.ListForm(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, forms)
}

// UpdateForm implements [IHandler].
func (h *handler) UpdateForm(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	var req RqUpdateFormDetail
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	req.ID = id
	updated, err := h.svc.Update(c.Request.Context(), &req)
	if err != nil {
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated)
}
