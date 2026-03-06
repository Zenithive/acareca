package calculation

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
)

type Service interface {
	NetResult(ctx context.Context, entry *Entry) (*Result, error)
	GrossResult(ctx context.Context, entry *Entry) (*GrossResult, error)
	TaxCalculate(ctx context.Context, taxType method.TaxTreatment, input *Input) (*method.Result, error)
}

type service struct {
	repo   Repository
	method method.IService
}

func NewService(repo Repository, method method.IService) Service {
	return &service{repo: repo, method: method}
}

func (s *service) TaxCalculate(ctx context.Context, taxType method.TaxTreatment, input *Input) (*method.Result, error) {
	return s.method.Calculate(ctx, taxType, &method.Input{
		Amount:    input.Value,
		GstAmount: input.TaxValue,
	})
}

func (s *service) calcInputs(ctx context.Context, inputs []Input, label string) (totals []float64, sum float64, results []*method.Result, err error) {
	totals = make([]float64, 0, len(inputs))
	results = make([]*method.Result, 0, len(inputs))
	for i := range inputs {
		res, e := s.TaxCalculate(ctx, inputs[i].TaxType, &inputs[i])
		if e != nil {
			return nil, 0, nil, fmt.Errorf("calculate %s[%d]: %w", label, i, e)
		}
		totals = append(totals, res.TotalAmount)
		sum += res.TotalAmount
		results = append(results, res)
	}
	return
}

func (s *service) NetResult(ctx context.Context, entry *Entry) (*Result, error) {
	incomeTotals, incomeSum, _, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}
	expenseTotals, expenseSum, _, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}
	return &Result{
		Income:  incomeTotals,
		Expense: expenseTotals,
		Result:  incomeSum - expenseSum,
	}, nil
}

func (s *service) GrossResult(ctx context.Context, entry *Entry) (*GrossResult, error) {
	_, grossPatientFees, _, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}

	_, _, expResults, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}
	var labBase, clinicExpenseGST float64
	for i, exp := range entry.Expense {
		if exp.PaidBy != nil && *exp.PaidBy == PaidByClinic {
			labBase += expResults[i].Amount
			clinicExpenseGST += expResults[i].GstAmount
		}
	}

	_, otherCostsSum, _, err := s.calcInputs(ctx, entry.OtherCosts, "other_costs")
	if err != nil {
		return nil, err
	}

	otherCostsSum += clinicExpenseGST

	netAmount := grossPatientFees - labBase
	serviceFee := netAmount * (*entry.ClinicShare / 100)
	gstServiceFee := serviceFee * 0.1
	totalServiceFee := serviceFee + gstServiceFee

	return &GrossResult{
		NetAmount:       netAmount,
		ServiceFee:      serviceFee,
		GstServiceFee:   gstServiceFee,
		TotalServiceFee: totalServiceFee,
		RemittedAmount:  netAmount - totalServiceFee - otherCostsSum,
	}, nil
}
