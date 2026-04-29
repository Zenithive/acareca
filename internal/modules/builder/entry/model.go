package entry

import (
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

const (
	EntryStatusDraft     = "DRAFT"
	EntryStatusSubmitted = "SUBMITTED"
)

type RqEntryValue struct {
	FormFieldID *string  `json:"form_field_id" validate:"omitempty,uuid"`
	CoaID       *string  `json:"coa_id" validate:"omitempty,uuid"`
	Amount      float64  `json:"amount" validate:"omitempty,min=0"`
	NetAmount   *float64 `json:"net_amount,omitempty"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
	Description *string  `json:"description,omitempty"` // For direct COA entries
}

// Validate ensures either form_field_id OR coa_id is provided (not both, not neither)
func (r *RqEntryValue) Validate() error {
	hasFormField := r.FormFieldID != nil && *r.FormFieldID != ""
	hasCoaID := r.CoaID != nil && *r.CoaID != ""

	if !hasFormField && !hasCoaID {
		return errors.New("either form_field_id or coa_id must be provided")
	}
	if hasFormField && hasCoaID {
		return errors.New("cannot provide both form_field_id and coa_id")
	}
	return nil
}

type RqFormEntry struct {
	ClinicID uuid.UUID      `json:"clinic_id" validate:"required,uuid"`
	Status   string         `json:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	Date     *string        `json:"date" validate:"omitempty"`
	Values   []RqEntryValue `json:"values,omitempty"`
}

type RqUpdateFormEntry struct {
	Status *string        `json:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	Date   *string        `json:"date" validate:"omitempty"`
	Values []RqEntryValue `json:"values,omitempty"`
}

type FormEntry struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	FormVersionID  uuid.UUID  `db:"form_version_id" json:"form_version_id"`
	PractitionerID uuid.UUID  `db:"practitioner_id" json:"practitioner_id"`
	ClinicID       uuid.UUID  `db:"clinic_id" json:"clinic_id"`
	SubmittedBy    *uuid.UUID `db:"submitted_by" json:"submitted_by,omitempty"`
	SubmittedAt    *string    `db:"submitted_at" json:"submitted_at,omitempty"`
	Date           *string    `db:"date" json:"date,omitempty"`
	Status         string     `db:"status" json:"status"`
	CreatedAt      string     `db:"created_at" json:"created_at"`
	UpdatedAt      *string    `db:"updated_at" json:"updated_at,omitempty"`
}

type FormEntryValue struct {
	ID          uuid.UUID  `db:"id"`
	EntryID     uuid.UUID  `db:"entry_id"`
	FormFieldID *uuid.UUID `db:"form_field_id"`
	CoaID       *uuid.UUID `db:"coa_id"`
	NetAmount   *float64   `db:"net_amount"`
	GstAmount   *float64   `db:"gst_amount"`
	GrossAmount *float64   `db:"gross_amount"`
	Description *string    `db:"description"`
	CreatedAt   string     `db:"created_at"`
	UpdatedAt   *string    `db:"updated_at"`
}

func (d *FormEntry) ToRs(values []*FormEntryValue) *RsFormEntry {
	rs := &RsFormEntry{
		ID:            d.ID,
		FormVersionID: d.FormVersionID,
		ClinicID:      d.ClinicID,
		Date:          d.Date,
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
		ev := RsEntryValue{
			FormFieldID: v.FormFieldID,
			CoaID:       v.CoaID,
			Description: v.Description,
		}
		if v.GstAmount != nil {
			ev.NetAmount = v.NetAmount
			ev.GstAmount = v.GstAmount
			ev.GrossAmount = v.GrossAmount
		} else {
			ev.Amount = v.NetAmount
		}
		rs.Values = append(rs.Values, ev)
	}
	return rs
}

type RsFormEntry struct {
	ID            uuid.UUID      `json:"id"`
	FormVersionID uuid.UUID      `json:"form_version_id"`
	ClinicID      uuid.UUID      `json:"clinic_id"`
	SubmittedBy   *uuid.UUID     `json:"submitted_by,omitempty"`
	SubmittedAt   string         `json:"submitted_at,omitempty"`
	Date          *string        `json:"date,omitempty"`
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
	FormFieldID *uuid.UUID `json:"form_field_id,omitempty"` // Nullable for direct COA entries
	CoaID       *uuid.UUID `json:"coa_id,omitempty"`        // For direct COA entries
	FieldKey    string     `json:"field_key,omitempty"`
	Label       string     `json:"label,omitempty"`
	IsComputed  bool       `json:"is_computed"`
	Description *string    `json:"description,omitempty"` // For direct COA entries
	// Amount is used when there is no GST (net == gross).
	Amount *float64 `json:"amount,omitempty"`
	// NetAmount, GstAmount, GrossAmount are used when a GST breakdown exists.
	NetAmount   *float64 `json:"net_amount,omitempty"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
}

type Filter struct {
	ClinicID *string `form:"clinic_id"`
	common.Filter
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
	Date          *string   `json:"date,omitempty"`
	IsExpense     bool      `json:"is_expense"`
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
	StartDate      *string `form:"start_date"` // Alias for date_from (used by COA entries endpoints)
	EndDate        *string `form:"end_date"`   // Alias for date_to (used by COA entries endpoints)
	VersionID      *string `form:"version_id"`
	Status         *string `form:"status" validate:"omitempty,oneof=DRAFT SUBMITTED"`
	Role           string  `form:"-"`
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

	// Support both date_from/date_to and start_date/end_date
	// Priority: start_date/end_date (for COA endpoints) > date_from/date_to (for transactions endpoint)
	dateFrom := f.DateFrom
	if f.StartDate != nil && *f.StartDate != "" {
		dateFrom = f.StartDate
	}
	if dateFrom != nil && *dateFrom != "" {
		filters["start_date"] = *dateFrom
		operators["start_date"] = common.OpGt
	}

	dateTo := f.DateTo
	if f.EndDate != nil && *f.EndDate != "" {
		dateTo = f.EndDate
	}
	if dateTo != nil && *dateTo != "" {
		filters["end_date"] = *dateTo
		operators["end_date"] = common.OpLt
	}

	return common.NewFilter(f.Search, filters, operators, f.Limit, f.Offset, f.SortBy, f.OrderBy)
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

	cf := common.NewFilter(f.Search, filters, nil, f.Limit, f.Offset, f.SortBy, f.OrderBy)

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
	Date          *string   `db:"date"`
	IsExpense     bool      `db:"is_expense"`
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

// RsCoaEntry represents a grouped COA row for the parent grid
type RsCoaEntry struct {
	CoaID            string  `json:"coa_id"`
	CoaName          string  `json:"coa_name"`
	TotalNetAmount   float64 `json:"total_net_amount"`
	TotalGrossAmount float64 `json:"total_gross_amount"`
	EntryCount       int     `json:"entry_count"`

	Details []*RsCoaEntryDetail `json:"details,omitempty"` // <--- Added for export purpose
}

// RsCoaEntryDetail represents a detailed entry row for the child grid
type RsCoaEntryDetail struct {
	ID            string   `json:"id"`
	EntryID       string   `json:"entry_id"`
	FormFieldID   string   `json:"form_field_id"`
	CoaID         string   `json:"coa_id"`
	TaxTypeID     *int16   `json:"tax_type_id"`
	FormID        string   `json:"form_id"`
	ClinicID      *string  `json:"clinic_id,omitempty"`
	VersionID     string   `json:"version_id"`
	FormFieldName string   `json:"form_field_name"`
	CoaName       string   `json:"coa_name"`
	TaxTypeName   *string  `json:"tax_type_name"`
	FormName      *string  `json:"form_name,omitempty"`
	ClinicName    *string  `json:"clinic_name,omitempty"`
	SupplierName  *string  `json:"supplier_name,omitempty"`
	IsExpense     bool     `json:"is_expense"`
	NetAmount     *float64 `json:"net_amount"`
	GstAmount     *float64 `json:"gst_amount"`
	GrossAmount   *float64 `json:"gross_amount"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     *string  `json:"updated_at,omitempty"`
	Date          *string  `json:"date,omitempty"`
}
