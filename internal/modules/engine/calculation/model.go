package calculation

import "github.com/iamarpitzala/acareca/internal/modules/builder/entry"

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
