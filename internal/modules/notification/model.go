package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type RqNotification struct {
	ID            uuid.UUID       `json:"id"`
	RecipientID   uuid.UUID       `json:"recipient_id"`
	RecipientType util.ActorType  `json:"recipient_type"`
	SenderID      *uuid.UUID      `json:"sender_id"`
	SenderType    *util.ActorType `json:"sender_type"`
	EventType     util.EventType  `json:"event_type"`
	EntityType    util.EntityType `json:"entity_type"`
	EntityID      uuid.UUID       `json:"entity_id"`
	Status        util.Status     `json:"status"`
	Payload       json.RawMessage `json:"payload"`
	Channels      []util.Channel  `json:"channels"`
	CreatedAt     time.Time       `json:"created_at"`
	ReadedAt      *time.Time      `json:"readed_at"`
}

type Notification struct {
	ID            uuid.UUID       `db:"id"`
	RecipientID   uuid.UUID       `db:"recipient_id"`
	RecipientType util.ActorType  `db:"recipient_type"`
	SenderID      *uuid.UUID      `db:"sender_id"`
	SenderType    *util.ActorType `db:"sender_type"`
	EventType     util.EventType  `db:"event_type"`
	EntityType    util.EntityType `db:"entity_type"`
	EntityID      uuid.UUID       `db:"entity_id"`
	Status        util.Status     `db:"status"`
	Payload       json.RawMessage `db:"payload" swaggertype:"object"`
	CreatedAt     time.Time       `db:"created_at"`
	ReadedAt      *time.Time      `db:"readed_at"`
}

type Delivery struct {
	ID             uuid.UUID           `db:"id"`
	NotificationID uuid.UUID           `db:"notification_id"`
	Channel        util.Channel        `db:"channel"`
	Status         util.DeliveryStatus `db:"status"`
	RetryCount     int                 `db:"retry_count"`
	LastAttemptAt  *time.Time          `db:"last_attempted_at"`
	DeliveredAt    *time.Time          `db:"delivered_at"`
	ErrorMessage   *string             `db:"error_message"`
}

type FailedDelivery struct {
	NotificationID uuid.UUID       `db:"notification_id"`
	RecipientID    uuid.UUID       `db:"recipient_id"`
	RecepientType  util.ActorType  `db:"recipient_type"`
	RetryCount     int             `db:"retry_count"`
	EventType      util.EventType  `db:"event_type"`
	EntityType     util.EntityType `db:"entity_type"`
	EntityID       uuid.UUID       `db:"entity_id"`
	Payload        json.RawMessage `db:"payload"`
	CreatedAt      time.Time       `db:"created_at"`
}

type NotificationPayload struct {
	Title      string                  `json:"title"`
	Body       json.RawMessage         `json:"body"`
	Channel    *util.Channel           `json:"channel,omitempty"`
	SenderName *string                 `json:"sender_name,omitempty"`
	EntityName *string                 `json:"entity_name,omitempty"`
	ExtraData  *map[string]interface{} `json:"extra_data,omitempty"`
}

type FilterNotification struct {
	Status *string `form:"status"`
	Search *string `form:"search"`
	Limit  *int    `form:"limit"`
	Offset *int    `form:"offset"`
}

func BuildNotificationPayload(title string, body json.RawMessage, senderName *string, entityName *string, extraData *map[string]interface{}) *NotificationPayload {
	return &NotificationPayload{
		Title:      title,
		Body:       body,
		SenderName: senderName,
		EntityName: entityName,
		ExtraData:  extraData,
	}
}

type RqBulkDismiss struct {
	IDs []uuid.UUID `json:"ids" validate:"required,min=1"`
}

// Event subjects
const (
	SubjectNotificationInApp = "notification.in_app"
	SubjectNotificationEmail = "notification.email"
	SubjectNotificationPush  = "notification.push"

	StreamNotification = "NOTIFICATION_STREAM"

	ConsumerNotificationInApp = "notification_in_app_consumer"
	ConsumerNotificationEmail = "notification_email_consumer"
	ConsumerNotificationPush  = "notification_push_consumer"
)

// NotificationEvent represents a notification event to be published
type NotificationEvent struct {
	ID            uuid.UUID       `json:"id"`
	RecipientID   uuid.UUID       `json:"recipient_id"`
	RecipientType util.ActorType  `json:"recipient_type"`
	SenderID      *uuid.UUID      `json:"sender_id"`
	SenderType    *util.ActorType `json:"sender_type"`
	EventType     util.EventType  `json:"event_type"`
	EntityType    util.EntityType `json:"entity_type"`
	EntityID      uuid.UUID       `json:"entity_id"`
	Payload       json.RawMessage `json:"payload"`
	Channels      []util.Channel  `json:"channels"`
	CreatedAt     time.Time       `json:"created_at"`
}
