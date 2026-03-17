package calculation

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

type NetFilter struct {
	SuperComponent *float64 `json:"super_component"`
}
