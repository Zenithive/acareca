package invoice

import (
	"errors"
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
// @Summary Create a new invoice for a clinic
// @Tags invoice
// @Accept json
// @Produce json
// @Param request body RqInvoice true "Invoice Data"
// @Success 201 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/invoice [post]
func (h *Handler) Create(c *gin.Context) {
	clinicId, ok := util.GetEntityID(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, errors.New("clinic not found!!"))
		return
	}
	var rq RqInvoice
	if err := util.BindAndValidate(c, &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	rq.ClinicID = clinicId

	if err := h.svc.Create(c.Request.Context(), &rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusCreated, nil, "invoice created")
}

// Delete implements [IHandler].
// @Summary Delete an invoice by ID
// @Tags invoice
// @Accept json
// @Produce json
// @Param id path string true "Invoice ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/invoice/{id} [delete]
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
// @Summary Get an invoice by ID
// @Tags invoice
// @Accept json
// @Produce json
// @Param id path string true "Invoice ID"
// @Success 200 {object} response.RsBase{data=RsInvoice}
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/invoice/{id} [get]
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
// @Summary List all invoices for a clinic
// @Tags invoice
// @Accept json
// @Produce json
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/invoice [get]
func (h *Handler) List(c *gin.Context) {
	var ft Filter
	if err := util.BindAndValidate(c, &ft); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	invoices, err := h.svc.List(c.Request.Context(), &ft)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	response.JSON(c, http.StatusOK, invoices, "invoices retrieved")
}

// Update implements [IHandler].
// @Summary Update an invoice by ID
// @Tags invoice
// @Accept json
// @Produce json
// @Param id path string true "Invoice ID"
// @Param request body RqUpdateInvoice true "Updated Invoice Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/invoice/{id} [put]
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
