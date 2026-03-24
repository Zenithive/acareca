package accountant

import (
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
