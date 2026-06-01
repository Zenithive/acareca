package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
)

type NotificationMeta struct {
	EntityID      uuid.UUID
	EntityKey     string // "invite_id", "task_id", "doc_id" …
	Title         string
	Body          string
	SenderName    *string
	EventType     notification.EventType
	EntityType    notification.EntityType
	RecipientType notification.ActorType
}

// RecipientWithPreferences holds recipient info and their notification preferences
type RecipientWithPreferences struct {
	RecipientID   uuid.UUID
	RecipientType notification.ActorType
	UserID        uuid.UUID
}

// PublishNotification sends notifications to multiple recipients based on their preferences
// It fetches each recipient's preferences and filters channels accordingly
func PublishNotification(
	ctx context.Context,
	notificationSvc notification.Service,
	recipients []RecipientWithPreferences,
	senderID uuid.UUID,
	senderType notification.ActorType,
	senderName string,
	eventType notification.EventType,
	entityType notification.EntityType,
	entityID uuid.UUID,
	entityKey string,
	title string,
	body string,
) {
	if notificationSvc == nil {
		log.Println("[WARN] notification service is nil, skipping notifications")
		return
	}

	// Map EventType to NotificationEventType for preference lookup
	notificationEventType := notification.MapEventTypeToNotificationEventType(eventType)

	for _, recipient := range recipients {
		// Get user preferences
		prefs, err := notificationSvc.GetPreferences(ctx, recipient.UserID)
		if err != nil {
			log.Printf("[ERROR] failed to get preferences for user %s: %v", recipient.UserID, err)
			continue
		}

		// Find channels enabled for this event type
		channels := []notification.Channel{}
		for _, pref := range prefs {
			if pref.EventType == notificationEventType {
				// Check each channel and add if enabled
				for ch, isEnabled := range pref.Channels {
					if isEnabled {
						channels = append(channels, notification.Channel(ch))
					}
				}
				break
			}
		}

		// Skip if no channels are enabled
		if len(channels) == 0 {
			log.Printf("[INFO] no enabled channels for user %s, event %s", recipient.UserID, eventType)
			continue
		}

		// Build payload with sender name
		extraData := map[string]interface{}{
			entityKey: entityID.String(),
		}
		payload := notification.BuildNotificationPayload(
			title,
			json.RawMessage(fmt.Sprintf(`"%s"`, body)),
			&senderName,
			nil,
			&extraData,
		)
		payloadBytes, _ := json.Marshal(payload)

		// Create notification request
		rq := notification.RqNotification{
			ID:            uuid.New(),
			RecipientID:   recipient.RecipientID,
			RecipientType: recipient.RecipientType,
			SenderID:      &senderID,
			SenderType:    &senderType,
			EventType:     eventType,
			EntityType:    entityType,
			EntityID:      entityID,
			Status:        notification.StatusUnread,
			Payload:       payloadBytes,
			Channels:      channels,
			CreatedAt:     time.Now(),
		}

		// Publish notification
		if err := notificationSvc.Publish(ctx, rq); err != nil {
			log.Printf("[ERROR] failed to publish %s notification to %s: %v", eventType, recipient.RecipientID, err)
		} else {
			log.Printf("[INFO] published %s notification to %s via channels: %v", eventType, recipient.RecipientID, channels)
		}
	}
}
