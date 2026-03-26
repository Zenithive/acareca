package audit

import (
	"encoding/json"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/common"
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

func toRsAuditLog(a *AuditLog) *RsAuditLog {
	return &RsAuditLog{
		ID:          a.ID,
		PracticeID:  a.PracticeID,
		UserID:      a.UserID,
		Action:      a.Action,
		Module:      a.Module,
		EntityType:  a.EntityType,
		EntityID:    a.EntityID,
		BeforeState: a.BeforeState,
		AfterState:  a.AfterState,
		IPAddress:   a.IPAddress,
		UserAgent:   a.UserAgent,
		CreatedAt:   a.CreatedAt,
	}
}

// RsAuditLog is the swagger response type for AuditLog.
// Uses interface{} for JSON fields since swag cannot resolve json.RawMessage.
type RsAuditLog struct {
	ID          string      `json:"id"`
	PracticeID  *string     `json:"practice_id,omitempty"`
	UserID      *string     `json:"user_id,omitempty"`
	Action      string      `json:"action"`
	Module      string      `json:"module"`
	EntityType  *string     `json:"entity_type,omitempty"`
	EntityID    *string     `json:"entity_id,omitempty"`
	BeforeState interface{} `json:"before_state,omitempty"`
	AfterState  interface{} `json:"after_state,omitempty"`
	IPAddress   *string     `json:"ip_address,omitempty"`
	UserAgent   *string     `json:"user_agent,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
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

type Filter struct {
	PracticeID *string    `form:"practice_id"`
	UserID     *string    `form:"user_id"`
	Module     *string    `form:"module"`
	Action     *string    `form:"action"`
	EntityType *string    `form:"entity_type"`
	EntityID   *string    `form:"entity_id"`
	StartDate  *time.Time `form:"start_date" time_format:"2006-01-02T15:04:05Z07:00"`
	EndDate    *time.Time `form:"end_date" time_format:"2006-01-02T15:04:05Z07:00"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.PracticeID != nil {
		filters["practice_id"] = *filter.PracticeID
	}
	if filter.UserID != nil {
		filters["user_id"] = *filter.UserID
	}
	if filter.Module != nil {
		filters["module"] = *filter.Module
	}
	if filter.Action != nil {
		filters["action"] = *filter.Action
	}
	if filter.EntityType != nil {
		filters["entity_type"] = *filter.EntityType
	}
	if filter.EntityID != nil {
		filters["entity_id"] = *filter.EntityID
	}
	if filter.StartDate != nil {
		filters["created_at_gte"] = *filter.StartDate
	}
	if filter.EndDate != nil {
		filters["created_at_lte"] = *filter.EndDate
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset)

	return f
}

func (a *AuditLog) ToRs() *RsAuditLog {
	return &RsAuditLog{
		ID:          a.ID,
		PracticeID:  a.PracticeID,
		UserID:      a.UserID,
		Action:      a.Action,
		Module:      a.Module,
		EntityType:  a.EntityType,
		EntityID:    a.EntityID,
		BeforeState: a.BeforeState,
		AfterState:  a.AfterState,
		IPAddress:   a.IPAddress,
		UserAgent:   a.UserAgent,
		CreatedAt:   a.CreatedAt,
	}
}
