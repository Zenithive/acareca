package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type RecipientWithPreferences struct {
	RecipientID   uuid.UUID
	RecipientType util.ActorType
	UserID        uuid.UUID
}

type NotificationService interface {
	Publish(ctx context.Context, rq NotificationRequest) error
	GetPreferences(ctx context.Context, userID uuid.UUID) (map[util.NotificationEventType][]util.Channel, error)
}

type NotificationRequest struct {
	ID            uuid.UUID
	RecipientID   uuid.UUID
	RecipientType util.ActorType
	SenderID      *uuid.UUID
	SenderType    *util.ActorType
	EventType     util.EventType
	EntityType    util.EntityType
	EntityID      uuid.UUID
	Status        util.Status
	Payload       []byte
	Channels      []util.Channel
	CreatedAt     time.Time
}

type NotificationPayload struct {
	Title      string                  `json:"title"`
	Body       json.RawMessage         `json:"body"`
	SenderName *string                 `json:"sender_name,omitempty"`
	Metadata   *map[string]interface{} `json:"metadata,omitempty"`
	ExtraData  *map[string]interface{} `json:"extra_data,omitempty"`
}

type PublishRequest struct {
	Recipients []RecipientWithPreferences
	SenderID   uuid.UUID
	SenderType util.ActorType
	SenderName string
	EventType  util.EventType
	EntityType util.EntityType
	EntityID   uuid.UUID
	EntityKey  string
	Title      string
	Body       string
	ExtraData  map[string]interface{}
}

type Publisher struct {
	notificationSvc NotificationService
}

func NewPublisher(notificationSvc NotificationService, adminRepo admin.Repository) *Publisher {
	return &Publisher{notificationSvc: notificationSvc}
}

func (p *Publisher) Publish(ctx context.Context, req PublishRequest) error {
	if p.notificationSvc == nil {
		return fmt.Errorf("notification service is nil")
	}

	for _, recipient := range req.Recipients {
		// Get user preferences as a map: eventType -> channels
		prefMap, err := p.notificationSvc.GetPreferences(ctx, recipient.UserID)
		if err != nil {
			log.Printf("[ERROR] failed to get preferences for user %s: %v", recipient.UserID, err)
			continue
		}

		// Check if user has enabled this event type
		channels, ok := prefMap[util.NotificationEventType(req.EventType)]
		if !ok {
			// Try mapped event types
			mappedTypes := util.MapEventTypeToNotificationEventType(req.EventType)
			for _, mappedType := range mappedTypes {
				if ch, found := prefMap[mappedType]; found {
					channels = ch
					ok = true
					break
				}
			}
		}

		// Skip if user hasn't enabled this event type or has no channels
		if !ok || len(channels) == 0 {
			continue
		}

		// Build and publish notification
		extraData := map[string]interface{}{req.EntityKey: req.EntityID.String()}
		if req.ExtraData != nil {
			maps.Copy(extraData, req.ExtraData)
		}

		payload := NotificationPayload{
			Title:      req.Title,
			Body:       json.RawMessage(fmt.Sprintf(`"%s"`, req.Body)),
			SenderName: &req.SenderName,
			ExtraData:  &extraData,
		}
		payloadBytes, _ := json.Marshal(payload)

		notifReq := NotificationRequest{
			ID:            uuid.New(),
			RecipientID:   recipient.RecipientID,
			RecipientType: recipient.RecipientType,
			SenderID:      &req.SenderID,
			SenderType:    &req.SenderType,
			EventType:     req.EventType,
			EntityType:    req.EntityType,
			EntityID:      req.EntityID,
			Status:        util.StatusUnread,
			Payload:       payloadBytes,
			Channels:      channels,
			CreatedAt:     time.Now(),
		}

		if err := p.notificationSvc.Publish(ctx, notifReq); err != nil {
			log.Printf("[ERROR] failed to publish %s notification to %s: %v", req.EventType, recipient.RecipientID, err)
		} else {
			log.Printf("[INFO] published %s notification to %s via channels: %v", req.EventType, recipient.RecipientID, channels)
		}
	}
	return nil
}
