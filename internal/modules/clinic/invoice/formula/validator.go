package formula

import (
	"errors"
	"fmt"
)

var (
	ErrNilNode         = errors.New("node is nil")
	ErrInvalidField    = errors.New("invalid field")
	ErrDivisionByZero  = errors.New("division by zero")
	ErrUnsupportedNode = errors.New("unsupported node")
	ErrMissingOperand  = errors.New("missing operand")
)

type Validator interface {
	Validate(node Evaluator) error
}

type ASTValidator struct {
	allowedFields map[string]struct{}
}

func NewValidator(fields []string) Validator {
	allowed := make(map[string]struct{}, len(fields))

	for _, field := range fields {
		allowed[field] = struct{}{}
	}

	return &ASTValidator{
		allowedFields: allowed,
	}
}

func (v *ASTValidator) validateBinary(left Evaluator, right Evaluator) error {

	if left == nil || right == nil {
		return ErrMissingOperand
	}

	if err := v.validateNode(left); err != nil {
		return err
	}

	if err := v.validateNode(right); err != nil {
		return err
	}

	return nil
}

func (v *ASTValidator) validateNode(node Evaluator) error {
	switch n := node.(type) {

	case *ConstantNode:
		return nil

	case *FieldNode:
		if _, ok := v.allowedFields[n.Key]; !ok {
			return fmt.Errorf("%w: %s", ErrInvalidField, n.Key)
		}

		return nil

	case *BasCodeNode:
		if _, ok := v.allowedFields[n.Key]; !ok {
			return fmt.Errorf("%w: %s", ErrInvalidField, n.Key)
		}

		return nil

	case *AddNode:
		return v.validateBinary(n.Left, n.Right)

	case *SubtractNode:
		return v.validateBinary(n.Left, n.Right)

	case *MultiplyNode:
		return v.validateBinary(n.Left, n.Right)

	case *DivideNode:

		if err := v.validateBinary(n.Left, n.Right); err != nil {
			return err
		}

		// detect: A / 0
		if c, ok := n.Right.(*ConstantNode); ok {
			if c.Value == 0 {
				return ErrDivisionByZero
			}
		}

		return nil

	default:
		return ErrUnsupportedNode
	}
}

// Validate implements [Validator].
func (a *ASTValidator) Validate(node Evaluator) error {
	if node == nil {
		return ErrNilNode
	}

	return a.validateNode(node)
}
