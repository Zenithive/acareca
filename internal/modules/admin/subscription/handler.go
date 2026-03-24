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

	// Permission management
	ListPermissions(c *gin.Context)
	UpdatePermission(c *gin.Context)
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
// @Success 201 {object} response.RsBase
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
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
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
// @Param name query string false "Filter by subscription name"
// @Param search query string false "Search in name and description"
// @Param sort_by query string false "Field to sort by (default: created_at)"
// @Param order_by query string false "Sort order: ASC or DESC (default: DESC)"
// @Param limit query int false "Number of items per page (default: 20)"
// @Param offset query int false "Number of items to skip"
// @Success 200 {object} util.RsList
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription [get]
func (h *handler) ListSubscriptions(c *gin.Context) {
	var f Filter
	if err := util.BindAndValidate(c, &f); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	list, err := h.svc.ListSubscriptions(c.Request.Context(), &f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Subscriptions fetched successfully")
}

// @Summary Update a subscription
// @Description update a subscription
// @Tags admin-subscription
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Param request body RqUpdateSubscription true "Updated Subscription Data"
// @Success 200 {object} response.RsBase
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

// @Summary List permissions for a subscription plan
// @Description Returns all permission keys and their limits for a given plan
// @Tags subscription
// @Produce json
// @Param id path int true "Subscription ID"
// @Success 200 {array} RsSubscriptionPermission
// @Failure 400 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription/{id}/permissions [get]
func (h *handler) ListPermissions(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	list, err := h.svc.ListPermissions(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, list, "Permissions fetched successfully")
}

// @Summary Update a permission limit for a subscription plan
// @Description Update usage_limit or is_enabled for a specific permission key on a plan
// @Tags subscription
// @Accept json
// @Produce json
// @Param id path int true "Subscription ID"
// @Param key path string true "Permission key (e.g. clinic.create)"
// @Param request body RqUpdatePermission true "Permission update"
// @Success 200 {object} response.RsBase
// @Failure 400 {object} response.RsError
// @Failure 404 {object} response.RsError
// @Failure 500 {object} response.RsError
// @Security BearerToken
// @Router /admin/subscription/{id}/permissions/{key} [put]
func (h *handler) UpdatePermission(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	key := c.Param("key")
	if key == "" {
		response.Error(c, http.StatusBadRequest, errors.New("permission key is required"))
		return
	}
	var req RqUpdatePermission
	if err := util.BindAndValidate(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.svc.UpdatePermission(c.Request.Context(), id, key, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, updated, "Permission updated successfully")
}
