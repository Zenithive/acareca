package method

import (
	"context"
	"errors"
	"fmt"
)

type IService interface {
	Calculate(ctx context.Context, taxType TaxTreatment, input *Input) (*Result, error)
}

type service struct {
}

func NewService() IService {
	return &service{}
}

func (s *service) Calculate(ctx context.Context, taxType TaxTreatment, input *Input) (*Result, error) {
	switch taxType {
	case TaxTreatmentInclusive:
		return s.inclusive(ctx, input)
	case TaxTreatmentExclusive:
		return s.exclusive(ctx, input)
	case TaxTreatmentManual:
		return s.manual(ctx, input)
	case TaxTreatmentZero:
		return s.zero(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported tax treatment: %v", taxType)
	}
}

func (s *service) exclusive(_ context.Context, input *Input) (*Result, error) {
	gstAmount := input.Amount * 0.10 // 10% GST
	return &Result{
		Amount:      input.Amount,
		GstAmount:   gstAmount,
		TotalAmount: input.Amount + gstAmount,
	}, nil
}

func (s *service) inclusive(_ context.Context, input *Input) (*Result, error) {
	gstAmount := input.Amount / 11
	baseAmount := input.Amount - gstAmount

	return &Result{
		Amount:      baseAmount,
		GstAmount:   gstAmount,
		TotalAmount: input.Amount,
	}, nil
}

func (s *service) manual(_ context.Context, input *Input) (*Result, error) {
	if input.GstAmount == nil {
		return nil, errors.New("gst amount is required")
	}
	gstAmount := *input.GstAmount
	return &Result{
		Amount:      input.Amount,
		GstAmount:   gstAmount,
		TotalAmount: input.Amount + gstAmount,
	}, nil
}

func (s *service) zero(_ context.Context, input *Input) (*Result, error) {
	return &Result{
		Amount:      input.Amount,
		GstAmount:   0,
		TotalAmount: input.Amount,
	}, nil
}
