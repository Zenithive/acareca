package version

import "github.com/google/uuid"

type RqFormVersion struct {
	Version  int  `json:"version" validate:"required"`
	IsActive bool `json:"is_active" validate:"required"`
}

type RqUpdateFormVersion struct {
	Version  *int  `json:"version" validate:"omitempty"`
	IsActive *bool `json:"is_active" validate:"omitempty"`
}

func (r *RqFormVersion) ToDB(formId uuid.UUID, userId uuid.UUID) *FormVersion {
	var createdBy *uuid.UUID
	if userId != uuid.Nil {
		createdBy = &userId
	}
	return &FormVersion{
		ID:        uuid.New(),
		FormId:    formId,
		Version:   r.Version,
		IsActive:  r.IsActive,
		CreatedBy: createdBy,
	}
}

type FormVersion struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	FormId    uuid.UUID  `json:"form_id" db:"form_id"`
	Version   int        `json:"version" db:"version"`
	IsActive  bool       `json:"is_active" db:"is_active"`
	CreatedBy *uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt string     `json:"created_at" db:"created_at"`
	UpdatedAt string     `json:"updated_at" db:"updated_at"`
}

func (d *FormVersion) ToRs() *RsFormVersion {
	rs := &RsFormVersion{
		Id:        d.ID,
		FormId:    d.FormId,
		Version:   d.Version,
		IsActive:  d.IsActive,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.CreatedBy != nil {
		rs.CreatedBy = *d.CreatedBy
	}
	return rs
}

type RsFormVersion struct {
	Id        uuid.UUID `json:"id"`
	FormId    uuid.UUID `json:"form_id"`
	Version   int       `json:"version"`
	IsActive  bool      `json:"is_active"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}
