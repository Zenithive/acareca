package detail

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
	StatusArchived  = "ARCHIVED"
)

type Filter struct {
	ClinicID *string `form:"clinic_id"`
	FormName *string `form:"form_name"`
	Method   *string `form:"method"`
	Status   *string `form:"status"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.ClinicID != nil {
		id, err := uuid.Parse(*filter.ClinicID)
		if err == nil {
			filters["clinic_id"] = id // Pass as uuid.UUID type to common.Filter
		}
	}
	if filter.Status != nil {
		filters["status"] = *filter.Status
	}
	if filter.Method != nil {
		filters["method"] = *filter.Method
	}
	if filter.FormName != nil {
		filters["form_name"] = *filter.FormName
	}
	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset)

	return f
}

type RqFormDetail struct {
	Name           string   `json:"name" validate:"required"`
	Description    *string  `json:"description" validate:"omitempty"`
	Status         string   `json:"status" validate:"required,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method         string   `json:"method" validate:"required,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE"`
	OwnerShare     int      `json:"owner_share" validate:"required,min=0,max=100"`
	ClinicShare    int      `json:"clinic_share" validate:"required,min=0,max=100"`
	SuperComponent *float64 `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type RqUpdateFormDetail struct {
	ID             uuid.UUID `json:"id" validate:"required"`
	Name           *string   `json:"name" validate:"omitempty"`
	Description    *string   `json:"description" validate:"omitempty"`
	Status         *string   `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method         *string   `json:"method" validate:"omitempty,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE"`
	OwnerShare     *int      `json:"owner_share" validate:"omitempty,min=0,max=100"`
	ClinicShare    *int      `json:"clinic_share" validate:"omitempty,min=0,max=100"`
	SuperComponent *float64  `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type FormDetail struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	ClinicID        uuid.UUID  `db:"clinic_id" json:"clinic_id"`
	Name            string     `db:"name" json:"name"`
	Description     *string    `db:"description" json:"description,omitempty"`
	Status          string     `db:"status" json:"status"`
	Method          string     `db:"method" json:"method"`
	OwnerShare      int        `db:"owner_share" json:"owner_share"`
	ClinicShare     int        `db:"clinic_share" json:"clinic_share"`
	SuperComponent  *float64   `db:"super_component" json:"super_component,omitempty"`
	ActiveVersionID *uuid.UUID `db:"active_version_id" json:"active_version_id,omitempty"`
	CreatedAt       string     `db:"created_at" json:"created_at"`
	UpdatedAt       string     `db:"updated_at" json:"updated_at"`
}

func (r *RqFormDetail) ToDB(clinicID uuid.UUID) *FormDetail {
	return &FormDetail{
		ID:             uuid.New(),
		ClinicID:       clinicID,
		Name:           r.Name,
		Description:    r.Description,
		Status:         r.Status,
		Method:         r.Method,
		OwnerShare:     r.OwnerShare,
		ClinicShare:    r.ClinicShare,
		SuperComponent: r.SuperComponent,
	}
}

func (r *RqUpdateFormDetail) Update() *FormDetail {
	d := &FormDetail{ID: r.ID}
	if r.Name != nil {
		d.Name = *r.Name
	}
	if r.Description != nil {
		d.Description = r.Description
	}
	if r.Status != nil {
		d.Status = *r.Status
	}
	if r.Method != nil {
		d.Method = *r.Method
	}
	if r.OwnerShare != nil {
		d.OwnerShare = *r.OwnerShare
	}
	if r.ClinicShare != nil {
		d.ClinicShare = *r.ClinicShare
	}
	if r.SuperComponent != nil {
		d.SuperComponent = r.SuperComponent
	}
	return d
}

func (d *FormDetail) ToRs() *RsFormDetail {
	return &RsFormDetail{
		ID:              d.ID,
		ClinicID:        d.ClinicID,
		Name:            d.Name,
		Description:     d.Description,
		Status:          d.Status,
		Method:          d.Method,
		OwnerShare:      d.OwnerShare,
		ClinicShare:     d.ClinicShare,
		SuperComponent:  d.SuperComponent,
		ActiveVersionID: d.ActiveVersionID,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

type RsFormDetail struct {
	ID              uuid.UUID  `json:"id"`
	ClinicID        uuid.UUID  `json:"clinic_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	Status          string     `json:"status"`
	Method          string     `json:"method"`
	OwnerShare      int        `json:"owner_share"`
	ClinicShare     int        `json:"clinic_share"`
	SuperComponent  *float64   `json:"super_component,omitempty"`
	ActiveVersionID *uuid.UUID `json:"active_version_id,omitempty"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
}
