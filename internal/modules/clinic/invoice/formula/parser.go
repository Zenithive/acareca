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

func (p *JSONParser) buildNode(dto *Expression) (Evaluator, error) {
	if dto == nil {
		return nil, ErrMissingOperand
	}

	switch dto.Type {

	case "constant":
		if dto.Value == nil {
			return nil, fmt.Errorf("constant value required")
		}

		return &ConstantNode{
			Value: *dto.Value,
		}, nil

	case "field":
		return &FieldNode{
			Key: dto.Key,
		}, nil

	case "operator":

		left, err := p.buildNode(dto.Left)
		if err != nil {
			return nil, fmt.Errorf("left operand: %w", err)
		}

		right, err := p.buildNode(dto.Right)
		if err != nil {
			return nil, fmt.Errorf("right operand: %w", err)
		}

		switch dto.Op {

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
			return nil, fmt.Errorf("unsupported operator: %s", dto.Op)
		}

	default:
		return nil, fmt.Errorf("unsupported type: %s", dto.Type)
	}
}

func (p *JSONParser) Parse(data []byte) (Evaluator, error) {
	var dto Expression

	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, err
	}

	return p.buildNode(&dto)
}
