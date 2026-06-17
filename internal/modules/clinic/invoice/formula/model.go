package formula

import (
	"time"

	"github.com/google/uuid"
)

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

type Formula struct {
	ID               uuid.UUID      `db:"id"`
	InvoiceID        uuid.UUID      `db:"invoice_id"`
	InvoiceSectionID uuid.UUID      `db:"invoice_section_id"`
	FieldKey         string         `db:"field_key"`
	FieldType        string         `db:"field_type"`
	Label            *string        `db:"label"`
	SortOrder        int            `db:"sort_order"`
	CreatedAt        time.Time      `db:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"`
	Nodes            []*FormulaNode `db:"-"`
}

type FormulaNode struct {
	ID                 uuid.UUID  `db:"id"`
	InvoiceFormulaID   uuid.UUID  `db:"invoice_formula_id"`
	ParentID           *uuid.UUID `db:"parent_id"`
	NodeType           string     `db:"node_type"`
	Operator           *string    `db:"operator"`
	ReferencedFieldKey *string    `db:"referenced_field_key"`
	ConstantValue      *float64   `db:"constant_value"`
	Position           int16      `db:"position"`
	CreatedAt          time.Time  `db:"created_at"`
}
