package practitioner

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type Practitioner struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	ABN              *string    `db:"abn"`
	Verified         bool       `db:"verified"`
	StripeCustomerID *string    `db:"stripe_customer_id"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`
}

// PractitionerWithUser is used for JOIN queries
type PractitionerWithUser struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	ABN              *string    `db:"abn"`
	Verified         bool       `db:"verified"`
	StripeCustomerID *string    `db:"stripe_customer_id"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`

	// user fields
	Email     string  `db:"email"`
	FirstName string  `db:"first_name"`
	LastName  string  `db:"last_name"`
	Phone     *string `db:"phone"`
}

type RqCreatePractitioner struct {
	UserID string `json:"user_id"`
}

type RsUserInfo struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Phone      *string   `json:"phone,omitempty"`
	JoinedDate time.Time `json:"joined_date"`
}

type RsPractitioner struct {
	ID       uuid.UUID   `json:"id"`
	ABN      *string     `json:"abn,omitempty"`
	Verified bool        `json:"verified"`
	User     *RsUserInfo `json:"user"`
}

func (p *Practitioner) ToRs() *RsPractitioner {
	return &RsPractitioner{
		ID:       p.ID,
		ABN:      p.ABN,
		Verified: p.Verified,
	}
}

func (p *PractitionerWithUser) ToRs() *RsPractitioner {
	return &RsPractitioner{
		ID:       p.ID,
		ABN:      p.ABN,
		Verified: p.Verified,
		User: &RsUserInfo{
			ID:         p.UserID,
			Email:      p.Email,
			FirstName:  p.FirstName,
			LastName:   p.LastName,
			Phone:      p.Phone,
			JoinedDate: p.CreatedAt,
		},
	}
}

type Filter struct {
	ID           *uuid.UUID `form:"id"`
	Email        *string    `form:"email"`
	FirstName    *string    `form:"first_name"`
	LastName     *string    `form:"last_name"`
	Phone        *string    `form:"phone"`
	ABN          *string    `form:"abn"`
	AccountantID *uuid.UUID `form:"-"` //internal used only
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}

	if filter.ID != nil {
		filters["p.id"] = *filter.ID
	}
	if filter.Email != nil {
		filters["u.email"] = *filter.Email
	}
	if filter.FirstName != nil {
		filters["u.first_name"] = *filter.FirstName
	}
	if filter.LastName != nil {
		filters["u.last_name"] = *filter.LastName
	}
	if filter.Phone != nil {
		filters["u.phone"] = *filter.Phone
	}
	if filter.ABN != nil {
		filters["p.abn"] = *filter.ABN
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
	return f
}
