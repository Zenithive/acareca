package subscription

import (
	"time"

	"github.com/google/uuid"
)

// Status matches practitioner_subscription_status enum.
type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusPastDue   Status = "PAST_DUE"
	StatusCancelled Status = "CANCELLED"
	StatusPaused    Status = "PAUSED"
	StatusExpired   Status = "EXPIRED"
)

// PractitionerSubscription matches tbl_practitioner_subscription.
type PractitionerSubscription struct {
	ID             int        `db:"id"`
	PractitionerID uuid.UUID  `db:"practitioner_id"`
	SubscriptionID int        `db:"subscription_id"`
	StartDate      time.Time  `db:"start_date"`
	EndDate        time.Time  `db:"end_date"`
	Status         Status     `db:"status"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// RqCreatePractitionerSubscription request to create a practitioner subscription.
type RqCreatePractitionerSubscription struct {
	SubscriptionID int    `json:"subscription_id" validate:"required,min=1"`
	StartDate      string `json:"start_date" validate:"required"` // RFC3339
	EndDate        string `json:"end_date" validate:"required"`
	Status         Status `json:"status" validate:"required,oneof=ACTIVE PAST_DUE CANCELLED PAUSED EXPIRED"`
}

// RqUpdatePractitionerSubscription request to update (e.g. status).
type RqUpdatePractitionerSubscription struct {
	Status *Status `json:"status" validate:"omitempty,oneof=ACTIVE PAST_DUE CANCELLED PAUSED EXPIRED"`
}

// RsPractitionerSubscription response.
type RsPractitionerSubscription struct {
	ID             int       `json:"id"`
	PractitionerID uuid.UUID `json:"practitioner_id"`
	SubscriptionID int       `json:"subscription_id"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *PractitionerSubscription) ToRs() *RsPractitionerSubscription {
	return &RsPractitionerSubscription{
		ID:             s.ID,
		PractitionerID: s.PractitionerID,
		SubscriptionID: s.SubscriptionID,
		StartDate:      s.StartDate,
		EndDate:        s.EndDate,
		Status:         s.Status,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}
