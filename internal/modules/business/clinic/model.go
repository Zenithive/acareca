package clinic

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// Database models
type Clinic struct {
	ID             uuid.UUID  `db:"id"`
	PractitionerID uuid.UUID  `db:"practitioner_id"`
	ProfilePicture *string    `db:"profile_picture"`
	Name           string     `db:"name"`
	ABN            *string    `db:"abn"`
	Description    *string    `db:"description"`
	IsActive       bool       `db:"is_active"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

type ClinicAddress struct {
	ID        uuid.UUID `db:"id"`
	ClinicID  uuid.UUID `db:"clinic_id"`
	Address   *string   `db:"address"`
	City      *string   `db:"city"`
	State     *string   `db:"state"`
	Postcode  *string   `db:"postcode"`
	IsPrimary bool      `db:"is_primary"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type ClinicContact struct {
	ID          uuid.UUID `db:"id"`
	ClinicID    uuid.UUID `db:"clinic_id"`
	ContactType string    `db:"contact_type"`
	Value       string    `db:"value"`
	Label       *string   `db:"label"`
	IsPrimary   bool      `db:"is_primary"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type FinancialSettings struct {
	ID              uuid.UUID  `db:"id"`
	ClinicID        uuid.UUID  `db:"clinic_id"`
	FinancialYearID uuid.UUID  `db:"financial_year_id"`
	LockDate        *time.Time `db:"lock_date"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// Request models
type RqCreateClinic struct {
	ProfilePicture *string           `json:"profile_picture"`
	Name           string            `json:"name" validate:"required"`
	ABN            *string           `json:"abn" validate:"omitempty,len=11"`
	Description    *string           `json:"description"`
	IsActive       *bool             `json:"is_active"`
	Addresses      []RqClinicAddress `json:"addresses"`
	Contacts       []RqClinicContact `json:"contacts"`
}

type RqClinicAddress struct {
	Address   *string `json:"address"`
	City      *string `json:"city"`
	State     *string `json:"state"`
	Postcode  *string `json:"postcode" validate:"omitempty,len=4"`
	IsPrimary *bool   `json:"is_primary"`
}

type RqClinicContact struct {
	ContactType string  `json:"contact_type" validate:"required,oneof=PHONE EMAIL WEBSITE FAX"`
	Value       string  `json:"value" validate:"required"`
	Label       *string `json:"label"`
	IsPrimary   *bool   `json:"is_primary"`
}

type RqFinancialSettings struct {
	FinancialYearID *uuid.UUID `json:"financial_year_id"` // optional; omit or null to skip financial settings
	LockDate        *time.Time `json:"lock_date"`
}

type RqUpdateClinic struct {
	ID              *uuid.UUID        `json:"id"`
	Name            *string           `json:"name"`
	ProfilePicture  *string           `json:"profile_picture"`
	ABN             *string           `json:"abn" validate:"omitempty,len=11"`
	Description     *string           `json:"description"`
	IsActive        *bool             `json:"is_active"`
	Addresses       []RqUpdateAddress `json:"addresses"`
	Contacts        []RqUpdateContact `json:"contacts"`
	FinancialYearID *uuid.UUID        `json:"financial_year_id"`
	LockDate        *time.Time        `json:"lock_date"`
}

type RqUpdateAddress struct {
	ID        *uuid.UUID `json:"id"`
	Address   *string    `json:"address"`
	City      *string    `json:"city"`
	State     *string    `json:"state"`
	Postcode  *string    `json:"postcode" validate:"omitempty,len=4"`
	IsPrimary *bool      `json:"is_primary"`
}

type RqUpdateContact struct {
	ID          *uuid.UUID `json:"id"`
	ContactType *string    `json:"contact_type" validate:"omitempty,oneof=PHONE EMAIL WEBSITE FAX"`
	Value       *string    `json:"value"`
	Label       *string    `json:"label"`
	IsPrimary   *bool      `json:"is_primary"`
}

type RqBulkUpdateClinic struct {
	Clinics []RqUpdateClinic `json:"clinics" validate:"required,dive"`
}

type RqBulkDeleteClinic struct {
	ClinicIDs []uuid.UUID `json:"clinic_ids" validate:"required,min=1"`
}

// Response models
type RsClinic struct {
	ID                uuid.UUID            `json:"id"`
	PractitionerID    uuid.UUID            `json:"practitioner_id"`
	ProfilePicture    *string              `json:"profile_picture,omitempty"`
	Name              string               `json:"name"`
	ABN               *string              `json:"abn,omitempty"`
	Description       *string              `json:"description,omitempty"`
	IsActive          bool                 `json:"is_active"`
	Addresses         []RsClinicAddress    `json:"addresses,omitempty"`
	Contacts          []RsClinicContact    `json:"contacts,omitempty"`
	FinancialSettings *RsFinancialSettings `json:"financial_settings,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

type RsClinicAddress struct {
	ID        uuid.UUID `json:"id"`
	Address   *string   `json:"address,omitempty"`
	City      *string   `json:"city,omitempty"`
	State     *string   `json:"state,omitempty"`
	Postcode  *string   `json:"postcode,omitempty"`
	IsPrimary bool      `json:"is_primary"`
}

type RsClinicContact struct {
	ID          uuid.UUID `json:"id"`
	ContactType string    `json:"contact_type"`
	Value       string    `json:"value"`
	Label       *string   `json:"label,omitempty"`
	IsPrimary   bool      `json:"is_primary"`
}

type RsFinancialSettings struct {
	ID              uuid.UUID  `json:"id"`
	FinancialYearID uuid.UUID  `json:"financial_year_id"`
	LockDate        *time.Time `json:"lock_date,omitempty"`
}

type Filter struct {
	ClinicName *string `form:"name"`
	ClinicId   *string `form:"id"`
	Search     *string `form:"search"`
	SortBy     *string `form:"sort_by"`
	OrderBy    *string `form:"order_by"`
	Limit      *int    `form:"limit"`
	Offset     *int    `form:"offset"`
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.ClinicId != nil {
		filters["clinic_id"] = *filter.ClinicId
	}
	if filter.ClinicName != nil {
		filters["name"] = *filter.ClinicName
	}

	f := common.NewFilter(nil, filters, nil, nil, nil)
	if filter.SortBy != nil {
		f.SortBy = *filter.SortBy
	}
	if filter.OrderBy != nil {
		f.OrderBy = *filter.OrderBy
	}

	if filter.Limit != nil {
		f.Limit = *filter.Limit
	}

	if filter.Offset != nil {
		f.Offset = *filter.Offset
	}

	if filter.Search != nil {
		f.Search = filter.Search
	}

	return f
}
