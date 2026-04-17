package calculation

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
)

type Method string

const (
	IndependentContractor Method = "INDEPENDENT_CONTRACTOR"
	ServiceFee            Method = "SERVICE_FEE"
)

type GrossResult struct {
	NetAmount float64 `json:"net_amount"`

	ServiceFee      float64 `json:"service_fee"`
	GstServiceFee   float64 `json:"gst_service_fee"`
	TotalServiceFee float64 `json:"total_service_fee"`
	RemittedAmount  float64 `json:"remitted_amount"`

	ClinicExpenseGST float64 `json:"clinic_expense_gst"`
}

type NetResult struct {
	NetAmount float64 `json:"net_amount"`

	TotalRemuneration float64 `json:"total_remuneration"`

	BaseRemuneration *float64 `json:"base_remuneration,omitempty"`

	SuperComponent *float64 `json:"super_component,omitempty"`

	GstOnRemuneration float64 `json:"gst_on_remuneration"`

	InvoiceTotal float64 `json:"invoice_total"`

	OtherCostDeduction float64 `json:"other_cost_deduction"`
}

type NetFilter struct {
	SuperComponent *float64 `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type RqCalculateFromEntries struct {
	FormID  string               `json:"form_id" validate:"required,uuid"`
	Entries []entry.RsEntryValue `json:"entries" validate:"required,min=1,dive"`

	SuperComponent *float64 `json:"super_component" validate:"omitempty,min=0,max=100"`
}

type RqFormulaCalculate struct {
	Values map[string]float64
}

// RsComputedFieldValue is the per-field result for a computed field.
type RsComputedFieldValue struct {
	FieldID       uuid.UUID  `json:"field_id"`
	FormFieldID   string     `json:"form_field_id"` // UUID as string for consistency with request
	FieldKey      string     `json:"field_key"`
	Label         string     `json:"label"`
	IsComputed    bool       `json:"is_computed"`
	NetAmount     float64    `json:"net_amount"`       // net amount (ex-GST when tax applies)
	GstAmount     *float64   `json:"gst_amount"`       // GST amount, null when no tax
	GrossAmount   *float64   `json:"gross_amount"`     // gross amount (including GST), null when no tax
	SectionType   *string    `json:"section_type"`     // COLLECTION, COST, OTHER_COST
	TaxType       *string    `json:"tax_type"`         // INCLUSIVE, EXCLUSIVE, MANUAL, ZERO
	CoaID         *uuid.UUID `json:"coa_id,omitempty"` // Chart of Account ID
	SortOrder     int        `json:"sort_order"`
	IsHighlighted bool       `json:"is_highlighted"`
}

// RsFormulaCalculate is the response for POST /calculate/formula/:form_id.
type RsFormulaCalculate struct {
	FormID         uuid.UUID              `json:"form_id"`
	ComputedFields []RsComputedFieldValue `json:"computed_fields"`
}

// RqLiveCalculateEntry represents a single field entry for live calculation.
type RqLiveCalculateEntry struct {
	FormFieldID string   `json:"form_field_id" validate:"required,uuid"`
	NetAmount   float64  `json:"net_amount"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
}

// RqLiveCalculate is the request for live calculation based on form version ID.
type RqLiveCalculate struct {
	FormVersionID string                 `json:"form_version_id" validate:"required,uuid"`
	Entries       []RqLiveCalculateEntry `json:"entries" validate:"required,min=1,dive"`
}

// RsLiveCalculate is the response for live calculation.
type RsLiveCalculate struct {
	FormVersionID  uuid.UUID              `json:"form_version_id"`
	ComputedFields []RsComputedFieldValue `json:"computed_fields"`
}

// RsTransactionRow represents a single row of transaction data for a form entry value.
type RsTransactionRow struct {
	ID            uuid.UUID `db:"id"`
	EntryID       uuid.UUID `db:"entry_id"`
	FormFieldID   uuid.UUID `db:"form_field_id"`
	FormFieldName string    `db:"form_field_name"`
	SectionType   *string   `db:"section_type"`
	TaxType       *string   `db:"tax_type"`
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

// transactionFlatRow is used internally for SQL scanning.
// It uses *pointers for nullable fields to avoid scanning errors.
type transactionFlatRow struct {
	ID            uuid.UUID `db:"id"`
	EntryID       uuid.UUID `db:"entry_id"`
	FormFieldID   uuid.UUID `db:"form_field_id"`
	FormFieldName string    `db:"form_field_name"`
	SectionType   *string   `db:"section_type"`
	TaxType       *string   `db:"tax_type"`
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

// Preview calculation structs
// RqPreviewEntry represents a single field entry for preview calculation
type RqPreviewEntry struct {
	FormFieldID string   `json:"form_field_id" validate:"required,uuid"`
	NetAmount   float64  `json:"net_amount"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
}

// RqFormPreview is the request for form preview calculation
type RqFormPreview struct {
	FormVersionID  string           `json:"form_version_id" validate:"required,uuid"`
	ClinicID       string           `json:"clinic_id" validate:"required,uuid"`
	Entries        []RqPreviewEntry `json:"entries" validate:"required,min=1,dive"`
	SuperComponent *float64         `json:"super_component,omitempty" validate:"omitempty,min=0,max=100"`
}

// RsPreviewFieldValue represents a single field value in the preview response
type RsPreviewFieldValue struct {
	FormFieldID   string   `json:"form_field_id"`
	FieldKey      string   `json:"field_key"`
	Label         string   `json:"label"`
	IsComputed    bool     `json:"is_computed"`
	NetAmount     *float64 `json:"net_amount,omitempty"`
	GstAmount     *float64 `json:"gst_amount,omitempty"`
	GrossAmount   *float64 `json:"gross_amount,omitempty"`
	SectionType   *string  `json:"section_type,omitempty"`
	TaxType       *string  `json:"tax_type,omitempty"`
	CoaID         *string  `json:"coa_id,omitempty"`
	SortOrder     int      `json:"sort_order"`
	IsHighlighted bool     `json:"is_highlighted"`
}

// RsFormPreview is the response for form preview calculation
type RsFormPreview struct {
	FormVersionID uuid.UUID             `json:"form_version_id"`
	ClinicID      uuid.UUID             `json:"clinic_id"`
	Method        string                `json:"method"`
	FormName      string                `json:"form_name"`
	ClinicName    string                `json:"clinic_name"`
	AllFields     []RsPreviewFieldValue `json:"all_fields"`
	Summary       *PreviewSummary       `json:"summary,omitempty"`
}

// PreviewSummary contains calculation summary based on form method
type PreviewSummary struct {
	// Common fields
	NetAmount float64 `json:"net_amount"`

	// SERVICE_FEE method fields
	ServiceFee       *float64 `json:"service_fee,omitempty"`
	GstServiceFee    *float64 `json:"gst_service_fee,omitempty"`
	TotalServiceFee  *float64 `json:"total_service_fee,omitempty"`
	RemittedAmount   *float64 `json:"remitted_amount,omitempty"`
	ClinicExpenseGST *float64 `json:"clinic_expense_gst,omitempty"`

	// INDEPENDENT_CONTRACTOR method fields
	TotalRemuneration  *float64 `json:"total_remuneration,omitempty"`
	BaseRemuneration   *float64 `json:"base_remuneration,omitempty"`
	SuperComponent     *float64 `json:"super_component,omitempty"`
	GstOnRemuneration  *float64 `json:"gst_on_remuneration,omitempty"`
	InvoiceTotal       *float64 `json:"invoice_total,omitempty"`
	OtherCostDeduction *float64 `json:"other_cost_deduction,omitempty"`

	// IC-specific fields (from attachICCalculation)
	Commission      *float64 `json:"commission,omitempty"`
	GstOnCommission *float64 `json:"gst_on_commission,omitempty"`
	PaymentReceived *float64 `json:"payment_received,omitempty"`
}
