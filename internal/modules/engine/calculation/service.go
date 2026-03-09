package calculation

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
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
	_, _, incomeResults, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}

	incomeSum := 0.0
	for _, r := range incomeResults {
		incomeSum += r.Amount
	}
	incomeGST := 0.0
	for _, r := range incomeResults {
		incomeGST += r.GstAmount
	}
	incomeSum -= incomeGST

	_, expenseSum, _, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}
	return &Result{
		Income:  []float64{incomeSum},
		Expense: []float64{expenseSum},
		Result:  incomeSum - expenseSum,
	}, nil
}

func (s *service) GrossResult(ctx context.Context, entry *Entry) (*GrossResult, error) {
	_, _, incResults, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}
	incomeSum := 0.0
	for _, r := range incResults {
		incomeSum += r.Amount
	}
	incomeGST := 0.0
	for _, r := range incResults {
		incomeGST += r.GstAmount
	}
	incomeSum -= incomeGST

	_, _, expResults, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}

	expenseGST := 0.0
	expenseSum := 0.0
	paidByClinicSum := 0.0
	paidByOwnerSum := 0.0
	for i, exp := range entry.Expense {
		if exp.PaidBy == nil {
			continue
		}
		switch *exp.PaidBy {
		case PaidByClinic:
			paidByClinicSum += expResults[i].Amount
			expenseSum += expResults[i].Amount
			expenseGST += expResults[i].GstAmount
		case PaidByOwner:
			expenseSum += expResults[i].TotalAmount
			paidByOwnerSum += expenseSum
		}
	}

	_, otherCostsSum, _, err := s.calcInputs(ctx, entry.OtherCosts, "other_costs")
	if err != nil {
		return nil, err
	}
	otherCostsSum += expenseGST

	clinicShare := 0.0
	if entry.ClinicShare != nil {
		clinicShare = *entry.ClinicShare
	}

	netAmount := incomeSum - expenseSum
	serviceFee := netAmount * (clinicShare / 100)
	gstServiceFee := serviceFee * 0.1
	totalServiceFee := serviceFee + gstServiceFee

	remittedAmount := netAmount - totalServiceFee - otherCostsSum + incomeGST + paidByOwnerSum

	return &GrossResult{
		NetAmount:       util.Round(netAmount, 2),
		ServiceFee:      util.Round(serviceFee, 2),
		GstServiceFee:   util.Round(gstServiceFee, 2),
		TotalServiceFee: util.Round(totalServiceFee, 2),
		RemittedAmount:  util.Round(remittedAmount, 2),
	}, nil
}
