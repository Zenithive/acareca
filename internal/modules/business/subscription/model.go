package subscription

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusPastDue   Status = "PAST_DUE"
	StatusCancelled Status = "CANCELLED"
	StatusPaused    Status = "PAUSED"
	StatusExpired   Status = "EXPIRED"
	StatusInactive  Status = "INACTIVE"
)

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusUnpaid    PaymentStatus = "UNPAID"
	PaymentStatusActive    PaymentStatus = "ACTIVE"
	PaymentStatusExpired   PaymentStatus = "EXPIRED"
	PaymentStatusCancelled PaymentStatus = "CANCELLED"
)

type PractitionerSubscription struct {
	ID                   int            `db:"id"`
	PractitionerID       uuid.UUID      `db:"practitioner_id"`
	SubscriptionID       int            `db:"subscription_id"`
	StartDate            time.Time      `db:"start_date"`
	EndDate              time.Time      `db:"end_date"`
	Status               Status         `db:"status"`
	PaymentStatus        PaymentStatus  `db:"payment_status"`
	StripeSubscriptionID *string        `db:"stripe_subscription_id"`
	StripeInvoiceID      *string        `db:"stripe_invoice_id"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
	DeletedAt            *time.Time     `db:"deleted_at"`
}

type RqCreatePractitionerSubscription struct {
	SubscriptionID int            `json:"subscription_id" validate:"required,min=1"`
	StartDate      string         `json:"start_date" validate:"required"`
	EndDate        string         `json:"end_date" validate:"required"`
	Status         Status         `json:"status" validate:"required,oneof=ACTIVE PAST_DUE CANCELLED PAUSED EXPIRED"`
	PaymentStatus  *PaymentStatus `json:"payment_status" validate:"omitempty,oneof=PENDING UNPAID ACTIVE EXPIRED CANCELLED"`
}

type RqUpdatePractitionerSubscription struct {
	Status        *Status        `json:"status" validate:"omitempty,oneof=ACTIVE PAST_DUE CANCELLED PAUSED EXPIRED"`
	PaymentStatus *PaymentStatus `json:"payment_status" validate:"omitempty,oneof=PENDING UNPAID ACTIVE EXPIRED CANCELLED"`
}

type SubscriptionInfo struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type RsActiveSubscription struct {
	ID             int              `json:"id"`
	PractitionerID uuid.UUID        `json:"practitioner_id"`
	Subscription   SubscriptionInfo `json:"subscription"`
	StartDate      time.Time        `json:"start_date"`
	EndDate        time.Time        `json:"end_date"`
	Status         Status           `json:"status"`
	PaymentStatus  PaymentStatus    `json:"payment_status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type RsPractitionerSubscription struct {
	ID             int           `json:"id"`
	PractitionerID uuid.UUID     `json:"practitioner_id"`
	SubscriptionID int           `json:"subscription_id"`
	StartDate      time.Time     `json:"start_date"`
	EndDate        time.Time     `json:"end_date"`
	Status         Status        `json:"status"`
	PaymentStatus  PaymentStatus `json:"payment_status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (s *PractitionerSubscription) ToRs() *RsPractitionerSubscription {
	return &RsPractitionerSubscription{
		ID:             s.ID,
		PractitionerID: s.PractitionerID,
		SubscriptionID: s.SubscriptionID,
		StartDate:      s.StartDate,
		EndDate:        s.EndDate,
		Status:         s.Status,
		PaymentStatus:  s.PaymentStatus,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

type WebhookUpsert struct {
	PractitionerID       uuid.UUID
	SubscriptionID       int
	StripeSubscriptionID string
	StripeInvoiceID      *string
	Status               Status
	PaymentStatus        PaymentStatus
	StartDate            time.Time
	EndDate              time.Time
}

type Filter struct {
	PractitionerID *uuid.UUID `form:"practitioner_id"`
	SubscriptionID *int       `form:"subscription_id"`
	Status         *Status    `form:"status"`
	FromDate       *time.Time `form:"from_date"`
	ToDate         *time.Time `form:"to_date"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}

	if filter.PractitionerID != nil {
		filters["ps.practitioner_id"] = *filter.PractitionerID
	}
	if filter.SubscriptionID != nil {
		filters["ps.subscription_id"] = *filter.SubscriptionID
	}
	if filter.Status != nil {
		filters["ps.status"] = string(*filter.Status)
	}
	if filter.FromDate != nil {
		filters["ps.created_at_gte"] = *filter.FromDate
	}
	if filter.ToDate != nil {
		filters["ps.created_at_lte"] = *filter.ToDate
	}

	return common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}
