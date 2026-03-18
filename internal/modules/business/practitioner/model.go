package practitioner

import (
	"time"

	"github.com/google/uuid"
)

type Practitioner struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	ABN       *string    `db:"abn"`
	Verified  bool       `db:"verified"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// PractitionerWithUser is used for JOIN queries
type PractitionerWithUser struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	ABN       *string    `db:"abn"`
	Verified  bool       `db:"verified"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`

	// user fields
	Email     string    `db:"email"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Phone     *string   `db:"phone"`
}

type RqCreatePractitioner struct {
	UserID string `json:"user_id"`
}

type RsUserInfo struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     *string   `json:"phone,omitempty"`
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
			ID:        p.UserID,
			Email:     p.Email,
			FirstName: p.FirstName,
			LastName:  p.LastName,
			Phone:     p.Phone,
		},
	}
}
