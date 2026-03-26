package notification

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	ListNotifications(c *gin.Context)
	MarkRead(c *gin.Context)
	MarkDismissed(c *gin.Context)
}

type handler struct {
	svc Service
}

func NewHandler(svc Service) IHandler {
	return &handler{svc: svc}
}

// @Summary      List notifications
// @Description  Returns a paginated list of notifications for the authenticated entity, along with unread count.
// @Tags         notification
// @Produce      json
// @Param        status  query     string  false  "Filter by status (UNREAD, READ, DISMISSED)"
// @Param        limit   query     int     false  "Number of records to return"
// @Param        page    query     int     false  "Page number"
// @Success      200     {object}  response.RsBase{data=RsListNotification}
// @Failure      400     {object}  response.RsError
// @Failure      500     {object}  response.RsError
// @Security     BearerToken
// @Router       /notification [get]
func (h *handler) ListNotifications(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	var filter FilterNotification
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.svc.List(c.Request.Context(), entityID, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, result, "")
}

// @Summary      Mark notification as read
// @Description  Marks a specific notification as READ for the authenticated entity.
// @Tags         notification
// @Produce      json
// @Param        id   path      string  true  "Notification UUID"
// @Success      200  {object}  response.RsBase
// @Failure      404  {object}  response.RsError
// @Failure      409  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/{id}/read [patch]
func (h *handler) MarkRead(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), id, entityID); err != nil {
		h.handleTransitionError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "marked as read")
}

// @Summary      Dismiss a notification
// @Description  Marks a specific notification as DISMISSED for the authenticated entity.
// @Tags         notification
// @Produce      json
// @Param        id   path      string  true  "Notification UUID"
// @Success      200  {object}  response.RsBase
// @Failure      404  {object}  response.RsError
// @Failure      409  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/{id}/dismissed [patch]
func (h *handler) MarkDismissed(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}
	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.MarkDismissed(c.Request.Context(), id, entityID); err != nil {
		h.handleTransitionError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "dismissed")
}

// handleTransitionError maps sentinel errors to the correct HTTP status.
func (h *handler) handleTransitionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, err)
	case errors.Is(err, ErrInvalidTransition):
		response.Error(c, http.StatusConflict, err)
	case errors.Is(err, ErrMaxRetriesExceeded):
		response.Error(c, http.StatusUnprocessableEntity, err)
	default:
		response.Error(c, http.StatusInternalServerError, err)
	}
}
