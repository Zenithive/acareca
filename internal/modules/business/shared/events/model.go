package events

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// SharedEvent represents a collaborative action between practitioner and accountant on shared entities.
type SharedEvent struct {
	ID             uuid.UUID `db:"id" json:"id"`
	PractitionerID uuid.UUID `db:"practitioner_id" json:"practitioner_id"`
	AccountantID   uuid.UUID `db:"accountant_id" json:"accountant_id"`
	ActorID        uuid.UUID `db:"actor_id" json:"actor_id"`
	ActorName      *string   `db:"actor_name" json:"actor_name"`
	ActorType      string    `db:"actor_type" json:"actor_type"`
	EventType      string    `db:"event_type" json:"event_type"`
	EntityType     string    `db:"entity_type" json:"entity_type"`
	EntityID       uuid.UUID `db:"entity_id" json:"entity_id"`
	Description    string    `db:"description" json:"description"`
	Metadata       JSONBMap  `db:"metadata" json:"metadata"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// JSONBMap for JSONB handling
type JSONBMap map[string]interface{}

// Value - Converts the Go map into a JSON string for Postgres
func (j JSONBMap) Value() (driver.Value, error) {
	if j == nil {
		return json.Marshal(map[string]interface{}{})
	}
	return json.Marshal(j)
}

// Scan - Converts the JSON string from Postgres back into a Go map
func (j *JSONBMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONBMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}
