package formula

import (
	"encoding/json"
	"fmt"
)

type Parser interface {
	Parse(data []byte) (Evaluator, error)
}

type JSONParser struct{}

func NewJSONParser() Parser {
	return &JSONParser{}
}

func (p *JSONParser) buildNode(exp *Expression) (Evaluator, error) {
	if exp == nil {
		return nil, ErrMissingOperand
	}

	switch exp.Type {

	case CONSTANT:
		if exp.Value == nil {
			return nil, fmt.Errorf("constant value required")
		}

		return &ConstantNode{
			Value: *exp.Value,
		}, nil

	case FIELD:
		return &FieldNode{
			Key: exp.Key,
		}, nil

	case BASCODE:
		if exp.Key == "" {
			return nil, fmt.Errorf("bas code key required")
		}

		return &BasCodeNode{
			Key: exp.Key,
		}, nil

	case OPERATOR:

		left, err := p.buildNode(exp.Left)
		if err != nil {
			return nil, fmt.Errorf("left operand: %w", err)
		}

		right, err := p.buildNode(exp.Right)
		if err != nil {
			return nil, fmt.Errorf("right operand: %w", err)
		}

		switch exp.Op {

		case "+":
			return &AddNode{
				Left:  left,
				Right: right,
			}, nil

		case "-":
			return &SubtractNode{
				Left:  left,
				Right: right,
			}, nil

		case "*":
			return &MultiplyNode{
				Left:  left,
				Right: right,
			}, nil

		case "/":
			return &DivideNode{
				Left:  left,
				Right: right,
			}, nil

		default:
			return nil, fmt.Errorf("unsupported operator: %s", exp.Op)
		}

	default:
		return nil, fmt.Errorf("unsupported type: %s", exp.Type)
	}
}

func (p *JSONParser) Parse(data []byte) (Evaluator, error) {
	var dto Expression

	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, err
	}

	return p.buildNode(&dto)
}
