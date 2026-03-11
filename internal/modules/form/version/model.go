package version

import "github.com/google/uuid"

type RqFormVersion struct {
	FormId   string `json:"form_id" binding:"required"`
	Version  int    `json:"version" binding:"required"`
	IsActive bool   `json:"is_active" binding:"required"`
}

func (r *RqFormVersion) ToDB(formId uuid.UUID, userId uuid.UUID) *FormVersion {

	return &FormVersion{
		ID:        uuid.New(),
		FormId:    formId,
		Version:   r.Version,
		IsActive:  r.IsActive,
		CreatedBy: userId,
	}
}

type FormVersion struct {
	ID        uuid.UUID `json:"id" db:"id"`
	FormId    uuid.UUID `json:"form_id" db:"form_id"`
	Version   int       `json:"version" db:"version"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`
}

func (d *FormVersion) ToRs() *RsFormVersion {
	return &RsFormVersion{
		Id:        d.ID,
		FormId:    d.FormId,
		Version:   d.Version,
		IsActive:  d.IsActive,
		CreatedBy: d.CreatedBy,
	}
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
