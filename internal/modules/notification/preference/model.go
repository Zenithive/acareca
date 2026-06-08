package preference

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Preference struct {
	ID         uuid.UUID                    `db:"id" json:"id"`
	UserID     uuid.UUID                    `db:"user_id" json:"user_id"`
	EntityID   uuid.UUID                    `db:"entity_id" json:"entity_id"`
	EntityType string                       `db:"entity_type" json:"entity_type"`
	EventType  []util.NotificationEventType `db:"event_type" json:"event_type"`
	Channels   []util.Channel               `db:"channels" json:"channels"`
	CreatedAt  time.Time                    `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time                    `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time                   `db:"deleted_at" json:"-"`
}

type RqUpdatePreference struct {
	EventType []util.NotificationEventType `json:"event_type"`
	Channels  []util.Channel               `json:"channels" validate:"required"`
}
