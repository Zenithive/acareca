package formula

import (
	"context"
	"fmt"
)

type Context struct {
	Context context.Context
	Values  map[string]float64
}

type Evaluator interface {
	Evaluate(ctx Context) (float64, error)
}

type ConstantNode struct {
	Value float64
}

type FieldNode struct {
	Key string
}

type BasCodeNode struct {
	Key string
}

type AddNode struct {
	Left  Evaluator
	Right Evaluator
}

type SubtractNode struct {
	Left  Evaluator
	Right Evaluator
}

type MultiplyNode struct {
	Left  Evaluator
	Right Evaluator
}

type DivideNode struct {
	Left  Evaluator
	Right Evaluator
}

func (n *ConstantNode) Evaluate(ctx Context) (float64, error) {
	return n.Value, nil
}

func (n *FieldNode) Evaluate(ctx Context) (float64, error) {
	value, ok := ctx.Values[n.Key]
	if !ok {
		return 0, fmt.Errorf("field %s not found", n.Key)
	}
	return value, nil
}

func (n *BasCodeNode) Evaluate(ctx Context) (float64, error) {
	value, ok := ctx.Values[n.Key]
	if !ok {
		return 0, fmt.Errorf("BAS code %s not found in context values", n.Key)
	}
	return value, nil
}

func (n *AddNode) Evaluate(ctx Context) (float64, error) {
	l, err := n.Left.Evaluate(ctx)
	if err != nil {
		return 0, err
	}
	r, err := n.Right.Evaluate(ctx)
	if err != nil {
		return 0, err
	}

	return l + r, nil
}

func (n *SubtractNode) Evaluate(ctx Context) (float64, error) {
	l, err := n.Left.Evaluate(ctx)
	if err != nil {
		return 0, err
	}
	r, err := n.Right.Evaluate(ctx)
	if err != nil {
		return 0, err
	}

	return l - r, nil
}

func (n *MultiplyNode) Evaluate(ctx Context) (float64, error) {
	l, err := n.Left.Evaluate(ctx)
	if err != nil {
		return 0, err
	}
	r, err := n.Right.Evaluate(ctx)
	if err != nil {
		return 0, err
	}

	return l * r, nil
}

func (n *DivideNode) Evaluate(ctx Context) (float64, error) {
	l, err := n.Left.Evaluate(ctx)
	if err != nil {
		return 0, err
	}
	r, err := n.Right.Evaluate(ctx)
	if err != nil {
		return 0, err
	}

	if r == 0 {
		return 0, ErrDivisionByZero
	}

	return l / r, nil
}
