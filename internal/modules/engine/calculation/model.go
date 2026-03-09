package calculation

import "github.com/iamarpitzala/acareca/internal/modules/engine/method"

type PaidBy string

const (
	PaidByClinic PaidBy = "clinic"
	PaidByOwner  PaidBy = "owner"
)

type Input struct {
	Name     string              `json:"name" validate:"omitempty,max=255"`
	Value    float64             `json:"value" validate:"omitempty,min=0"`
	TaxType  method.TaxTreatment `json:"tax" validate:"omitempty,oneof=inclusive exclusive manual zero"`
	TaxValue *float64            `json:"tax_value" validate:"omitempty"`
	PaidBy   *PaidBy             `json:"paid_by" validate:"omitempty,oneof=clinic owner"`
}

type Entry struct {
	OwnerShare        *float64 `json:"owner_share" validate:"omitempty,min=0"`
	ClinicShare       *float64 `json:"clinic_share" validate:"omitempty,min=0"`
	Income            []Input  `json:"income" validate:"omitempty"`
	Expense           []Input  `json:"expense" validate:"omitempty"`
	OtherCosts        []Input  `json:"other_costs" validate:"omitempty"`
	SuperComponent    *float64 `json:"super_component" validate:"omitempty"`
	OutWorkPercentage *float64 `json:"out_work_percentage" validate:"omitempty,min=0,max=100"`
}

type NetAmountResult struct {
	Income  []float64 `json:"income"`
	Expense []float64 `json:"expense"`
	Result  float64   `json:"result"`
}

type GrossResult struct {
	NetAmount float64 `json:"net_amount"`

	ServiceFee      float64 `json:"service_fee"`
	GstServiceFee   float64 `json:"gst_service_fee"`
	TotalServiceFee float64 `json:"total_service_fee"`
	RemittedAmount  float64 `json:"remitted_amount"`
}

type NetResult struct {
	NetAmount                float64  `json:"net_amount"`
	Commission               float64  `json:"commission"`
	SuperComponent           *float64 `json:"super_component"`
	SuperComponentCommission *float64 `json:"super_component_commission"`
	TotalRemuneration        *float64 `json:"total_remuneration"`
	GstCommission            float64  `json:"gst_commission"`
	TotalCommission          float64  `json:"total_commission"`
}

type OutWorkResult struct {
	NetAmount       float64 `json:"net_amount"`
	ServiceFee      float64 `json:"service_fee"`
	GstServiceFee   float64 `json:"gst_service_fee"`
	TotalServiceFee float64 `json:"total_service_fee"`
	NetPayable      float64 `json:"net_payable"`
}
