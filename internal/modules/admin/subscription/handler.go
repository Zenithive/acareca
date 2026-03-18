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
	CreateSubscription(c *gin.Context)
	GetSubscription(c *gin.Context)
	ListSubscriptions(c *gin.Context)
	UpdateSubscription(c *gin.Context)
	DeleteSubscription(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary Create a new subscription
// @Description create a new subscription
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Param request body RqCreateSubscription true "Subscription Data"
// @Success 201 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription [post]
func (h *handler) CreateSubscription(c *gin.Context) {
	var req RqCreateSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.svc.CreateSubscription(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusCreated, created, "Subscription created successfully")
}

// @Summary Get a subscription
// @Description get a subscription
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription/{id} [get]
func (h *handler) GetSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	sub, err := h.svc.GetSubscription(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, sub, "Subscription fetched successfully")
}

// @Summary List subscriptions
// @Description list subscriptions
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Success 200 {object} util.RsList
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription [get]
func (h *handler) ListSubscriptions(c *gin.Context) {
	list, err := h.svc.ListSubscriptions(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, util.RsList{Items: list, Total: len(list)}, "Subscriptions fetched successfully")
}

// @Summary Update a subscription
// @Description update a subscription
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Param request body RqUpdateSubscription true "Updated Subscription Data"
// @Success 200 {object} RsSubscription
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription/{id} [patch]
func (h *handler) UpdateSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	var req RqUpdateSubscription
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdateSubscription(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Subscription updated successfully")
}

// @Summary Delete a subscription
// @Description delete a subscription
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription/{id} [delete]
func (h *handler) DeleteSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	if err := h.svc.DeleteSubscription(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"message": "deleted"}, "Subscription deleted successfully")
}
