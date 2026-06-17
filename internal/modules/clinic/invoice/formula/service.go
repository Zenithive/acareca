package formula

import "fmt"

// allowedFields returns the field keys that are referenced by any
// *FieldNode in the AST. Callers can use this to pre-populate the
// Context with default values (e.g. zero) for field nodes that are
// not user-managed (like BAS codes or section totals).
func allowedFields(evaluator Evaluator) []string {
	var fields []string
	walkFields(evaluator, &fields)
	return fields
}

func walkFields(node Evaluator, fields *[]string) {
	switch n := node.(type) {
	case *FieldNode:
		*fields = append(*fields, n.Key)
	case *AddNode:
		walkFields(n.Left, fields)
		walkFields(n.Right, fields)
	case *SubtractNode:
		walkFields(n.Left, fields)
		walkFields(n.Right, fields)
	case *MultiplyNode:
		walkFields(n.Left, fields)
		walkFields(n.Right, fields)
	case *DivideNode:
		walkFields(n.Left, fields)
		walkFields(n.Right, fields)
	}
}

func BuildFormula(ctx Context, json []byte) (float64, error) {
	// Parse (JSON → AST)
	parser := NewJSONParser()
	evaluator, err := parser.Parse(json)
	if err != nil {
		return 0, fmt.Errorf("Parse failed: %v", err)
	}
	if evaluator == nil {
		return 0, fmt.Errorf("Parse returned nil evaluator (AST root)")
	}

	// Validate (AST checks)
	validator := NewValidator(allowedFields(evaluator))
	if err := validator.Validate(evaluator); err != nil {
		return 0, fmt.Errorf("Validate failed: %v", err)
	}

	// Evaluate (walk AST → compute result)
	value, err := evaluator.Evaluate(ctx)
	if err != nil {
		return 0, err
	}

	return value, nil
}
