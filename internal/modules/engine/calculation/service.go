package calculation

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	NetAmount(ctx context.Context, entry *Entry) (*NetAmountResult, error)
	NetResult(ctx context.Context, entry *Entry) (*NetResult, error)
	GrossResult(ctx context.Context, entry *Entry) (*GrossResult, error)
	OutWorkResult(ctx context.Context, entry *Entry) (*OutWorkResult, error)
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
	if taxType == method.TaxTreatmentManual && input.TaxValue != nil {
		return s.method.Calculate(ctx, taxType, &method.Input{
			Amount:    input.Value - *input.TaxValue,
			GstAmount: input.TaxValue,
		})
	}

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

func (s *service) NetAmount(ctx context.Context, entry *Entry) (*NetAmountResult, error) {
	_, incomeSum, _, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}

	_, expenseSum, _, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}

	return &NetAmountResult{
		Income:  []float64{util.Round(incomeSum, 2)},
		Expense: []float64{util.Round(expenseSum, 2)},
		Result:  util.Round(incomeSum-expenseSum, 2),
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

// NetResult implements [Service].
func (s *service) NetResult(ctx context.Context, entry *Entry) (*NetResult, error) {
	_, _, incResults, err := s.calcInputs(ctx, entry.Income, "income")
	if err != nil {
		return nil, err
	}
	var incomeSum, expenseSum float64
	for _, r := range incResults {
		incomeSum += r.Amount
	}

	_, _, expResults, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}
	for _, r := range expResults {
		expenseSum += r.Amount
	}

	netAmount := incomeSum - expenseSum

	ownerShare := 0.0
	if entry.OwnerShare != nil {
		ownerShare = *entry.OwnerShare
	}

	superDecimal := 0.0
	if entry.SuperComponent != nil {
		superDecimal = *entry.SuperComponent / 100
	}

	totalRemuneration := netAmount * (ownerShare / 100)

	commissionBase := totalRemuneration
	var superAmount float64
	if superDecimal > 0 {
		commissionBase = totalRemuneration / (1 + superDecimal)
		superAmount = commissionBase * superDecimal
	}

	gstCommission := commissionBase * 0.10
	totalCommission := commissionBase + gstCommission

	netResult := NetResult{
		NetAmount:       util.Round(netAmount, 2),
		Commission:      util.Round(totalRemuneration, 2),
		GstCommission:   util.Round(gstCommission, 2),
		TotalCommission: util.Round(totalCommission, 2),
	}

	if superDecimal > 0 {
		sa := util.Round(superAmount, 2)
		netResult.SuperComponent = &sa

		cb := util.Round(commissionBase, 2)
		netResult.SuperComponentCommission = &cb
	}

	tr := util.Round(totalRemuneration, 2)
	netResult.TotalRemuneration = &tr

	return &netResult, nil
}

// OutWorkResult implements [Service].
func (s *service) OutWorkResult(ctx context.Context, entry *Entry) (*OutWorkResult, error) {
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

	_, _, expenseResults, err := s.calcInputs(ctx, entry.Expense, "expense")
	if err != nil {
		return nil, err
	}
	expenseGST := 0.0
	for i, exp := range entry.Expense {
		if exp.PaidBy == nil {
			continue
		}
		switch *exp.PaidBy {
		case PaidByClinic:
			expenseGST += expenseResults[i].GstAmount
		}
	}

	expenseSum := 0.0
	for _, r := range expenseResults {
		expenseSum += r.Amount
	}
	expenseSum -= expenseGST

	_, otherCostsSum, _, err := s.calcInputs(ctx, entry.OtherCosts, "other_costs")
	if err != nil {
		return nil, err
	}
	otherCostsSum += expenseSum

	clinicShare := 0.0
	if entry.ClinicShare != nil {
		clinicShare = *entry.ClinicShare
	}

	outWorkPercentage := 0.0
	if entry.OutWorkPercentage != nil {
		outWorkPercentage = *entry.OutWorkPercentage
	}

	serviceFee := incomeSum * (clinicShare / 100)
	outWorkAmount := otherCostsSum * (outWorkPercentage / 100)
	outServiceFee := outWorkAmount + serviceFee
	gstOutServiceFee := outServiceFee * 0.1
	totalOutServiceFee := outServiceFee + gstOutServiceFee

	return &OutWorkResult{
		NetAmount:       util.Round(incomeSum-otherCostsSum, 2),
		ServiceFee:      util.Round(outServiceFee, 2),
		GstServiceFee:   util.Round(gstOutServiceFee, 2),
		TotalServiceFee: util.Round(totalOutServiceFee, 2),
		NetPayable:      util.Round(incomeSum-totalOutServiceFee, 2),
	}, nil
}
