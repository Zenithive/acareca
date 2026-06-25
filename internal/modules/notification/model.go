package notification

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
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

type NotificationEventType string

const (
	EventNewTransaction          NotificationEventType = "new.transaction"
	EventAccountantActivityAlert NotificationEventType = "accountant.activity.alert"
	EventSystemActivityAlert     NotificationEventType = "system.activity.alert"
)

func MapEventTypeToNotificationEventType(eventType EventType) NotificationEventType {
	switch eventType {
	case EventTransactionCreated, EventTransactionUpdated:
		return EventNewTransaction
	case EventClinicUpdated, EventFormSubmitted, EventFormUpdated, EventDocumentUploaded,
		EventInviteSent, EventInviteAccepted, EventInviteDeclined:
		return EventAccountantActivityAlert
	case EventAuditLogCreated, EventSystemError, EventSystemWarning:
		return EventSystemActivityAlert
	default:
		return EventSystemActivityAlert
	}
}

type NotificationPreference struct {
	ID         uuid.UUID             `db:"id" json:"id"`
	UserID     uuid.UUID             `db:"user_id" json:"user_id"`
	EntityID   uuid.UUID             `db:"entity_id" json:"entity_id"`
	EntityType string                `db:"entity_type" json:"entity_type"`
	EventType  NotificationEventType `db:"event_type" json:"event_type"`
	Channels   NotificationChannels  `db:"channels" json:"channels"`
	CreatedAt  time.Time             `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time             `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time            `db:"deleted_at" json:"-"`
}

type RqUpdatePreference struct {
	EventType NotificationEventType `json:"event_type" validate:"required"`
	Channels  NotificationChannels  `json:"channels"   validate:"required"`
}

type NotificationChannels map[string]bool

func (nc NotificationChannels) Value() (driver.Value, error) {
	if nc == nil {
		return nil, nil
	}
	return json.Marshal(nc)
}

func (nc *NotificationChannels) Scan(value interface{}) error {
	if value == nil {
		*nc = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("cannot scan %T into NotificationChannels", value)
	}

	return json.Unmarshal(bytes, nc)
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
