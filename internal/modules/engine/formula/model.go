package formula

import (
	"errors"

	"github.com/google/uuid"
)

type Formula struct {
	ID            uuid.UUID `db:"id"`
	FormVersionID uuid.UUID `db:"form_version_id"`
	FieldID       uuid.UUID `db:"field_id"`
	FieldKey      string    `db:"field_key"`
	Name          string    `db:"name"`
	CreatedAt     string    `db:"created_at"`
}

type FormulaNode struct {
	ID            uuid.UUID  `db:"id"`
	FormulaID     uuid.UUID  `db:"formula_id"`
	ParentID      *uuid.UUID `db:"parent_id"`
	NodeType      string     `db:"node_type"`
	Operator      *string    `db:"operator"`
	FieldID       *uuid.UUID `db:"field_id"`
	ConstantValue *float64   `db:"constant_value"`
	Position      *int16     `db:"position"`
	CreatedAt     string     `db:"created_at"`
}

type FormulaNodeWithKey struct {
	FormulaNode
	FieldKey *string `db:"field_key"`
}

type ExprNode struct {
	Type  string    `json:"type"`  // "operator" | "field" | "constant"
	Op    string    `json:"op"`    // "+", "-", "*", "/" — only for operator
	Key   string    `json:"key"`   // field key e.g. "A" — only for field
	Value *float64  `json:"value"` // numeric value — only for constant
	Left  *ExprNode `json:"left"`
	Right *ExprNode `json:"right"`
}

func (e *ExprNode) Validate() error {
	if e == nil {
		return errors.New("expression node is nil")
	}
	switch e.Type {
	case "operator":
		if e.Op == "" {
			return errors.New("operator node missing op")
		}
		if e.Left == nil || e.Right == nil {
			return errors.New("operator node must have left and right children")
		}
		if err := e.Left.Validate(); err != nil {
			return err
		}
		return e.Right.Validate()
	case "field":
		if e.Key == "" {
			return errors.New("field node missing key")
		}
	case "constant":
		if e.Value == nil {
			return errors.New("constant node missing value")
		}
	case "section":
		if e.Key == "" {
			return errors.New("section node missing key")
		}
	case "text":
		// TEXT nodes are always valid
	default:
		return errors.New("unknown node type: " + e.Type)
	}
	return nil
}

type RqFormula struct {
	FieldKey   string    `json:"field_key" validate:"required,max=5"`
	Name       string    `json:"name" validate:"required,max=255"`
	Expression *ExprNode `json:"expression" validate:"required"`
}

func (r *RqFormula) Validate() error {
	if r.Expression == nil {
		return errors.New("formula expression is required")
	}
	return r.Expression.Validate()
}

type RsFormula struct {
	ID            uuid.UUID `json:"id,omitempty"`
	FormVersionID uuid.UUID `json:"form_version_id,omitempty"`
	FieldID       uuid.UUID `json:"field_id,omitempty"`
	FieldKey      string    `json:"field_key"`
	Name          string    `json:"name"`
	Expression    *ExprNode `json:"expression,omitempty"`
	CreatedAt     string    `json:"created_at,omitempty"`
}
