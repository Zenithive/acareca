package calculation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/clinic"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	OutWorkResult(ctx context.Context, entry *Entry) (*OutWorkResult, error)

	GrossMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue) (*GrossResult, error)
	NetMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, filter *NetFilter) (*NetResult, error)

	Calculate(ctx context.Context, formId uuid.UUID, filter *NetFilter) (interface{}, error)
}

type service struct {
	method     method.IService
	clinicSrv  clinic.Service
	formSvc    form.IService
	versionSvc version.IService
	fieldSvc   field.IService
	entries    entry.IService
}

// OutWorkResult implements [Service].

func NewService(method method.IService, clinicSvc clinic.Service, formSvc form.IService, versionSvc version.IService, fieldSvc field.IService, entries entry.IService) Service {
	return &service{method: method, clinicSrv: clinicSvc, formSvc: formSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, entries: entries}
}

func (s *service) OutWorkResult(ctx context.Context, entry *Entry) (*OutWorkResult, error) {
	panic("unimplemented")
}

// OutWorkResult implements [Service].
// func (s *service) OutWorkResult(ctx context.Context, entry *Entry) (*OutWorkResult, error) {
// 	_, _, incResults, err := s.calcInputs(ctx, entry.Income, "income")
// 	if err != nil {
// 		return nil, err
// 	}
// 	incomeSum := 0.0
// 	for _, r := range incResults {
// 		incomeSum += r.Amount
// 	}
// 	incomeGST := 0.0
// 	for _, r := range incResults {
// 		incomeGST += r.GstAmount
// 	}
// 	incomeSum -= incomeGST

// 	_, _, expenseResults, err := s.calcInputs(ctx, entry.Expense, "expense")
// 	if err != nil {
// 		return nil, err
// 	}
// 	expenseGST := 0.0
// 	for i, exp := range entry.Expense {
// 		if exp.PaidBy == nil {
// 			continue
// 		}
// 		switch *exp.PaidBy {
// 		case PaidByClinic:
// 			expenseGST += expenseResults[i].GstAmount
// 		}
// 	}

// 	expenseSum := 0.0
// 	for _, r := range expenseResults {
// 		expenseSum += r.Amount
// 	}
// 	expenseSum -= expenseGST

// 	_, otherCostsSum, _, err := s.calcInputs(ctx, entry.OtherCosts, "other_costs")
// 	if err != nil {
// 		return nil, err
// 	}
// 	otherCostsSum += expenseSum

// 	clinicShare := 0.0
// 	if entry.ClinicShare != nil {
// 		clinicShare = *entry.ClinicShare
// 	}

// 	outWorkPercentage := 0.0
// 	if entry.OutWorkPercentage != nil {
// 		outWorkPercentage = *entry.OutWorkPercentage
// 	}

// 	serviceFee := incomeSum * (clinicShare / 100)
// 	outWorkAmount := otherCostsSum * (outWorkPercentage / 100)
// 	outServiceFee := outWorkAmount + serviceFee
// 	gstOutServiceFee := outServiceFee * 0.1
// 	totalOutServiceFee := outServiceFee + gstOutServiceFee

// 	return &OutWorkResult{
// 		NetAmount:       util.Round(incomeSum-otherCostsSum, 2),
// 		ServiceFee:      util.Round(outServiceFee, 2),
// 		GstServiceFee:   util.Round(gstOutServiceFee, 2),
// 		TotalServiceFee: util.Round(totalOutServiceFee, 2),
// 		NetPayable:      util.Round(incomeSum-totalOutServiceFee, 2),
// 	}, nil
// }

func (s *service) GrossMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue) (*GrossResult, error) {
	var (
		incomeSum    float64
		incomeGST    float64
		expenseSum   float64
		expenseGST   float64
		otherCostSum float64

		paidByOwnerSum float64
	)

	for _, v := range formValue {
		field, err := s.fieldSvc.GetByID(ctx, v.FormFieldID)
		if err != nil {
			return nil, err
		}

		switch field.SectionType {

		case "COLLECTION":
			if v.NetAmount != nil {
				incomeSum += *v.NetAmount
			}
			if v.GstAmount != nil {
				incomeGST += *v.GstAmount
			}

		case "COST":
			if field.PaymentResponsibility == nil {
				continue
			}

			switch *field.PaymentResponsibility {

			case "CLINIC":
				if v.GrossAmount != nil {
					expenseSum += *v.GrossAmount
				}
				if v.GstAmount != nil {
					expenseGST += *v.GstAmount
				}

			case "OWNER":
				if v.GrossAmount != nil {
					expenseSum += *v.GrossAmount
					paidByOwnerSum += *v.GrossAmount
				}
			}

		case "OTHER_COST":
			if v.NetAmount != nil {
				otherCostSum += *v.NetAmount
			}
		}
	}

	// Deduct GST from gross income to get net income
	netIncome := incomeSum - incomeGST

	// Deduct expenses (excluding owner-paid, which is passed through) to get net amount
	netAmount := netIncome - (expenseSum - paidByOwnerSum)

	clinicShare := float64(formDetail.ClinicShare)
	serviceFee := netAmount * (clinicShare / 100)
	gstServiceFee := serviceFee * 0.1
	totalServiceFee := serviceFee + gstServiceFee

	remittedAmount := netAmount - totalServiceFee - otherCostSum + incomeGST + paidByOwnerSum

	return &GrossResult{
		NetAmount:       util.Round(netAmount, 2),
		ServiceFee:      util.Round(serviceFee, 2),
		GstServiceFee:   util.Round(gstServiceFee, 2),
		TotalServiceFee: util.Round(totalServiceFee, 2),
		RemittedAmount:  util.Round(remittedAmount, 2),
	}, nil
}

func (s *service) NetMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, filter *NetFilter) (*NetResult, error) {
	var (
		incomeSum  float64
		expenseSum float64
	)

	for _, v := range formValue {
		field, err := s.fieldSvc.GetByID(ctx, v.FormFieldID)
		if err != nil {
			return nil, err
		}

		switch field.SectionType {

		case "COLLECTION":
			if v.NetAmount != nil {
				incomeSum += *v.NetAmount
			}

		case "COST":
			if v.NetAmount != nil {
				expenseSum += *v.NetAmount
			}
			if v.GstAmount != nil {
				expenseSum += *v.GstAmount
			}
		}
	}

	netAmount := incomeSum - expenseSum
	ownerShare := float64(formDetail.OwnerShare)

	superDecimal := 0.0
	if filter != nil && filter.SuperComponent != nil {
		superDecimal = *filter.SuperComponent / 100
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

// Calculate implements [Service].
func (s *service) Calculate(ctx context.Context, formID uuid.UUID, filter *NetFilter) (interface{}, error) {

	form, err := s.formSvc.GetFormByID(ctx, formID)
	if err != nil {
		return nil, err
	}
	version, err := s.versionSvc.GetVersionByFormID(ctx, form.ID)
	if err != nil {
		return nil, err
	}
	entries, err := s.entries.GetByVersionID(ctx, version.Id)
	if err != nil {
		return nil, err
	}
	switch Method(form.Method) {
	case IndependentContractor:
		return s.NetMethod(ctx, form, entries.Values, filter)
	case ServiceFee:
		return s.GrossMethod(ctx, form, entries.Values)
	default:
		return nil, fmt.Errorf("unsupported method: %s", form.Method)
	}
}
