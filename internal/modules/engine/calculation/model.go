package calculation

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
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
	Formulas       []formula.RsFormula    `json:"formulas"`
}

type RsCoaEntry struct {
	CoaID            string  `json:"coa_id" db:"coa_id"`
	CoaName          string  `json:"coa_name" db:"coa_name"`
	SectionType      string  `json:"section_type" db:"section_type"`
	TotalNetAmount   float64 `json:"total_net_amount" db:"total_net_amount"`
	TotalGSTAmount   float64 `json:"total_gst_amount" db:"total_gst_amount"`
	TotalGrossAmount float64 `json:"total_gross_amount" db:"total_gross_amount"`
	EntryCount       int     `json:"entry_count" db:"entry_count"`
}

type RsCategorizedSummary struct {
	Collection []*RsCoaEntry `json:"collection"`
	Costs      []*RsCoaEntry `json:"costs"`
	OtherCosts []*RsCoaEntry `json:"other_costs"`
}

// Preview calculation structs
// RqPreviewEntry represents a single field entry for preview calculation
type RqPreviewEntry struct {
	FieldKey    string   `json:"field_key" validate:"required"`
	NetAmount   float64  `json:"net_amount"`
	GstAmount   *float64 `json:"gst_amount,omitempty"`
	GrossAmount *float64 `json:"gross_amount,omitempty"`
}

// RqPreviewFormula represents a formula definition
type RqPreviewFormula struct {
	FieldKey   string            `json:"field_key" validate:"required"`
	Name       string            `json:"name" validate:"required"`
	Expression *formula.ExprNode `json:"expression" validate:"required"`
}

// RqPreviewField represents a field definition for preview calculation
type RqPreviewField struct {
	FieldKey              string            `json:"key" validate:"required"`
	Slug                  string            `json:"slug,omitempty"`
	Label                 string            `json:"label" validate:"required"`
	IsComputed            bool              `json:"is_computed"`
	Formula               *formula.ExprNode `json:"formula,omitempty"`
	SectionType           *string           `json:"section_type,omitempty"`
	PaymentResponsibility *string           `json:"payment_responsibility,omitempty"`
	TaxType               *string           `json:"tax_type,omitempty"`
	CoaID                 *string           `json:"coa_id,omitempty"`
	SortOrder             int               `json:"sort_order"`
	IsHighlighted         bool              `json:"is_highlighted"`
}

// RqFormPreview is the request for form preview calculation
type RqFormPreview struct {
	ClinicID       string             `json:"clinic_id" validate:"required,uuid"`
	Method         string             `json:"method" validate:"required,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE"`
	OwnerShare     int                `json:"owner_share" validate:"required,min=0,max=100"`
	ClinicShare    int                `json:"clinic_share" validate:"required,min=0,max=100"`
	Fields         []RqPreviewField   `json:"fields" validate:"required,min=1,dive"`
	Formulas       []RqPreviewFormula `json:"formulas,omitempty"`
	Values         []RqPreviewEntry   `json:"values" validate:"omitempty,dive"`
	SuperComponent *float64           `json:"super_component,omitempty" validate:"omitempty,min=0,max=100"`
}

// RsPreviewFieldValue represents a single field value in the preview response
type RsPreviewFieldValue struct {
	FieldKey      string   `json:"key"`
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
	Method     string                `json:"method"`
	FormName   string                `json:"form_name"`
	ClinicName string                `json:"clinic_name"`
	AllFields  []RsPreviewFieldValue `json:"all_fields"`
	Summary    *PreviewSummary       `json:"summary,omitempty"`
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

type RqICCalculation struct {
	gstamount    *float64
	grossamount  *float64
	actualamount float64 // value fed into formula engine (GROSS for OTHER_COST tax fields)
	displaynet   float64 // net amount shown in all_fields response
}
