package calculation

import (
	"context"
	"fmt"
	"maps"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/form"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	GrossMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField) (*GrossResult, error)
	NetMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField, filter *NetFilter) (*NetResult, error)
	Calculate(ctx context.Context, formId uuid.UUID, filter *NetFilter) (interface{}, error)
	CalculateFromEntries(ctx context.Context, req *RqCalculateFromEntries) (interface{}, error)
	FormulaCalculate(ctx context.Context, formID uuid.UUID, req *RqFormulaCalculate) (*RsFormulaCalculate, error)
	LiveCalculate(ctx context.Context, req *RqLiveCalculate) (*RsLiveCalculate, error)
}

type service struct {
	formSvc    form.IService
	versionSvc version.IService
	fieldSvc   field.IService
	entries    entry.IService
	formulaSvc formula.IService
	methodSvc  method.IService
}

func NewService(formSvc form.IService, versionSvc version.IService, fieldSvc field.IService, entries entry.IService) Service {
	return &service{
		formSvc:    formSvc,
		versionSvc: versionSvc,
		fieldSvc:   fieldSvc,
		entries:    entries,
		methodSvc:  method.NewService(),
	}
}

// NewServiceWithFormula constructs the service with formula and method support.
func NewServiceWithFormula(formSvc form.IService, versionSvc version.IService, fieldSvc field.IService, entries entry.IService, formulaSvc formula.IService) Service {
	return &service{
		formSvc:    formSvc,
		versionSvc: versionSvc,
		fieldSvc:   fieldSvc,
		entries:    entries,
		formulaSvc: formulaSvc,
		methodSvc:  method.NewService(),
	}
}

func (s *service) GrossMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField) (*GrossResult, error) {
	var (
		incomeSum    float64
		incomeGST    float64
		expenseSum   float64
		expenseGST   float64
		otherCostSum float64

		paidByOwnerSum float64
	)

	for _, v := range formValue {
		f, ok := fieldMap[v.FormFieldID]
		if !ok {
			return nil, fmt.Errorf("field %s not found", v.FormFieldID)
		}
		if f.SectionType == nil {
			continue
		}

		switch *f.SectionType {

		case "COLLECTION":
			if v.NetAmount != nil {
				incomeSum += *v.NetAmount
			}
			if f.TaxType != nil && *f.TaxType == field.TaxTypeManual && v.GstAmount != nil {
				incomeGST += *v.GstAmount
			}

		case "COST":
			if f.PaymentResponsibility == nil {
				continue
			}

			switch *f.PaymentResponsibility {

			case "CLINIC":
				if v.NetAmount != nil {
					expenseSum += *v.NetAmount
				}
				if v.GstAmount != nil {
					expenseGST += *v.GstAmount
				}

			case "OWNER":
				if v.NetAmount != nil {
					expenseSum += *v.NetAmount
					paidByOwnerSum += *v.NetAmount
				}
			}

		case "OTHER_COST":
			if v.NetAmount != nil {
				otherCostSum += *v.NetAmount
			}
		}
	}

	netIncome := incomeSum

	netAmount := netIncome - expenseSum

	clinicShare := float64(formDetail.ClinicShare)
	serviceFee := netAmount * (clinicShare / 100)
	gstServiceFee := serviceFee * 0.1
	totalServiceFee := serviceFee + gstServiceFee

	remittedAmount := netAmount - totalServiceFee - otherCostSum + paidByOwnerSum + incomeGST

	return &GrossResult{
		NetAmount:        util.Round(netAmount, 2),
		ServiceFee:       util.Round(serviceFee, 2),
		GstServiceFee:    util.Round(gstServiceFee, 2),
		TotalServiceFee:  util.Round(totalServiceFee, 2),
		RemittedAmount:   util.Round(remittedAmount, 2),
		ClinicExpenseGST: util.Round(expenseGST, 2),
	}, nil
}

func (s *service) NetMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField, filter *NetFilter) (*NetResult, error) {
	var (
		incomeSum    float64
		expenseSum   float64
		otherCostSum float64
	)

	for _, v := range formValue {
		f, ok := fieldMap[v.FormFieldID]
		if !ok {
			return nil, fmt.Errorf("field %s not found", v.FormFieldID)
		}
		if f.SectionType == nil {
			continue
		}

		switch *f.SectionType {

		case "COLLECTION":
			if v.NetAmount != nil {
				incomeSum += *v.NetAmount
			}

		case "COST":
			if v.NetAmount != nil {
				expenseSum += *v.NetAmount
			}

		case "OTHER_COST":
			if v.NetAmount != nil {
				otherCostSum += *v.NetAmount
			}
		}
	}

	netAmount := incomeSum - expenseSum - otherCostSum

	ownerShare := float64(formDetail.OwnerShare)

	superDecimal := 0.0
	if filter != nil && filter.SuperComponent != nil {
		superDecimal = *filter.SuperComponent / 100
	}

	totalRemuneration := netAmount * (ownerShare / 100)

	commissionBase := totalRemuneration
	var superAmount float64
	if superDecimal > 0 {
		superAmount = commissionBase * superDecimal
	}

	gstOnRemuneration := commissionBase * 0.10
	invoiceTotal := commissionBase + gstOnRemuneration + superAmount

	netResult := NetResult{
		NetAmount:          util.Round(netAmount, 2),
		TotalRemuneration:  util.Round(totalRemuneration, 2),
		GstOnRemuneration:  util.Round(gstOnRemuneration, 2),
		InvoiceTotal:       util.Round(invoiceTotal, 2),
		OtherCostDeduction: util.Round(otherCostSum, 2),
	}

	if superDecimal > 0 {
		sa := util.Round(superAmount, 2)
		netResult.SuperComponent = &sa

		br := util.Round(commissionBase, 2)
		netResult.BaseRemuneration = &br
	}

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
	fieldMap, err := s.fieldSvc.GetFieldMap(ctx, version.Id)
	if err != nil {
		return nil, err
	}
	switch Method(form.Method) {
	case IndependentContractor:
		return s.NetMethod(ctx, form, entries.Values, fieldMap, filter)
	case ServiceFee:
		return s.GrossMethod(ctx, form, entries.Values, fieldMap)
	default:
		return nil, fmt.Errorf("unsupported method: %s", form.Method)
	}
}

// CalculateFromEntries implements [Service].
func (s *service) CalculateFromEntries(ctx context.Context, req *RqCalculateFromEntries) (interface{}, error) {
	formID, err := uuid.Parse(req.FormID)
	if err != nil {
		return nil, fmt.Errorf("invalid form_id: %w", err)
	}

	form, err := s.formSvc.GetFormByID(ctx, formID)
	if err != nil {
		return nil, err
	}

	version, err := s.versionSvc.GetVersionByFormID(ctx, form.ID)
	if err != nil {
		return nil, err
	}

	fieldMap, err := s.fieldSvc.GetFieldMap(ctx, version.Id)
	if err != nil {
		return nil, err
	}

	filter := &NetFilter{SuperComponent: req.SuperComponent}

	switch Method(form.Method) {
	case IndependentContractor:
		return s.NetMethod(ctx, form, req.Entries, fieldMap, filter)
	case ServiceFee:
		return s.GrossMethod(ctx, form, req.Entries, fieldMap)
	default:
		return nil, fmt.Errorf("unsupported method: %s", form.Method)
	}
}

// FormulaCalculate evaluates all is_computed=true fields for a form using the
// provided manual field key→amount values, and returns per-field results with
// net/gst/gross breakdown when the field has a tax_type.
func (s *service) FormulaCalculate(ctx context.Context, formID uuid.UUID, req *RqFormulaCalculate) (*RsFormulaCalculate, error) {
	// Resolve active version for the form.
	ver, err := s.versionSvc.GetVersionByFormID(ctx, formID)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}

	// Fetch all fields so we can look up label, tax_type, is_computed, and key.
	fieldMap, err := s.fieldSvc.GetFieldMap(ctx, ver.Id)
	if err != nil {
		return nil, fmt.Errorf("get field map: %w", err)
	}

	// Build tax type map so computed fields with GST feed gross into downstream formulas.
	taxTypeByKey := make(map[string]string, len(fieldMap))
	for _, f := range fieldMap {
		if f.IsComputed && f.TaxType != nil && *f.TaxType != "" {
			taxTypeByKey[f.FieldKey] = *f.TaxType
		}
	}

	// Compute section totals from keyValues (which contain NET amounts)
	sectionTotals := make(map[string]float64)
	for _, f := range fieldMap {
		if f.IsComputed || f.SectionType == nil || *f.SectionType == "" {
			continue
		}
		val, ok := req.Values[f.FieldKey]
		if !ok {
			continue
		}
		sectionKey := "SECTION:" + *f.SectionType
		sectionTotals[sectionKey] += val
	}

	// Merge section totals into req.Values
	maps.Copy(req.Values, sectionTotals)

	// Evaluate all formulas in topological order.
	computed, err := s.formulaSvc.EvalFormulas(ctx, ver.Id, req.Values, taxTypeByKey)
	if err != nil {
		return nil, fmt.Errorf("eval formulas: %w", err)
	}

	results := make([]RsComputedFieldValue, 0, len(computed))
	for fieldID, val := range computed {
		f, ok := fieldMap[fieldID]
		if !ok || !f.IsComputed {
			continue
		}

		netAmount := util.Round(val, 2)
		var gstAmount *float64
		var grossAmount *float64

		// Apply tax treatment when the computed field has a tax_type.
		if f.TaxType != nil && *f.TaxType != "" {
			taxResult, err := s.methodSvc.Calculate(ctx, method.TaxTreatment(*f.TaxType), &method.Input{Amount: val})
			if err != nil {
				return nil, fmt.Errorf("tax calc for field %s: %w", f.FieldKey, err)
			}
			net := util.Round(taxResult.Amount, 2)
			gst := util.Round(taxResult.GstAmount, 2)
			gross := util.Round(taxResult.TotalAmount, 2)

			netAmount = net
			gstAmount = &gst
			grossAmount = &gross

		}

		item := RsComputedFieldValue{
			FieldID:     fieldID,
			FormFieldID: fieldID.String(),
			FieldKey:    f.FieldKey,
			Label:       f.Label,
			IsComputed:  f.IsComputed,
			NetAmount:   netAmount,
			GstAmount:   gstAmount,
			GrossAmount: grossAmount,
			SectionType: f.SectionType,
			TaxType:     f.TaxType,
			CoaID:       f.CoaID,
			SortOrder:   f.SortOrder,
		}

		results = append(results, item)
	}

	return &RsFormulaCalculate{
		FormID:         formID,
		ComputedFields: results,
	}, nil
}

func (s *service) LiveCalculate(ctx context.Context, req *RqLiveCalculate) (*RsLiveCalculate, error) {
	formVersionID, err := uuid.Parse(req.FormVersionID)
	if err != nil {
		return nil, fmt.Errorf("invalid form_version_id: %w", err)
	}

	fieldMap, err := s.fieldSvc.GetFieldMap(ctx, formVersionID)
	if err != nil {
		return nil, fmt.Errorf("get field map: %w", err)
	}

	keyValues := make(map[string]float64)
	for _, f := range fieldMap {
		if !f.IsComputed {
			keyValues[f.FieldKey] = 0
		}
	}

	// Process each entry: calculate proper net/gst/gross based on field tax type
	// The frontend sends the ENTERED value as net_amount, but we need to interpret it correctly
	for _, entry := range req.Entries {
		fieldID, err := uuid.Parse(entry.FormFieldID)
		if err != nil {
			return nil, fmt.Errorf("invalid form_field_id %s: %w", entry.FormFieldID, err)
		}

		f, ok := fieldMap[fieldID]
		if !ok {
			return nil, fmt.Errorf("field %s not found in form version", entry.FormFieldID)
		}

		if f.IsComputed {
			continue
		}

		// Calculate actual net amount based on tax type
		actualNetAmount := entry.NetAmount

		if f.TaxType != nil && *f.TaxType != "" {
			taxType := method.TaxTreatment(*f.TaxType)
			switch taxType {
			case method.TaxTreatmentInclusive:
				// User entered GROSS amount (includes GST), extract NET
				// Example: entered 1000 → net = 1000/1.1 = 909.09
				taxResult, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: entry.NetAmount})
				if err != nil {
					return nil, fmt.Errorf("tax calc for field %s: %w", f.FieldKey, err)
				}
				if (f.SectionType) != nil && *f.SectionType == "OTHER_COST" {
					actualNetAmount = taxResult.TotalAmount
				} else {
					actualNetAmount = taxResult.Amount
				}

			case method.TaxTreatmentExclusive:
				taxResult, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: entry.NetAmount})
				if err != nil {
					return nil, fmt.Errorf("tax calc for field %s: %w", f.FieldKey, err)
				}
				if (f.SectionType) != nil && *f.SectionType == "OTHER_COST" {
					actualNetAmount = taxResult.TotalAmount
				} else {
					actualNetAmount = entry.NetAmount
				}
				// User entered NET amount, use as-is

			case method.TaxTreatmentManual:
				if (f.SectionType) != nil && *f.SectionType == "COLLECTION" {
					if entry.GstAmount != nil {
						actualNetAmount = entry.NetAmount - *entry.GstAmount
					}
				} else {
					actualNetAmount = entry.NetAmount
				}
			}
		}

		keyValues[f.FieldKey] = actualNetAmount
	}

	// Build tax type map so computed fields with GST feed gross into downstream formulas.
	taxTypeByKey := make(map[string]string, len(fieldMap))
	for _, f := range fieldMap {
		if f.IsComputed && f.TaxType != nil && *f.TaxType != "" {
			taxTypeByKey[f.FieldKey] = *f.TaxType
		}
	}

	// Compute section totals using corrected NET amounts
	sectionTotals := make(map[string]float64)
	for _, entry := range req.Entries {
		fieldID, err := uuid.Parse(entry.FormFieldID)
		if err != nil {
			continue
		}
		f, ok := fieldMap[fieldID]
		if !ok || f.IsComputed || f.SectionType == nil || *f.SectionType == "" {
			continue
		}

		// Calculate actual net amount based on tax type (same logic as above)
		actualNetAmount := entry.NetAmount
		if f.TaxType != nil && *f.TaxType != "" {
			taxType := method.TaxTreatment(*f.TaxType)
			if taxType == method.TaxTreatmentInclusive {
				// Extract net from gross (entered value)
				taxResult, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: entry.NetAmount})
				if err == nil {
					actualNetAmount = taxResult.Amount
				}
			}
		}

		sectionKey := "SECTION:" + *f.SectionType
		sectionTotals[sectionKey] += actualNetAmount
	}

	// Merge section totals into keyValues for formula evaluation
	for key, val := range sectionTotals {
		keyValues[key] = val
	}

	computed, err := s.formulaSvc.EvalFormulas(ctx, formVersionID, keyValues, taxTypeByKey)
	if err != nil {
		return nil, fmt.Errorf("eval formulas: %w", err)
	}

	results := make([]RsComputedFieldValue, 0, len(computed))
	for fieldID, val := range computed {
		f, ok := fieldMap[fieldID]
		if !ok || !f.IsComputed {
			continue
		}

		netAmount := util.Round(val, 2)
		var gstAmount *float64
		var grossAmount *float64

		if f.TaxType != nil && *f.TaxType != "" {
			taxResult, err := s.methodSvc.Calculate(ctx, method.TaxTreatment(*f.TaxType), &method.Input{Amount: val})
			if err != nil {
				return nil, fmt.Errorf("tax calc for field %s: %w", f.FieldKey, err)
			}
			net := util.Round(taxResult.Amount, 2)
			gst := util.Round(taxResult.GstAmount, 2)
			gross := util.Round(taxResult.TotalAmount, 2)
			netAmount = net
			gstAmount = &gst
			grossAmount = &gross
		}

		item := RsComputedFieldValue{
			FieldID:     fieldID,
			FormFieldID: fieldID.String(),
			FieldKey:    f.FieldKey,
			Label:       f.Label,
			IsComputed:  f.IsComputed,
			NetAmount:   netAmount,
			GstAmount:   gstAmount,
			GrossAmount: grossAmount,
			SectionType: f.SectionType,
			TaxType:     f.TaxType,
			CoaID:       f.CoaID,
			SortOrder:   f.SortOrder,
		}

		results = append(results, item)
	}

	return &RsLiveCalculate{
		FormVersionID:  formVersionID,
		ComputedFields: results,
	}, nil
}
