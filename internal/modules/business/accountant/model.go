package accountant

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type Accountant struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	LicenseNo *string    `db:"license_no"`
	Verified  bool       `db:"verified"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type RqCreateAccountant struct {
	UserID    string `json:"user_id"`
	LicenseNo string `json:"license_no"`
}

type RsAccountant struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	LicenseNo *string   `json:"license_no"`
	Verified  bool      `json:"verified"`
}

type RsAccountantUser struct {
	ID        uuid.UUID `json:"id"            db:"id"`
	Email     string    `json:"email"         db:"email"`
	FirstName string    `json:"first_name"    db:"first_name"`
	LastName  string    `json:"last_name"     db:"last_name"`
	Phone     string    `json:"phone"         db:"phone"`

	Clinics          json.RawMessage `json:"clinics"        db:"clinics" swaggertype:"array,object"`
	InvitationStatus *string         `json:"invitation_status" db:"invitation_status"`

	CreatedAt time.Time `json:"created_at"    db:"created_at"`
	UpdatedAt time.Time `json:"updated_at"    db:"updated_at"`
}

type ClinicDetail struct {
	Name        string          `json:"name"`
	ABN         string          `json:"abn"`
	Description string          `json:"description" db:"description"`
	Address     string          `json:"address"`
	City        string          `json:"city"`
	Postcode    string          `json:"postcode"`
	Contacts    json.RawMessage `json:"contacts" db:"contacts" swaggertype:"array,object"`
}

type RsAccountantForm struct {
	ID             uuid.UUID `json:"id"               db:"id"`
	ClinicID       uuid.UUID `json:"clinic_id"        db:"clinic_id"`
	ClinicName     string    `json:"clinic_name"      db:"clinic_name"`
	Name           string    `json:"name"             db:"name"`
	Description    *string   `json:"description"      db:"description"`
	Status         string    `json:"status"           db:"status"`
	Method         string    `json:"method"           db:"method"`
	OwnerShare     int       `json:"owner_share"      db:"owner_share"`
	ClinicShare    int       `json:"clinic_share"     db:"clinic_share"`
	SuperComponent *float64  `json:"super_component"  db:"super_component"`
	CreatedAt      time.Time `json:"created_at"       db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"       db:"updated_at"`
}

// Analytics
type FilterAnalytics struct {
	ClinicID       *string    `form:"clinic_id"`
	FormID         *string    `form:"form_id"`
	PractitionerID *string    `form:"practitioner_id"`
	StartDate      *time.Time `form:"start_date" time_format:"2006-01-02T15:04:05Z07:00"`
	EndDate        *time.Time `form:"end_date" time_format:"2006-01-02T15:04:05Z07:00"`
	common.Filter
}

func (filter *FilterAnalytics) MapToFilter() common.Filter {
	filters := map[string]any{}
	if filter.ClinicID != nil {
		if id, err := uuid.Parse(*filter.ClinicID); err == nil {
			filters["clinic_id"] = id
		}
	}
	if filter.FormID != nil {
		if id, err := uuid.Parse(*filter.FormID); err == nil {
			filters["form_id"] = id
		}
	}
	if filter.PractitionerID != nil {
		if id, err := uuid.Parse(*filter.PractitionerID); err == nil {
			filters["practitioner_id"] = id
		}
	}
	if filter.StartDate != nil {
		filters["start_date"] = *filter.StartDate
	}
	if filter.EndDate != nil {
		filters["end_date"] = *filter.EndDate
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)

	return f
}

// Summary represents the overall statistics
type Summary struct {
	TotalClinics       int `json:"total_clinics" db:"total_clinics"`
	TotalForms         int `json:"total_forms" db:"total_forms"`
	TotalTransactions  int `json:"total_transactions" db:"total_transactions"`
	TotalPractitioners int `json:"total_practitioners" db:"total_practitioners"`
}

// RecentTransaction represents a transaction record
type RecentTransaction struct {
	ID         string    `json:"id" db:"id"`
	ClinicID   string    `json:"clinic_id" db:"clinic_id"`
	ClinicName string    `json:"clinic_name" db:"clinic_name"`
	Amount     float64   `json:"amount" db:"amount"`
	Type       string    `json:"type" db:"type"`
	Date       time.Time `json:"date" db:"date"`
	Status     string    `json:"status" db:"status"`
}

// Practitioner represents a practitioner record
type Practitioner struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Email       string `json:"email" db:"email"`
	ClinicCount int    `json:"clinic_count" db:"clinic_count"`
	Status      string `json:"status" db:"status"`
}

// Clinic represents a clinic record
type Clinic struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Location  string    `json:"location" db:"location"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Form represents a form record
type Form struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	ClinicID  string    `json:"clinic_id" db:"clinic_id"`
	Version   string    `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RsAnalytics represents the analytics response
type RsAnalytics struct {
	Summary            Summary             `json:"summary"`
	RecentTransactions []RecentTransaction `json:"recent_transactions"`
	Practitioners      []Practitioner      `json:"practitioners"`
	Clinics            []Clinic            `json:"clinics"`
	Forms              []Form              `json:"forms"`
}
