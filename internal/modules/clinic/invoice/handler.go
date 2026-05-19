package invoice

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

type Handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &Handler{
		svc: svc,
	}
}

// Create implements [IHandler].
func (h *Handler) Create(c *gin.Context) {
	var rq RqInvoice
	if err := util.BindAndValidate(c, &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.Create(c.Request.Context(), &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusCreated, nil, "invoice created")
}

// Delete implements [IHandler].
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "invoice deleted")
}

// Get implements [IHandler].
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	invoice, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, invoice, "invoice retrieved")
}

// List implements [IHandler].
func (h *Handler) List(c *gin.Context) {
	invoices, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, invoices, "invoices retrieved")
}

// Update implements [IHandler].
func (h *Handler) Update(c *gin.Context) {
	var rq RqUpdateInvoice
	if err := util.BindAndValidate(c, &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	rq.ID = id

	if err := h.svc.Update(c.Request.Context(), &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "invoice updated")
}
