package accountant

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	Name     string `json:"name"`
	ABN      string `json:"abn"`
	Address  string `json:"address"`
	City     string `json:"city"`
	Postcode string `json:"postcode"`
	Contacts any    `json:"contacts"`
}
