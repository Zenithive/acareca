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

type RqCreatePractitioner struct {
	UserID string `json:"user_id"`
}

type RsPractitioner struct {
	ID     uuid.UUID `json:"id"`
	UserID string    `json:"user_id"`
}

func (p *Practitioner) ToRs() *RsPractitioner {
	return &RsPractitioner{
		ID:     p.ID,
		UserID: p.UserID.String(),
	}
}
