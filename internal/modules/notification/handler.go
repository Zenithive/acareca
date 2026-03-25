package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IHandler interface {
	ListNotifications(c *gin.Context)
	MarkRead(c *gin.Context)
	MarkDismissed(c *gin.Context)
}

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListNotifications(c *gin.Context) {
	uid, ok := util.GetUserID(c)
	if !ok {
		return
	}

	var filter Filter
	if err := util.BindAndValidate(c, &filter); err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	res, err := h.svc.ListNotifications(c.Request.Context(), uid, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err)
		return
	}

	response.JSON(c, http.StatusOK, res, "Notifications fetched successfully")
}

func (h *Handler) MarkRead(c *gin.Context) {
	uid, ok := util.GetUserID(c)
	if !ok {
		return
	}

	idParam := c.Param("id")
	nid, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), uid, nid); err != nil {
		response.Error(c, http.StatusNotFound, err)
		return
	}

	response.Message(c, http.StatusOK, "Notification marked as read")
}

func (h *Handler) MarkDismissed(c *gin.Context) {
	uid, ok := util.GetUserID(c)
	if !ok {
		return
	}

	idParam := c.Param("id")
	nid, err := uuid.Parse(idParam)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.MarkDismissed(c.Request.Context(), uid, nid); err != nil {
		response.Error(c, http.StatusNotFound, err)
		return
	}

	response.Message(c, http.StatusOK, "Notification dismissed")
}
