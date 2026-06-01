package notification

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	ListNotifications(c *gin.Context)
	MarkRead(c *gin.Context)
	MarkAllRead(c *gin.Context)
	MarkDismissed(c *gin.Context)
	MarkAllDismissed(c *gin.Context)
	GetPreferences(c *gin.Context)
	UpdatePreference(c *gin.Context)
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
// @Success      200     {object}  response.RsBase{data=util.RsList}
// @Failure      400     {object}  response.RsError
// @Failure      401     {object}  response.RsError
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
// @Failure      401  {object}  response.RsError
// @Failure  	 404  {object}  response.RsError
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

// @Summary      Mark all notifications as read
// @Description  Marks all currently UNREAD notifications as READ for the authenticated entity.
// @Tags         notification
// @Produce      json
// @Success      200  {object}  response.RsBase
// @Failure      401  {object}  response.RsError
// @Failure      409  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/read-all [patch]
func (h *handler) MarkAllRead(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	if err := h.svc.MarkAllRead(c.Request.Context(), entityID); err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "all notifications marked as read")
}

// @Summary      Mark a notification as dismissed
// @Description  Dismisses a specific notification by its UUID for the authenticated clinic/practitioner entity context.
// @Tags         notification
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Notification UUID"  Format(uuid)
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      409  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/{id}/dismiss [patch]
func (h *handler) MarkDismissed(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	id, ok := util.ParseUuidID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.MarkDismissed(c.Request.Context(), []uuid.UUID{id}, entityID); err != nil {
		h.handleTransitionError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "dismissed")
}

// @Summary      Dismiss multiple notifications
// @Description  Marks a list of specific notifications as DISMISSED for the authenticated entity.
// @Tags         notification
// @Accept       json
// @Produce      json
// @Param        request body     RqBulkDismiss  true  "List of Notification UUIDs"
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      409  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/dismiss [patch]
func (h *handler) MarkAllDismissed(c *gin.Context) {
	entityID, ok := util.GetEntityID(c)
	if !ok {
		return
	}

	var req RqBulkDismiss
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.MarkDismissed(c.Request.Context(), req.IDs, entityID); err != nil {
		h.handleTransitionError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, nil, "dismissed")
}

// @Summary      Get notification preferences
// @Description  Returns the current channel preferences for the authenticated user.
// @Tags         notification
// @Success      200  {object}  response.RsBase
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/preferences [get]
func (h *handler) GetPreferences(c *gin.Context) {
	userID, ok := util.GetUserID(c)
	if !ok {
		return
	}

	prefs, err := h.svc.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}
	response.JSON(c, http.StatusOK, prefs, "Notification preferences fetched successfully")
}

// @Summary      Update notification preference
// @Description  Updates or creates a preference for a specific event type.
// @Tags         notification
// @Param        body  body  RqUpdatePreference  true  "Preference Update"
// @Success      200  {object}  response.RsBase
// @Failure      400  {object}  response.RsError
// @Failure      401  {object}  response.RsError
// @Failure      404  {object}  response.RsError
// @Failure      500  {object}  response.RsError
// @Security     BearerToken
// @Router       /notification/preferences [put]
func (h *handler) UpdatePreference(c *gin.Context) {
	userID, okUser := util.GetUserID(c)
	actorID, okEntity := util.GetEntityID(c)
	if !okUser || !okEntity {
		return
	}

	role := c.GetString("role")

	var rq RqUpdatePreference
	if err := c.ShouldBindJSON(&rq); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	err := h.svc.UpdatePreference(c.Request.Context(), userID, actorID, role, rq)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, nil, "Notification preferences updated successfully")
}

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
