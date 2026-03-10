package setting

import "time"

// Practitioner matches tbl_practitioner (spelling from schema).
type Practitioner struct {
	ID        int        `db:"id"`
	UserID    string     `db:"user_id"`
	ABN       *string    `db:"abn"`
	Verifed   bool       `db:"verifed"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// TentantSetting matches tbl_practitioner_setting.
type TentantSetting struct {
	ID        int        `db:"id"`
	TentantID int        `db:"tentant_id"`
	Timezone  string     `db:"timezone"`
	Logo      *string    `db:"logo"`
	Color     string     `db:"color"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// RqCreateTentant request to create a practitioner.
type RqCreateTentant struct {
	UserID  string  `json:"user_id" validate:"required"`
	ABN     *string `json:"abn" validate:"omitempty,max=20"`
	Verifed *bool   `json:"verifed"`
}

func (r *RqCreateTentant) ToTentant() *Practitioner {
	verified := false
	if r.Verifed != nil {
		verified = *r.Verifed
	}
	return &Practitioner{
		UserID:  r.UserID,
		ABN:     r.ABN,
		Verifed: verified,
	}
}

// RqUpdateTentant request to update a practitioner.
type RqUpdateTentant struct {
	ABN     *string `json:"abn" validate:"omitempty,max=20"`
	Verifed *bool   `json:"verifed"`
}

// RqUpsertTentantSetting request to create or update practitioner settings.
type RqUpsertTentantSetting struct {
	Timezone *string `json:"timezone" validate:"omitempty,max=255"`
	Logo     *string `json:"logo" validate:"omitempty,max=255"`
	Color    *string `json:"color" validate:"omitempty,len=7"`
}

// RsTentant response for a practitioner.
type RsTentant struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	ABN       *string   `json:"abn,omitempty"`
	Verifed   bool      `json:"verifed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (t *Practitioner) ToRs() *RsTentant {
	return &RsTentant{
		ID:        t.ID,
		UserID:    t.UserID,
		ABN:       t.ABN,
		Verifed:   t.Verifed,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// RsTentantSetting response for practitioner settings.
type RsTentantSetting struct {
	ID        int       `json:"id"`
	TentantID int       `json:"tentant_id"`
	Timezone  string    `json:"timezone"`
	Logo      *string   `json:"logo,omitempty"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *TentantSetting) ToRs() *RsTentantSetting {
	return &RsTentantSetting{
		ID:        s.ID,
		TentantID: s.TentantID,
		Timezone:  s.Timezone,
		Logo:      s.Logo,
		Color:     s.Color,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
