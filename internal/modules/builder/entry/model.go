package entry

import (
	"github.com/google/uuid"
)

const (
	EntryStatusDraft     = "DRAFT"
	EntryStatusSubmitted = "SUBMITTED"
)

type RqEntryValue struct {
	FormFieldID string   `json:"form_field_id" validate:"required,uuid"`
	NetAmount   *float64 `json:"net_amount,omitempty"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
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
	ID          uuid.UUID `db:"id" json:"id"`
	EntryID     uuid.UUID `db:"entry_id" json:"entry_id"`
	FormFieldID uuid.UUID `db:"form_field_id" json:"form_field_id"`
	NetAmount   *float64  `db:"net_amount" json:"net_amount,omitempty"`
	GstAmount   *float64  `db:"gst_amount" json:"gst_amount,omitempty"`
	GrossAmount *float64  `db:"gross_amount" json:"gross_amount,omitempty"`
	CreatedAt   string    `db:"created_at" json:"created_at"`
	UpdatedAt   string    `db:"updated_at" json:"updated_at"`
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
	ClinicID *uuid.UUID `json:"clinic_id,omitempty"`
}
