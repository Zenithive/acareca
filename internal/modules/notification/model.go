package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Enums

// Status is the user-facing state of a notification.
type Status string

const (
	StatusUnread    Status = "UNREAD"
	StatusRead      Status = "READ"
	StatusDismissed Status = "DISMISSED"
)

// DeliveryStatus tracks per-channel delivery state.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "PENDING"
	DeliveryDelivered DeliveryStatus = "DELIVERED"
	DeliveryFailed    DeliveryStatus = "FAILED"
)

type EventType string

const (
	// Practitioner → Account
	EventInviteSent     EventType = "invite.sent"
	EventInviteAccepted EventType = "invite.accepted"
	EventInviteDeclined EventType = "invite.declined"

	// Account → Practitioner
	EventClinicUpdated      EventType = "clinic.updated"
	EventFormSubmitted      EventType = "form.submitted"
	EventFormUpdated        EventType = "form.updated"
	EventTransactionCreated EventType = "transaction.created"
	EventTransactionUpdated EventType = "transaction.status_changed"
	EventDocumentUploaded   EventType = "document.uploaded"

	// System → Admin
	EventAuditLogCreated EventType = "audit_log.created"
	EventSystemError     EventType = "system.error"
	EventSystemWarning   EventType = "system.warning"
)

type EntityType string

const (
	EntityClinic      EntityType = "clinic"
	EntityForm        EntityType = "form"
	EntityTransaction EntityType = "transaction"
	EntityDocument    EntityType = "document"
	EntityInvite      EntityType = "invite"
	EntityAuditLog    EntityType = "audit_log"
	EntitySystem      EntityType = "system"
)

type Channel string

const (
	ChannelInApp Channel = "in_app"
	ChannelPush  Channel = "push"
	ChannelEmail Channel = "email"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelInApp, ChannelPush, ChannelEmail:
		return true
	default:
		return false
	}
}

type ChannelMap map[string]bool

type ActorType string

const (
	ActorPractitioner ActorType = "PRACTITIONER"
	ActorAccountant   ActorType = "ACCOUNTANT"
	ActorAdmin        ActorType = "ADMIN"
	ActorSystem       ActorType = "SYSTEM"
)

type RqNotification struct {
	ID            uuid.UUID       `json:"id"`
	RecipientID   uuid.UUID       `json:"recipient_id"`
	RecipientType ActorType       `json:"recipient_type"`
	SenderID      *uuid.UUID      `json:"sender_id"`
	SenderType    *ActorType      `json:"sender_type"`
	EventType     EventType       `json:"event_type"`
	EntityType    EntityType      `json:"entity_type"`
	EntityID      uuid.UUID       `json:"entity_id"`
	Status        Status          `json:"status"`
	Payload       json.RawMessage `json:"payload"`
	Channels      []Channel       `json:"channels"`
	CreatedAt     time.Time       `json:"created_at"`
	ReadedAt      *time.Time      `json:"readed_at"`
}

type Notification struct {
	ID            uuid.UUID       `db:"id"`
	RecipientID   uuid.UUID       `db:"recipient_id"`
	RecipientType ActorType       `db:"recipient_type"`
	SenderID      *uuid.UUID      `db:"sender_id"`
	SenderType    *ActorType      `db:"sender_type"`
	EventType     EventType       `db:"event_type"`
	EntityType    EntityType      `db:"entity_type"`
	EntityID      uuid.UUID       `db:"entity_id"`
	Status        Status          `db:"status"`
	Payload       json.RawMessage `db:"payload" swaggertype:"object"`
	CreatedAt     time.Time       `db:"created_at"`
	ReadedAt      *time.Time      `db:"readed_at"`
}

// Delivery tracks the send state for a single channel of a notification.
type Delivery struct {
	ID             uuid.UUID      `db:"id"`
	NotificationID uuid.UUID      `db:"notification_id"`
	Channel        Channel        `db:"channel"`
	Status         DeliveryStatus `db:"status"`
	RetryCount     int            `db:"retry_count"`
	LastAttemptAt  *time.Time     `db:"last_attempted_at"`
	DeliveredAt    *time.Time     `db:"delivered_at"`
	ErrorMessage   *string        `db:"error_message"`
}

// FailedDelivery is used by the retry worker — joins delivery + notification data.
type FailedDelivery struct {
	NotificationID uuid.UUID       `db:"notification_id"`
	RecipientID    uuid.UUID       `db:"recipient_id"`
	RetryCount     int             `db:"retry_count"`
	EventType      EventType       `db:"event_type"`
	EntityType     EntityType      `db:"entity_type"`
	EntityID       uuid.UUID       `db:"entity_id"`
	Payload        json.RawMessage `db:"payload"`
	CreatedAt      time.Time       `db:"created_at"`
}

func (n *RqNotification) MapToDB() Notification {
	return Notification{
		ID:            n.ID,
		RecipientID:   n.RecipientID,
		RecipientType: n.RecipientType,
		SenderID:      n.SenderID,
		SenderType:    n.SenderType,
		EventType:     n.EventType,
		EntityType:    n.EntityType,
		EntityID:      n.EntityID,
		Status:        StatusUnread,
		Payload:       n.Payload,
		CreatedAt:     n.CreatedAt,
		ReadedAt:      n.ReadedAt,
	}
}

type Preference struct {
	EntityID  uuid.UUID `db:"entity_id"`
	EventType EventType `db:"event_type"`
	// JSON array: ["in_app","email","push"]
	Channels []byte `db:"channels"`
}

// ─── Payload helpers (stored as jsonb) ───────────────────────────────────────

type NotificationPayload struct {
	Title      string                  `json:"title"`
	Body       json.RawMessage         `json:"body"`
	Channel    *Channel                `json:"channel,omitempty"`
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

// --- NOTIFICATION PREFERENCES ---

type NotificationEventType string

const (
	EventNewTransaction          NotificationEventType = "new.transaction"
	EventAccountantActivityAlert NotificationEventType = "accountant.activity.alert"
	EventSystemActivityAlert     NotificationEventType = "system.activity.alert"
)

type NotificationEventTypes []NotificationEventType

type NotificationPreference struct {
	ID         uuid.UUID              `db:"id" json:"id"`
	UserID     uuid.UUID              `db:"user_id" json:"user_id"`
	EntityID   uuid.UUID              `db:"entity_id" json:"entity_id"`
	EntityType string                 `db:"entity_type" json:"entity_type"`
	EventType  NotificationEventTypes `db:"event_type" json:"event_type"`
	Channels   NotificationChannels   `db:"channels" json:"channels"`
	CreatedAt  time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time              `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time             `db:"deleted_at" json:"-"`
}

type RqUpdatePreference struct {
	EventType NotificationEventTypes `json:"event_type" validate:"required"`
	Channels  NotificationChannels   `json:"channels"   validate:"required"`
}

// Define a custom type for the channels map
type NotificationChannels map[string]bool

type RqBulkDismiss struct {
	IDs []uuid.UUID `json:"ids" validate:"required,min=1"`
}

const (
	// Subjects
	SubjectNotificationInApp = "notification.in_app"
	SubjectNotificationEmail = "notification.email"
	SubjectNotificationPush  = "notification.push"

	// JetStream
	StreamNotification = "NOTIFICATION_STREAM"

	ConsumerNotificationInApp = "notification.in_app.consumer"
	ConsumerNotificationEmail = "notification.email.consumer"
	ConsumerNotificationPush  = "notification.push.consumer"
)

// NotificationEvent represents the event payload published to NATS
type NotificationEvent struct {
	ID            uuid.UUID       `json:"id"`
	RecipientID   uuid.UUID       `json:"recipient_id"`
	RecipientType ActorType       `json:"recipient_type"`
	SenderID      *uuid.UUID      `json:"sender_id"`
	SenderType    *ActorType      `json:"sender_type"`
	EventType     EventType       `json:"event_type"`
	EntityType    EntityType      `json:"entity_type"`
	EntityID      uuid.UUID       `json:"entity_id"`
	Payload       json.RawMessage `json:"payload"`
	Channels      []Channel       `json:"channels"`
	CreatedAt     time.Time       `json:"created_at"`
}
