package method

const (
	// GSTRate is the standard GST rate in Australia (10%)
	GSTRate = 0.10
	// GSTDivisor is used for inclusive tax calculations (amount / 11 = GST component)
	GSTDivisor = 11.0
	// GSTMultiplier is used for inclusive tax calculations (1 + GST rate)
	GSTMultiplier = 1.1
)

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
