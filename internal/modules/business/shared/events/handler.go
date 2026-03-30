package events

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/response"
)

type Handler struct {
	service Service
}

func NewHandler(s Service) *Handler {
	return &Handler{service: s}
}

type RqRecordEvent struct {
	PractitionerID uuid.UUID              `json:"practitioner_id" binding:"required"`
	AccountantID   uuid.UUID              `json:"accountant_id" binding:"required"`
	ActorID        uuid.UUID              `json:"actor_id" binding:"required"`
	ActorName      string                 `json:"actor_name"`
	ActorType      string                 `json:"actor_type" binding:"required"`
	EventType      string                 `json:"event_type" binding:"required"`
	EntityType     string                 `json:"entity_type" binding:"required"`
	EntityID       uuid.UUID              `json:"entity_id" binding:"required"`
	Description    string                 `json:"description" binding:"required"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func (h *Handler) RecordEvent(c *gin.Context) {
	var req RqRecordEvent
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Errorf("bind json: %w", err))
		return
	}

	event := SharedEvent{
		ID:             uuid.New(),
		PractitionerID: req.PractitionerID,
		AccountantID:   req.AccountantID,
		ActorID:        req.ActorID,
		ActorName:      &req.ActorName,
		ActorType:      req.ActorType,
		EventType:      req.EventType,
		EntityType:     req.EntityType,
		EntityID:       req.EntityID,
		Description:    req.Description,
		Metadata:       JSONBMap(req.Metadata),
	}

	if err := h.service.Record(c.Request.Context(), event); err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Errorf("record event: %w", err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": event})
}
