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
