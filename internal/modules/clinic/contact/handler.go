package contact

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

	DeleteAddressByID(c *gin.Context)
}

type handler struct {
	svc IService
}

func NewHandler(svc IService) IHandler {
	return &handler{svc: svc}
}

// Create implements [IHandler].
// @Summary Create a new contact for a clinic
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Param request body RqContact true "Contact Data"
// @Success 200 {object} response.RsBase{data=RsContact}
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact [post]
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
// @Summary Delete a contact by ID
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Param id path string true "Contact ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Delete(c, uuid); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "contact deleted")
}

// Get implements [IHandler].
// @Summary Get a contact by ID
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Param id path string true "Contact ID"
// @Success 200 {object} response.RsBase{data=RsContact}
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact/{id} [get]
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
// @Summary List all contacts for a clinic
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Success 200 {object} util.RsList
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact [get]
func (h *handler) List(c *gin.Context) {
	clinicId, ok := util.GetEntityID(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, errors.New("clinic not found!"))
		return
	}

	var ft Filter
	if err := util.BindAndValidate(c, &ft); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	contacts, err := h.svc.List(c, clinicId, &ft)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	response.JSON(c, http.StatusOK, contacts, "contacts retrieved")
}

// Update implements [IHandler].
// @Summary Update a contact by ID
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Param id path string true "Contact ID"
// @Param request body RqUpdateContact true "Updated Contact Data"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact/{id} [put]
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

// DeleteAddressByID implements [IHandler].
// @Summary Delete a contact address by ID
// @Tags clinic-contact
// @Accept json
// @Produce json
// @Param id path string true "Contact Address ID"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /clinic/contact/address/{id} [delete]
func (h *handler) DeleteAddressByID(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.DeleteAddressByID(c, uuid); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "address deleted")
}
