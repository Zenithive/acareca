package detail

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/samber/lo"
)

type RqFormDetail struct {
	Name           string   `json:"name" validate:"required"`
	Description    *string  `json:"description" validate:"omitempty"`
	Status         string   `json:"status" validate:"required,oneof=DRAFT PUBLISHED"`
	Method         string   `json:"method" validate:"required,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE EXPENSE_ENTRY"`
	OwnerShare     int      `json:"owner_share" validate:"required,min=0,max=100"`
	ClinicShare    int      `json:"clinic_share" validate:"required,min=0,max=100"`
	SuperComponent *float64 `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type RqUpdateFormDetail struct {
	ID             uuid.UUID `json:"id" validate:"required"`
	Name           *string   `json:"name" validate:"omitempty"`
	Description    *string   `json:"description" validate:"omitempty"`
	Status         *string   `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED"`
	Method         *string   `json:"method" validate:"omitempty,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE EXPENSE_ENTRY"`
	OwnerShare     *int      `json:"owner_share" validate:"omitempty,min=0,max=100"`
	ClinicShare    *int      `json:"clinic_share" validate:"omitempty,min=0,max=100"`
	SuperComponent *float64  `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type FormDetail struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	ClinicID        uuid.UUID  `db:"clinic_id" json:"clinic_id"`
	ClinicName      string     `db:"clinic_name" json:"clinic_name"`
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
	var fdetail FormDetail
	fdetail.ID = r.ID
	if r.Name != nil {
		fdetail.Name = *r.Name
	}
	if r.Description != nil {
		fdetail.Description = r.Description
	}
	if r.Status != nil {
		fdetail.Status = *r.Status
	}
	if r.Method != nil {
		fdetail.Method = *r.Method
	}
	if r.OwnerShare != nil {
		fdetail.OwnerShare = *r.OwnerShare
	}
	if r.ClinicShare != nil {
		fdetail.ClinicShare = *r.ClinicShare
	}
	if r.SuperComponent != nil {
		fdetail.SuperComponent = r.SuperComponent
	}
	return &fdetail
}

func (d *FormDetail) ToRs() *RsFormDetail {
	rs := &RsFormDetail{
		ID:              d.ID,
		Name:            d.Name,
		Description:     d.Description,
		Status:          d.Status,
		Method:          d.Method,
		SuperComponent:  d.SuperComponent,
		ActiveVersionID: d.ActiveVersionID,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}

	if d.Method != "EXPENSE_ENTRY" {
		rs.ClinicID = &d.ClinicID
		rs.OwnerShare = &d.OwnerShare
		rs.ClinicShare = &d.ClinicShare
	}

	return rs
}

type RsFormDetail struct {
	ID              uuid.UUID  `json:"id"`
	ClinicID        *uuid.UUID `json:"clinic_id,omitempty"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	Status          string     `json:"status"`
	Method          string     `json:"method"`
	OwnerShare      *int       `json:"owner_share,omitempty"`
	ClinicShare     *int       `json:"clinic_share,omitempty"`
	SuperComponent  *float64   `json:"super_component,omitempty"`
	ActiveVersionID *uuid.UUID `json:"active_version_id,omitempty"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
}

type RqUpdateFormStatus struct {
	ID     uuid.UUID `json:"id" validate:"required"`
	Status string    `json:"status" validate:"required,oneof=DRAFT PUBLISHED"`
}

type Filter struct {
	PractitionerID *uuid.UUID  `form:"practitioner_id"`
	ClinicIDs      []uuid.UUID `form:"clinic_ids"`
	FormName       *string     `form:"name"`
	Method         *string     `form:"method"`
	Status         *string     `form:"status"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.PractitionerID != nil {
		filters["practitioner_id"] = filter.PractitionerID
	}
	if filter.ClinicIDs != nil {
		Ids := make([]uuid.UUID, 0, len(filter.ClinicIDs))
		for _, id := range filter.ClinicIDs {
			Ids = append(Ids, id)
		}

		if len(Ids) > 0 {
			filters["clinic_ids"] = Ids
		}
	}
	if filter.Status != nil {
		filters["status"] = lo.ToPtr(filter.Status)
	}
	if filter.Method != nil {
		filters["method"] = lo.ToPtr(filter.Method)
	}
	if filter.FormName != nil {
		filters["form_name"] = lo.ToPtr(*filter.FormName)
	}
	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)

	return f
}
