package formula

type ExpressionType string

const (
	BASCODE  ExpressionType = "bas_code"
	OPERATOR ExpressionType = "operator"
	FIELD    ExpressionType = "field"
	CONSTANT ExpressionType = "constant"
)

type Expression struct {
	Type ExpressionType `json:"type"`

	Key string `json:"key,omitempty"`

	Value *float64 `json:"value,omitempty"`

	Op string `json:"op,omitempty"`

	Left  *Expression `json:"left,omitempty"`
	Right *Expression `json:"right,omitempty"`
}
