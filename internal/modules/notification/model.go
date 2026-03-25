package notification

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// Enums

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusDelivered Status = "DELIVERED"
	StatusRead      Status = "READ"
	StatusDismissed Status = "DISMISSED"
	StatusFailed    Status = "FAILED"
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

// ─── Domain structs ───────────────────────────────────────────────────────────

type Notification struct {
	ID          uuid.UUID  `db:"id"`
	RecipientID uuid.UUID  `db:"recipient_id"`
	SenderID    *uuid.UUID `db:"sender_id"`
	EventType   EventType  `db:"event_type"`
	EntityType  EntityType `db:"entity_type"`
	EntityID    uuid.UUID  `db:"entity_id"`
	Status      Status     `db:"status"`
	Payload     []byte     `db:"payload"`
	RetryCount  int        `db:"retry_count"`
	CreatedAt   time.Time  `db:"created_at"`
	ReadedAt    *time.Time `db:"readed_at"`
}

type OutboxEvent struct {
	ID          uuid.UUID  `db:"id"`
	EventType   EventType  `db:"event_type"`
	ActorID     uuid.UUID  `db:"actor_id"`
	EntityType  EntityType `db:"entity_type"`
	EntityID    uuid.UUID  `db:"entity_id"`
	Payload     []byte     `db:"payload"`
	ProcessedAt *time.Time `db:"processed_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

type Preference struct {
	EntityID  uuid.UUID `db:"entity_id"`
	EventType EventType `db:"event_type"`
	// JSON array: ["in_app","email","push"]
	Channels []byte `db:"channels"`
}

// ─── Payload helpers (stored as jsonb) ───────────────────────────────────────

type NotificationPayload struct {
	Title      string                 `json:"title"`
	Body       string                 `json:"body"`
	SenderName string                 `json:"sender_name,omitempty"`
	EntityName string                 `json:"entity_name,omitempty"`
	ExtraData  map[string]interface{} `json:"extra_data,omitempty"`
}

type Filter struct {
	common.QueryFilter
	Status     *Status     `form:"status"`
	EventType  *EventType  `form:"event_type"`
	EntityType *EntityType `form:"entity_type"`
}

func (f *Filter) MapToFilter() common.Filter {
	fields := map[string]interface{}{}
	if f.Status != nil {
		fields["status"] = string(*f.Status)
	}
	if f.EventType != nil {
		fields["event_type"] = string(*f.EventType)
	}
	if f.EntityType != nil {
		fields["entity_type"] = string(*f.EntityType)
	}
	return common.ParseQueryFilter(f.QueryFilter, fields, nil, "created_at")
}

type RsListNotifications struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	Total         int            `json:"total"`
}

type RqUpdatePreference struct {
	EventType EventType `json:"event_type" binding:"required"`
	Channels  []Channel `json:"channels"   binding:"required"`
}

type RqPublishEvent struct {
	EventType  EventType           `json:"event_type"`
	ActorID    uuid.UUID           `json:"actor_id"`
	EntityType EntityType          `json:"entity_type"`
	EntityID   uuid.UUID           `json:"entity_id"`
	Payload    NotificationPayload `json:"payload"`
}
