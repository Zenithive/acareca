package entry

import (
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

const (
	EntryStatusDraft     = "DRAFT"
	EntryStatusSubmitted = "SUBMITTED"
)

type RqEntryValue struct {
	FormFieldID string   `json:"form_field_id" validate:"required,uuid"`
	Amount      float64  `json:"amount" validate:"required,min=0"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
}

type RqFormEntry struct {
	ClinicID uuid.UUID      `json:"clinic_id" validate:"required,uuid"`
	Status   string         `json:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	Values   []RqEntryValue `json:"values,omitempty"`
}

type RqUpdateFormEntry struct {
	Status *string        `json:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	Values []RqEntryValue `json:"values,omitempty"`
}

type FormEntry struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	FormVersionID uuid.UUID  `db:"form_version_id" json:"form_version_id"`
	ClinicID      uuid.UUID  `db:"clinic_id" json:"clinic_id"`
	SubmittedBy   *uuid.UUID `db:"submitted_by" json:"submitted_by,omitempty"`
	SubmittedAt   *string    `db:"submitted_at" json:"submitted_at,omitempty"`
	Status        string     `db:"status" json:"status"`
	CreatedAt     string     `db:"created_at" json:"created_at"`
	UpdatedAt     string     `db:"updated_at" json:"updated_at"`
}

type FormEntryValue struct {
	ID          uuid.UUID `db:"id"`
	EntryID     uuid.UUID `db:"entry_id"`
	FormFieldID uuid.UUID `db:"form_field_id"`
	NetAmount   *float64  `db:"net_amount"`
	GstAmount   *float64  `db:"gst_amount"`
	GrossAmount *float64  `db:"gross_amount"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   string    `db:"updated_at"`
}

func (d *FormEntry) ToRs(values []*FormEntryValue) *RsFormEntry {
	rs := &RsFormEntry{
		ID:            d.ID,
		FormVersionID: d.FormVersionID,
		ClinicID:      d.ClinicID,
		Status:        d.Status,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
	rs.SubmittedBy = d.SubmittedBy
	if d.SubmittedAt != nil {
		rs.SubmittedAt = *d.SubmittedAt
	}
	rs.Values = make([]RsEntryValue, 0, len(values))
	for _, v := range values {
		rs.Values = append(rs.Values, RsEntryValue{
			FormFieldID: v.FormFieldID,
			NetAmount:   v.NetAmount,
			GstAmount:   v.GstAmount,
			GrossAmount: v.GrossAmount,
		})
	}
	return rs
}

type RsFormEntry struct {
	ID            uuid.UUID      `json:"id"`
	FormVersionID uuid.UUID      `json:"form_version_id"`
	ClinicID      uuid.UUID      `json:"clinic_id"`
	SubmittedBy   *uuid.UUID     `json:"submitted_by,omitempty"`
	SubmittedAt   string         `json:"submitted_at,omitempty"`
	Status        string         `json:"status"`
	Values        []RsEntryValue `json:"values,omitempty"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

type RsEntryValue struct {
	FormFieldID uuid.UUID `json:"form_field_id"`
	NetAmount   *float64  `json:"net_amount,omitempty"`
	GstAmount   *float64  `json:"gst_amount,omitempty"`
	GrossAmount *float64  `json:"gross_amount,omitempty"`
}

type Filter struct {
	ClinicID *string `form:"clinic_id"`
	Search   *string `form:"search"`
	SortBy   *string `form:"sort_by"`
	OrderBy  *string `form:"order_by"`
	Limit    *int    `form:"limit"`
	Offset   *int    `form:"offset"`
}

func (f *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if f.ClinicID != nil {
		id, err := uuid.Parse(*f.ClinicID)
		if err != nil {
			log.Printf("failed to parse clinic id: %v", err)
		}
		filters["clinic_id"] = id
	}

	cf := common.NewFilter(f.Search, filters, nil, f.Limit, f.Offset)

	if f.SortBy != nil {
		cf.SortBy = *f.SortBy
	} else {
		cf.SortBy = "created_at"
	}
	if f.OrderBy != nil {
		cf.OrderBy = *f.OrderBy
	} else {
		cf.OrderBy = "DESC"
	}
	return cf
}
