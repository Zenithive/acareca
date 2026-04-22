package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
)

func PublishNotification[T any](
	ctx context.Context,
	notificationSvc notification.Service,
	recipientID *uuid.UUID,
	practitionerID uuid.UUID,
	data T,
	mapper func(T) NotificationMeta,
) {
	if notificationSvc == nil {
		return
	}

	if recipientID == nil || *recipientID == practitionerID {
		return
	}

	meta := mapper(data)
	extraData := map[string]interface{}{
		meta.EntityKey: meta.EntityID.String(),
	}

	payload := notification.BuildNotificationPayload(
		meta.Title,
		json.RawMessage(meta.Body),
		nil, nil,
		&extraData,
	)
	payloadBytes, _ := json.Marshal(payload)

	senderType := notification.ActorPractitioner
	rq := notification.RqNotification{
		ID:            uuid.New(),
		RecipientID:   *recipientID,
		RecipientType: meta.RecipientType,
		SenderID:      &practitionerID,
		SenderType:    &senderType,
		EventType:     meta.EventType,
		EntityType:    meta.EntityType,
		EntityID:      meta.EntityID,
		Status:        notification.StatusUnread,
		Payload:       payloadBytes,
		CreatedAt:     time.Now(),
	}

	if err := notificationSvc.Publish(ctx, rq); err != nil {
		fmt.Printf("[ERROR] failed to publish %s notification: %v\n", meta.EventType, err)
	}
}

type NotificationMeta struct {
	EntityID      uuid.UUID
	EntityKey     string // "invite_id", "task_id", "doc_id" …
	Title         string
	Body          string
	EventType     notification.EventType
	EntityType    notification.EntityType
	RecipientType notification.ActorType
}
