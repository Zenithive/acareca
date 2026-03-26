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
	UpdatedAt     *string    `db:"updated_at" json:"updated_at,omitempty"`
}

type FormEntryValue struct {
	ID          uuid.UUID `db:"id"`
	EntryID     uuid.UUID `db:"entry_id"`
	FormFieldID uuid.UUID `db:"form_field_id"`
	NetAmount   *float64  `db:"net_amount"`
	GstAmount   *float64  `db:"gst_amount"`
	GrossAmount *float64  `db:"gross_amount"`
	CreatedAt   string    `db:"created_at"`
	UpdatedAt   *string   `db:"updated_at"`
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
	UpdatedAt     *string        `json:"updated_at"`

	// Populated for INDEPENDENT_CONTRACTOR forms only.
	Commission      *float64 `json:"commission,omitempty"`
	GstOnCommission *float64 `json:"gst_on_commission,omitempty"`
	PaymentReceived *float64 `json:"payment_received,omitempty"`
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

// RsTransactionRow is a flat, one-row-per-entry-value transaction response.
type RsTransactionRow struct {
	ID            uuid.UUID `json:"id"`
	EntryID       uuid.UUID `json:"entry_id"`
	FormFieldID   uuid.UUID `json:"form_field_id"`
	FormFieldName string    `json:"form_field_name"`
	CoaID         uuid.UUID `json:"coa_id"`
	CoaName       string    `json:"coa_name"`
	TaxTypeID     *int16    `json:"tax_type_id"`
	TaxTypeName   *string   `json:"tax_type_name"`
	FormID        uuid.UUID `json:"form_id"`
	FormName      string    `json:"form_name"`
	ClinicID      uuid.UUID `json:"clinic_id"`
	ClinicName    string    `json:"clinic_name"`
	NetAmount     *float64  `json:"net_amount"`
	GstAmount     *float64  `json:"gst_amount"`
	GrossAmount   *float64  `json:"gross_amount"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     *string   `json:"updated_at,omitempty"`
}

// RsTransactionDetail kept for backward compat (used by old RsTransaction).
type RsTransactionDetail struct {
	FieldName string   `json:"field_name"`
	GstType   *string  `json:"gst_type"`
	Amount    *float64 `json:"amount"`
	GstAmount *float64 `json:"gst_amount"`
	NetAmount *float64 `json:"net_amount"`
}

type RsTransaction struct {
	ID            uuid.UUID             `json:"id"`
	FormVersionID uuid.UUID             `json:"form_version_id"`
	ClinicID      uuid.UUID             `json:"clinic_id"`
	ClinicName    string                `json:"clinic_name"`
	FormID        uuid.UUID             `json:"form_id"`
	FormName      string                `json:"form_name"`
	Method        string                `json:"method"`
	FormStatus    string                `json:"form_status"`
	EntryDetail   []RsTransactionDetail `json:"entry_detail"`
}

type TransactionFilter struct {
	PractitionerID *string `form:"-"`
	ClinicID       *string `form:"clinic_id"`
	FormID         *string `form:"form_id"`
	CoaID          *string `form:"coa_id"`
	TaxTypeID      *int16  `form:"tax_type_id"`
	DateFrom       *string `form:"date_from"`
	DateTo         *string `form:"date_to"`
	VersionID      *string `form:"version_id"`
	Status         *string `form:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	common.Filter
}

func (f *TransactionFilter) ToCommonFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	if f.PractitionerID != nil && *f.PractitionerID != "" {
		if id, err := uuid.Parse(*f.PractitionerID); err == nil {
			filters["practitioner_id"] = id
		}
	}
	if f.ClinicID != nil && *f.ClinicID != "" {
		if id, err := uuid.Parse(*f.ClinicID); err == nil {
			filters["clinic_id"] = id
		}
	}
	if f.FormID != nil && *f.FormID != "" {
		if id, err := uuid.Parse(*f.FormID); err == nil {
			filters["form_id"] = id
		}
	}
	if f.CoaID != nil && *f.CoaID != "" {
		if id, err := uuid.Parse(*f.CoaID); err == nil {
			filters["coa_id"] = id
		}
	}
	if f.TaxTypeID != nil {
		filters["tax_type_id"] = *f.TaxTypeID
	}
	if f.VersionID != nil && *f.VersionID != "" {
		if id, err := uuid.Parse(*f.VersionID); err == nil {
			filters["version_id"] = id
		}
	}
	if f.Status != nil && *f.Status != "" {
		filters["status"] = *f.Status
	}
	if f.DateFrom != nil && *f.DateFrom != "" {
		filters["date_from"] = *f.DateFrom
		operators["date_from"] = common.OpGt
	}
	if f.DateTo != nil && *f.DateTo != "" {
		filters["date_to"] = *f.DateTo
		operators["date_to"] = common.OpLt
	}
	return common.NewFilter(nil, filters, operators, f.Limit, f.Offset)
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

	return cf
}

type transactionFlatRow struct {
	ID            uuid.UUID `db:"id"`
	EntryID       uuid.UUID `db:"entry_id"`
	FormFieldID   uuid.UUID `db:"form_field_id"`
	FormFieldName string    `db:"form_field_name"`
	CoaID         uuid.UUID `db:"coa_id"`
	CoaName       string    `db:"coa_name"`
	TaxTypeID     *int16    `db:"tax_type_id"`
	TaxTypeName   *string   `db:"tax_type_name"`
	FormID        uuid.UUID `db:"form_id"`
	FormName      string    `db:"form_name"`
	ClinicID      uuid.UUID `db:"clinic_id"`
	ClinicName    string    `db:"clinic_name"`
	NetAmount     *float64  `db:"net_amount"`
	GstAmount     *float64  `db:"gst_amount"`
	GrossAmount   *float64  `db:"gross_amount"`
	CreatedAt     string    `db:"created_at"`
	UpdatedAt     *string   `db:"updated_at"`
}

type RsFieldSummary struct {
	FormFieldID    uuid.UUID `json:"form_field_id"`
	Label          string    `json:"label"`
	SectionType    string    `json:"section_type"`
	Responsibility string    `json:"payment_responsibility"`
	TaxType        string    `json:"tax_type"`
	TotalNet       float64   `json:"total_net"`
	TotalGst       float64   `json:"total_gst"`
	TotalGross     float64   `json:"total_gross"`
}
