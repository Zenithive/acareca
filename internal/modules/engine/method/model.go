package method

type TaxTreatment string

const (
	TaxTreatmentInclusive TaxTreatment = "INCLUSIVE"
	TaxTreatmentExclusive TaxTreatment = "EXCLUSIVE"
	TaxTreatmentManual    TaxTreatment = "MANUAL"
	TaxTreatmentZero      TaxTreatment = "ZERO"
)

type Input struct {
	Amount    float64  `json:"amount" validate:"required,min=0"`
	GstAmount *float64 `json:"gst_amount" validate:"omitempty"`
}

type Result struct {
	Amount      float64 `json:"amount"`
	GstAmount   float64 `json:"gst_amount"`
	TotalAmount float64 `json:"total_amount"`
}
