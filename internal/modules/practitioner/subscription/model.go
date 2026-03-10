package subscription

import "time"

// Status matches practitioner_subscription_status enum.
type Status string

const (
	StatusActive    Status = "active"
	StatusPastDue   Status = "past_due"
	StatusCancelled Status = "cancelled"
	StatusPaused    Status = "paused"
	StatusExpired   Status = "expired"
)

// TentantSubscription matches tbl_practitioner_subscription.
type TentantSubscription struct {
	ID             int        `db:"id"`
	TentantID      int        `db:"tentant_id"`
	SubscriptionID int        `db:"subscription_id"`
	StartDate      time.Time  `db:"start_date"`
	EndDate        time.Time  `db:"end_date"`
	Status         Status     `db:"status"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// RqCreateTentantSubscription request to create a practitioner subscription.
type RqCreateTentantSubscription struct {
	SubscriptionID int     `json:"subscription_id" validate:"required,min=1"`
	StartDate      string  `json:"start_date" validate:"required"` // RFC3339
	EndDate        string  `json:"end_date" validate:"required"`
	Status         *Status `json:"status" validate:"omitempty,oneof=active past_due cancelled paused expired"`
}

// RqUpdateTentantSubscription request to update (e.g. status).
type RqUpdateTentantSubscription struct {
	Status *Status `json:"status" validate:"omitempty,oneof=active past_due cancelled paused expired"`
}

// RsTentantSubscription response.
type RsTentantSubscription struct {
	ID             int       `json:"id"`
	TentantID      int       `json:"tentant_id"`
	SubscriptionID int       `json:"subscription_id"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *TentantSubscription) ToRs() *RsTentantSubscription {
	return &RsTentantSubscription{
		ID:             s.ID,
		TentantID:      s.TentantID,
		SubscriptionID: s.SubscriptionID,
		StartDate:      s.StartDate,
		EndDate:        s.EndDate,
		Status:         s.Status,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}
