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
	ID          uuid.UUID  `db:"id"           json:"id"`
	RecipientID uuid.UUID  `db:"recipient_id" json:"recipient_id"`
	SenderID    *uuid.UUID `db:"sender_id"    json:"sender_id,omitempty"`
	EventType   EventType  `db:"event_type"   json:"event_type"`
	EntityType  EntityType `db:"entity_type"  json:"entity_type"`
	EntityID    uuid.UUID  `db:"entity_id"    json:"entity_id"`
	Status      Status     `db:"status"       json:"status"`
	Payload     []byte     `db:"payload"      json:"payload"`
	RetryCount  int        `db:"retry_count"  json:"retry_count"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	ReadAt      *time.Time `db:"read_at"      json:"read_at,omitempty"`
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
	UserID    uuid.UUID `db:"user_id"`
	EventType EventType `db:"event_type"`
	// JSON array: ["in_app","email","push"]
	Channels []byte `db:"channels"`
}

type InviteQueue struct {
	ID             uuid.UUID  `db:"id"              json:"id"`
	PractitionerID uuid.UUID  `db:"practitioner_id" json:"practitioner_id"`
	InviteEmail    string     `db:"invite_email"    json:"invite_email"`
	Token          string     `db:"token"           json:"token"`
	Status         string     `db:"status"          json:"status"` // pending | accepted | expired
	ExpiresAt      time.Time  `db:"expires_at"      json:"expires_at"`
	CreatedAt      time.Time  `db:"created_at"      json:"created_at"`
	AcceptedAt     *time.Time `db:"accepted_at"     json:"accepted_at,omitempty"`
}

// ─── Payload helpers (stored as jsonb) ───────────────────────────────────────

type NotificationPayload struct {
	Title      string         `json:"title"`
	Body       string         `json:"body"`
	SenderName string         `json:"sender_name,omitempty"`
	EntityName string         `json:"entity_name,omitempty"`
	ExtraData  map[string]any `json:"extra_data,omitempty"`
}

// ─── Request / Response types ─────────────────────────────────────────────────

type SendInviteRequest struct {
	Email string `json:"email" binding:"required,email"`
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

type ListNotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	Total         int            `json:"total"`
}

type UpdatePreferenceRequest struct {
	EventType EventType `json:"event_type" binding:"required"`
	Channels  []Channel `json:"channels"   binding:"required"`
}

type PublishEventRequest struct {
	EventType  EventType           `json:"event_type"`
	ActorID    uuid.UUID           `json:"actor_id"`
	EntityType EntityType          `json:"entity_type"`
	EntityID   uuid.UUID           `json:"entity_id"`
	Payload    NotificationPayload `json:"payload"`
}
