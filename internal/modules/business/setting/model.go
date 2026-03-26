package setting

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// Practitioner matches tbl_practitioner (spelling from schema).
type Practitioner struct {
	ID        uuid.UUID  `db:"id"`
	UserID    string     `db:"user_id"`
	ABN       *string    `db:"abn"`
	Verified  bool       `db:"verified"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// PractitionerSetting matches tbl_practitioner_setting.
type PractitionerSetting struct {
	ID             int        `db:"id"`
	PractitionerID uuid.UUID  `db:"practitioner_id"`
	Timezone       string     `db:"timezone"`
	Logo           *string    `db:"logo"`
	Color          string     `db:"color"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// RqCreatePractitioner request to create a practitioner.
type RqCreatePractitioner struct {
	UserID   string  `json:"user_id" validate:"required"`
	ABN      *string `json:"abn" validate:"omitempty,max=20"`
	Verified *bool   `json:"verified"`
}

func (r *RqCreatePractitioner) ToPractitioner() *Practitioner {
	verified := false
	if r.Verified != nil {
		verified = *r.Verified
	}
	return &Practitioner{
		UserID:   r.UserID,
		ABN:      r.ABN,
		Verified: verified,
	}
}

// RqUpdatePractitioner request to update a practitioner.
type RqUpdatePractitioner struct {
	ABN      *string `json:"abn" validate:"omitempty,max=20"`
	Verified *bool   `json:"verified"`
}

// RqUpsertPractitionerSetting request to create or update practitioner settings.
type RqUpsertPractitionerSetting struct {
	Timezone *string `json:"timezone" validate:"omitempty,max=255"`
	Logo     *string `json:"logo" validate:"omitempty,max=255"`
	Color    *string `json:"color" validate:"omitempty,len=7"`
}

// RsPractitioner response for a practitioner.
type RsPractitioner struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	ABN       *string   `json:"abn,omitempty"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t *Practitioner) ToRs() *RsPractitioner {
	return &RsPractitioner{
		ID:        t.ID,
		UserID:    t.UserID,
		ABN:       t.ABN,
		Verified:  t.Verified,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// RsPractitionerSetting response for practitioner settings.
type RsPractitionerSetting struct {
	ID             int       `json:"id"`
	PractitionerID uuid.UUID `json:"practitioner_id"`
	Timezone       string    `json:"timezone"`
	Logo           *string   `json:"logo,omitempty"`
	Color          string    `json:"color"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *PractitionerSetting) ToRs() *RsPractitionerSetting {
	return &RsPractitionerSetting{
		ID:             s.ID,
		PractitionerID: s.PractitionerID,
		Timezone:       s.Timezone,
		Logo:           s.Logo,
		Color:          s.Color,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

type Filter struct {
	ID       *string `form:"id"`
	ABN      *string `form:"abn"`
	Verified *bool   `form:"verified"`
	Search   *string `form:"search"`
	Limit    *int    `form:"limit"`
	Offset   *int    `form:"offset"`
	SortBy   *string `form:"sort_by"`
	OrderBy  *string `form:"order_by"`
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}

	if filter.ID != nil {
		filters["id"] = *filter.ID
	}
	if filter.ABN != nil {
		filters["abn"] = *filter.ABN
	}
	if filter.Verified != nil {
		filters["verified"] = *filter.Verified
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset)
	if filter.SortBy != nil {
		f.SortBy = *filter.SortBy
	} else {
		f.SortBy = "created_at"
	}

	if filter.OrderBy != nil {
		f.OrderBy = *filter.OrderBy
	} else {
		f.OrderBy = "DESC"
	}
	return f
}
