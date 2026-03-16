package subscription

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	ListByPractitionerID(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary Get a subscription by ID
// @Description get a subscription by ID
// @Tags subscription
// @Accept json
// @Produce json
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{id} [get]
// @Param id path int true "Subscription ID"
func (h *handler) Create(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	var req RqCreatePractitionerSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.Create(c.Request.Context(), practitionerID, &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created)
}

// @Summary List subscriptions by practitioner ID
// @Description list subscriptions by practitioner ID
// @Tags subscription
// @Accept json
// @Produce json
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription [get]
// @Param practitioner_id path string true "Practitioner ID"
func (h *handler) GetByID(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
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

// @Summary List subscriptions by practitioner ID
// @Description list subscriptions by practitioner ID
// @Tags subscription
// @Accept json
// @Produce json
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription [get]
// @Param practitioner_id path string true "Practitioner ID"
func (h *handler) ListByPractitionerID(c *gin.Context) {
	practitionerID, ok := util.GetPractitionerID(c)
	if !ok {
		return
	}
	list, err := h.svc.ListByPractitionerID(c.Request.Context(), practitionerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list)
}

// @Summary Update a subscription
// @Description update a subscription
// @Tags subscription
// @Accept json
// @Produce json
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{id} [put]
// @Param id path int true "Subscription ID"
// @Param practitioner_id path string true "Practitioner ID"
func (h *handler) Update(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
	if !ok {
		return
	}
	var req RqUpdatePractitionerSubscription
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

// @Summary Delete a subscription
// @Description delete a subscription
// @Tags subscription
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /practitioner/subscription/{id} [delete]
// @Param id path int true "Subscription ID"
// @Param practitioner_id path string true "Practitioner ID"
func (h *handler) Delete(c *gin.Context) {
	id, ok := util.ParseIntID(c, "sub_id")
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
	response.JSON(c, http.StatusOK, map[string]string{"message": "deleted"})
}
