package version

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	List(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// Create implements [IHandler].
func (h *handler) Create(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	var req RqFormVersion
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	userID := uuid.Nil // TODO: from auth context when available
	created, err := h.svc.Create(c.Request.Context(), formID, clinicID, &req, userID)
	if err != nil {
		if err == ErrForbidden {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

// Get implements [IHandler].
func (h *handler) Get(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	v, err := h.svc.Get(c.Request.Context(), id, clinicID)
	if err != nil {
		if err == ErrForbidden {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, v)
}

// Update implements [IHandler].
func (h *handler) Update(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	var req RqUpdateFormVersion
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), id, clinicID, &req)
	if err != nil {
		if err == ErrForbidden {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated)
}

// Delete implements [IHandler].
func (h *handler) Delete(c *gin.Context) {
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id, clinicID); err != nil {
		if err == ErrForbidden {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		if err == ErrNotFound {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusNoContent, nil)
}

// List implements [IHandler].
func (h *handler) List(c *gin.Context) {
	formID, ok := util.ParseUuidID(c, "form_id")
	if !ok {
		return
	}
	clinicID, ok := util.GetClinicID(c)
	if !ok {
		return
	}
	list, err := h.svc.List(c.Request.Context(), formID, clinicID)
	if err != nil {
		if err == ErrForbidden {
			response.Error(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}
