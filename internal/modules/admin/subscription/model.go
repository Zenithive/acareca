package subscription

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type Subscription struct {
	ID           int        `db:"id"`
	Name         string     `db:"name"`
	Description  *string    `db:"description"`
	Price        float64    `db:"price"`
	DurationDays int        `db:"duration_days"`
	IsActive     bool       `db:"is_active"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type RqCreateSubscription struct {
	Name         string  `json:"name" validate:"required,max=255"`
	Description  *string `json:"description"`
	Price        float64 `json:"price" validate:"min=0"`
	DurationDays int     `json:"duration_days" validate:"required,min=1"`
	IsActive     *bool   `json:"is_active"`
}

func (r *RqCreateSubscription) ToSubscription() *Subscription {
	active := true
	if r.IsActive != nil {
		active = *r.IsActive
	}
	return &Subscription{
		Name:         r.Name,
		Description:  r.Description,
		Price:        r.Price,
		DurationDays: r.DurationDays,
		IsActive:     active,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

type RqUpdateSubscription struct {
	Name         *string  `json:"name" validate:"omitempty,max=255"`
	Description  *string  `json:"description"`
	Price        *float64 `json:"price" validate:"omitempty,min=0"`
	DurationDays *int     `json:"duration_days" validate:"omitempty,min=1"`
	IsActive     *bool    `json:"is_active"`
}

type RsSubscription struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	Price        float64   `json:"price"`
	DurationDays int       `json:"duration_days"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Subscription) ToRs() *RsSubscription {
	return &RsSubscription{
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Price:        s.Price,
		DurationDays: s.DurationDays,
		IsActive:     s.IsActive,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// --- Plan Permission models ---

// PlanPermission represents a single permission key (e.g. "clinic.create").
type PlanPermission struct {
	ID  int    `db:"id"`
	Key string `db:"key"`
}

// SubscriptionPermission is the join between a plan and a permission key with its limit.
type SubscriptionPermission struct {
	ID             int    `db:"id"`
	SubscriptionID int    `db:"subscription_id"`
	PermissionID   int    `db:"permission_id"`
	Key            string `db:"key"`
	IsEnabled      bool   `db:"is_enabled"`
	UsageLimit     int    `db:"usage_limit"`
}

// RqUpdatePermission is the request body for updating a single permission limit on a plan.
type RqUpdatePermission struct {
	UsageLimit *int  `json:"usage_limit"` // -1 = unlimited, 0 = blocked, >0 = capped
	IsEnabled  *bool `json:"is_enabled"`
}

// RsSubscriptionPermission is the response for a single permission entry.
type RsSubscriptionPermission struct {
	Key        string `json:"key"`
	IsEnabled  bool   `json:"is_enabled"`
	UsageLimit int    `json:"usage_limit"`
}

func (p *SubscriptionPermission) ToRs() *RsSubscriptionPermission {
	return &RsSubscriptionPermission{
		Key:        p.Key,
		IsEnabled:  p.IsEnabled,
		UsageLimit: p.UsageLimit,
	}
}

type Filter struct {
	Id   *string `form:"id"`
	Name *string `form:"name"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.Id != nil {
		id, err := uuid.Parse(*filter.Id)
		if err != nil {
			fmt.Println("invalid subscription_id: %w", err)
		}
		filters["id"] = uuid.UUID(id)
	}
	if filter.Name != nil {
		filters["name"] = *filter.Name
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset)

	return f
}
