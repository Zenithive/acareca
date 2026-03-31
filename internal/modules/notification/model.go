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
)

type EntityType string

const (
	EntityClinic      EntityType = "clinic"
	EntityForm        EntityType = "form"
	EntityTransaction EntityType = "transaction"
	EntityDocument    EntityType = "document"
	EntityInvite      EntityType = "invite"
)

type Channel string

const (
	ChannelInApp Channel = "in_app"
	ChannelPush  Channel = "push"
	ChannelEmail Channel = "email"
)

type ActorType string

const (
	ActorPractitioner ActorType = "PRACTITIONER"
	ActorAccountant   ActorType = "ACCOUNTANT"
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
	SenderName *string                 `json:"sender_name,omitempty"`
	EntityName *string                 `json:"entity_name,omitempty"`
	ExtraData  *map[string]interface{} `json:"extra_data,omitempty"`
}

type FilterNotification struct {
	Status *string `form:"status"`
	Limit  int     `form:"limit"`
	Page   int     `form:"page"`
}

type RqUpdatePreference struct {
	EventType EventType `json:"event_type" binding:"required"`
	Channels  []Channel `json:"channels"   binding:"required"`
}

type RsListNotification struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	Total         int            `json:"total"`
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
