package contact

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Get(c *gin.Context)
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
	var rq RqContact

	if err := util.BindAndValidate(c, &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	contact, err := h.svc.Create(c, rq)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, contact, "contact created")
}

// Delete implements [IHandler].
func (h *handler) Delete(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	h.svc.Delete(c, uuid)
	response.JSON(c, http.StatusOK, nil, "contact deleted")
}

// Get implements [IHandler].
func (h *handler) Get(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	contact, err := h.svc.Get(c, uuid)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	response.JSON(c, http.StatusOK, contact, "contact retrieved")

}

// List implements [IHandler].
func (h *handler) List(c *gin.Context) {
	contacts, err := h.svc.List(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	response.JSON(c, http.StatusOK, contacts, "contacts retrieved")
}

// Update implements [IHandler].
func (h *handler) Update(c *gin.Context) {
	var rq RqUpdateContact

	if err := util.BindAndValidate(c, &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	rq.ID = uuid

	if err := h.svc.Update(c, rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "contact updated")

}
