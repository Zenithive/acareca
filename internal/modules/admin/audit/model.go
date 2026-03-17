package audit

import (
	"encoding/json"
	"time"
)

type AuditLog struct {
	ID          string           `db:"id" json:"id"`
	PracticeID  *string          `db:"practice_id" json:"practice_id,omitempty"`
	UserID      *string          `db:"user_id" json:"user_id,omitempty"`
	Action      string           `db:"action" json:"action"`
	Module      string           `db:"module" json:"module"`
	EntityType  *string          `db:"entity_type" json:"entity_type,omitempty"`
	EntityID    *string          `db:"entity_id" json:"entity_id,omitempty"`
	BeforeState *json.RawMessage `db:"before_state" json:"before_state,omitempty"`
	AfterState  *json.RawMessage `db:"after_state" json:"after_state,omitempty"`
	IPAddress   *string          `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent   *string          `db:"user_agent" json:"user_agent,omitempty"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at"`
}

// LogEntry is used to create new audit log entries
type LogEntry struct {
	PracticeID  *string
	UserID      *string
	Action      string
	Module      string
	EntityType  *string
	EntityID    *string
	BeforeState interface{}
	AfterState  interface{}
	IPAddress   *string
	UserAgent   *string
}

// QueryParams for filtering audit logs
type QueryParams struct {
	PracticeID *string
	UserID     *string
	Module     *string
	Action     *string
	EntityType *string
	EntityID   *string
	StartDate  *time.Time
	EndDate    *time.Time
	Limit      int
	Offset     int
}
