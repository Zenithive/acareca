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
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/modules/engine/method"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	GrossMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField) (*GrossResult, error)
	NetMethod(ctx context.Context, formDetail *detail.RsFormDetail, formValue []entry.RsEntryValue, fieldMap map[uuid.UUID]*field.RsFormField, filter *NetFilter) (*NetResult, error)
	Calculate(ctx context.Context, formId uuid.UUID, filter *NetFilter, actorID uuid.UUID, role string) (interface{}, error)
	CalculateFromEntries(ctx context.Context, req *RqCalculateFromEntries, actorID uuid.UUID, role string) (interface{}, error)
	FormulaCalculate(ctx context.Context, formID uuid.UUID, req *RqFormulaCalculate) (*RsFormulaCalculate, error)
	LiveCalculate(ctx context.Context, req *RqLiveCalculate) (*RsLiveCalculate, error)
	FormPreview(ctx context.Context, req *RqFormPreview, actorID uuid.UUID, role string) (*RsFormPreview, error)
	GetFormSummary(ctx context.Context, formID string, actorID uuid.UUID, role string) ([]*RsTransactionRow, error)
}

type service struct {
	repo       Repository
	formSvc    form.IService
	versionSvc version.IService
	fieldSvc   field.IService
	entries    entry.IService
	formulaSvc formula.IService
	methodSvc  method.IService
}

func NewService(repo Repository, formSvc form.IService, versionSvc version.IService, fieldSvc field.IService, entries entry.IService) Service {
	return &service{
		repo:       repo,
		formSvc:    formSvc,
		versionSvc: versionSvc,
		fieldSvc:   fieldSvc,
		entries:    entries,
		methodSvc:  method.NewService(),
	}
}

// NewServiceWithFormula constructs the service with formula and method support.
func NewServiceWithFormula(repo Repository, formSvc form.IService, versionSvc version.IService, fieldSvc field.IService, entries entry.IService, formulaSvc formula.IService) Service {
	return &service{
		repo:       repo,
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
func (s *service) Calculate(ctx context.Context, formID uuid.UUID, filter *NetFilter, actorID uuid.UUID, role string) (interface{}, error) {

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
func (s *service) CalculateFromEntries(ctx context.Context, req *RqCalculateFromEntries, actorID uuid.UUID, role string) (interface{}, error) {
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
	for k, v := range sectionTotals {
		req.Values[k] = v
	}

	// Evaluate all formulas in topological order.
	// Note: FormulaCalculate doesn't have access to manually entered GST amounts,
	// so we pass an empty map. This is acceptable for non-live calculations.
	manualGSTByKey := make(map[string]float64)
	computed, err := s.formulaSvc.EvalFormulas(ctx, ver.Id, req.Values, taxTypeByKey, manualGSTByKey)
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
					// if entry.GstAmount != nil {
					actualNetAmount = entry.NetAmount
					// }
				} else {
					// For MANUAL tax type, use GROSS amount if provided, otherwise use net
					actualNetAmount = entry.NetAmount
				}
			}

		}

		keyValues[f.FieldKey] = actualNetAmount
	}

	// Build tax type map so computed fields with GST feed gross into downstream formulas.
	taxTypeByKey := make(map[string]string, len(fieldMap))
	manualGSTByKey := make(map[string]float64, len(fieldMap))
	for _, f := range fieldMap {
		if f.IsComputed && f.TaxType != nil && *f.TaxType != "" {
			taxTypeByKey[f.FieldKey] = *f.TaxType
		}
	}

	// Collect manually entered GST amounts for computed fields with MANUAL tax type
	for _, entry := range req.Entries {
		if entry.GstAmount == nil {
			continue
		}
		fieldID, err := uuid.Parse(entry.FormFieldID)
		if err != nil {
			continue
		}
		f, ok := fieldMap[fieldID]
		if !ok || !f.IsComputed {
			continue
		}
		if f.TaxType != nil && *f.TaxType == "MANUAL" {
			manualGSTByKey[f.FieldKey] = *entry.GstAmount
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
			switch taxType {
			case method.TaxTreatmentInclusive:
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

	computed, err := s.formulaSvc.EvalFormulas(ctx, formVersionID, keyValues, taxTypeByKey, manualGSTByKey)
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
			input := &method.Input{Amount: val}

			// For MANUAL tax type on computed fields, check if GST was provided in entries
			if method.TaxTreatment(*f.TaxType) == method.TaxTreatmentManual {
				// Find if this computed field has a corresponding entry with GST amount
				var entryGST *float64
				for _, entry := range req.Entries {
					entryFieldID, _ := uuid.Parse(entry.FormFieldID)
					if entryFieldID == fieldID && entry.GstAmount != nil {
						entryGST = entry.GstAmount
						break
					}
				}

				// If GST amount is empty or zero, send net with gst=0, gross=net
				if entryGST == nil {
					net := util.Round(val, 2)
					gst := 0.0
					gross := net

					netAmount = net
					gstAmount = &gst
					grossAmount = &gross
				} else {
					// If GST provided, send net=net, gst=entry.gst, gross=net+gst
					net := util.Round(val, 2)
					gst := util.Round(*entryGST, 2)
					gross := util.Round(net+gst, 2)

					netAmount = net
					gstAmount = &gst
					grossAmount = &gross
				}
			} else {
				taxResult, err := s.methodSvc.Calculate(ctx, method.TaxTreatment(*f.TaxType), input)
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
		}

		item := RsComputedFieldValue{
			FieldID:       fieldID,
			FormFieldID:   fieldID.String(),
			FieldKey:      f.FieldKey,
			Label:         f.Label,
			IsComputed:    f.IsComputed,
			NetAmount:     netAmount,
			GstAmount:     gstAmount,
			GrossAmount:   grossAmount,
			SectionType:   f.SectionType,
			TaxType:       f.TaxType,
			CoaID:         f.CoaID,
			SortOrder:     f.SortOrder,
			IsHighlighted: f.IsHighlighted,
		}

		results = append(results, item)
	}

	return &RsLiveCalculate{
		FormVersionID:  formVersionID,
		ComputedFields: results,
	}, nil
}

func (s *service) GetFormSummary(ctx context.Context, formID string, actorID uuid.UUID, role string) ([]*RsTransactionRow, error) {
	return s.repo.GetTransactionsByFormID(ctx, formID, actorID, role)
}

// FormPreview implements [Service] - provides complete form preview with all fields and calculation summary
func (s *service) FormPreview(ctx context.Context, req *RqFormPreview, actorID uuid.UUID, role string) (*RsFormPreview, error) {
	// Validate clinic_id
	_, err := uuid.Parse(req.ClinicID)
	if err != nil {
		return nil, fmt.Errorf("invalid clinic_id: %w", err)
	}

	// Merge formulas into fields
	formulaMap := make(map[string]*formula.ExprNode)
	for i := range req.Formulas {
		f := &req.Formulas[i]
		formulaMap[f.FieldKey] = f.Expression
	}

	// Apply formulas to computed fields
	for i := range req.Fields {
		field := &req.Fields[i]
		if field.IsComputed && field.Formula == nil {
			if expr, ok := formulaMap[field.FieldKey]; ok {
				field.Formula = expr
			}
		}
	}

	// Build field map from request fields
	fieldKeyToField := make(map[string]*RqPreviewField)
	for i := range req.Fields {
		field := &req.Fields[i]
		fieldKeyToField[field.FieldKey] = field
	}

	// Process entries and calculate values
	keyValues := make(map[string]float64)
	fieldResults := make(map[string]*RsPreviewFieldValue)

	// Initialize all non-computed fields with zero
	for _, field := range req.Fields {
		if !field.IsComputed {
			keyValues[field.FieldKey] = 0
			fieldResults[field.FieldKey] = &RsPreviewFieldValue{
				FieldKey:      field.FieldKey,
				Label:         field.Label,
				IsComputed:    false,
				SectionType:   field.SectionType,
				TaxType:       field.TaxType,
				CoaID:         field.CoaID,
				SortOrder:     field.SortOrder,
				IsHighlighted: field.IsHighlighted,
			}
		}
	}

	// Process each entry with tax calculations
	for _, entry := range req.Values {
		field, ok := fieldKeyToField[entry.FieldKey]
		if !ok {
			return nil, fmt.Errorf("field with key %s not found in field definitions", entry.FieldKey)
		}

		if field.IsComputed {
			continue
		}

		// Calculate actual net amount based on tax type
		actualNetAmount := entry.NetAmount
		var gstAmount *float64
		var grossAmount *float64

		if field.TaxType != nil && *field.TaxType != "" {
			taxType := method.TaxTreatment(*field.TaxType)
			switch taxType {
			case method.TaxTreatmentInclusive:
				taxResult, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: entry.NetAmount})
				if err != nil {
					return nil, fmt.Errorf("tax calc for field %s: %w", field.FieldKey, err)
				}
				actualNetAmount = taxResult.Amount
				gst := taxResult.GstAmount
				gross := taxResult.TotalAmount
				gstAmount = &gst
				grossAmount = &gross

			case method.TaxTreatmentExclusive:
				taxResult, err := s.methodSvc.Calculate(ctx, taxType, &method.Input{Amount: entry.NetAmount})
				if err != nil {
					return nil, fmt.Errorf("tax calc for field %s: %w", field.FieldKey, err)
				}
				actualNetAmount = entry.NetAmount
				gst := taxResult.GstAmount
				gross := taxResult.TotalAmount
				gstAmount = &gst
				grossAmount = &gross

			case method.TaxTreatmentManual:
				if field.SectionType != nil && *field.SectionType == "COLLECTION" {
					actualNetAmount = entry.NetAmount
					if entry.GstAmount != nil {
						gstAmount = entry.GstAmount
						gross := entry.NetAmount + *entry.GstAmount
						grossAmount = &gross
					}
				} else {
					actualNetAmount = entry.NetAmount
					gstAmount = entry.GstAmount
					if entry.GstAmount != nil {
						gross := entry.NetAmount + *entry.GstAmount
						grossAmount = &gross
					}
				}

			case method.TaxTreatmentZero:
				actualNetAmount = entry.NetAmount
				gross := entry.NetAmount
				grossAmount = &gross
			}
		} else {
			// No tax type
			gross := entry.NetAmount
			grossAmount = &gross
		}

		keyValues[field.FieldKey] = actualNetAmount

		// Store the calculated values
		net := actualNetAmount
		fieldResults[field.FieldKey].NetAmount = &net
		fieldResults[field.FieldKey].GstAmount = gstAmount
		fieldResults[field.FieldKey].GrossAmount = grossAmount
	}

	// Build tax type map for computed fields
	taxTypeByKey := make(map[string]string)
	manualGSTByKey := make(map[string]float64)
	for _, field := range req.Fields {
		if field.IsComputed && field.TaxType != nil && *field.TaxType != "" {
			taxTypeByKey[field.FieldKey] = *field.TaxType
		}
	}

	// Collect manually entered GST amounts for computed fields
	for _, entry := range req.Values {
		if entry.GstAmount == nil {
			continue
		}
		field, ok := fieldKeyToField[entry.FieldKey]
		if !ok || !field.IsComputed {
			continue
		}
		if field.TaxType != nil && *field.TaxType == "MANUAL" {
			manualGSTByKey[field.FieldKey] = *entry.GstAmount
		}
	}

	// Compute section totals
	sectionTotals := make(map[string]float64)
	for _, fieldResult := range fieldResults {
		field, ok := fieldKeyToField[fieldResult.FieldKey]
		if !ok || field.IsComputed || field.SectionType == nil || *field.SectionType == "" {
			continue
		}
		if fieldResult.NetAmount != nil {
			sectionKey := "SECTION:" + *field.SectionType
			sectionTotals[sectionKey] += *fieldResult.NetAmount
		}
	}

	// Merge section totals into keyValues
	for key, val := range sectionTotals {
		keyValues[key] = val
	}

	// Evaluate formulas for computed fields
	computed, err := s.evaluatePreviewFormulas(ctx, req.Fields, keyValues, taxTypeByKey, manualGSTByKey)
	if err != nil {
		return nil, fmt.Errorf("eval formulas: %w", err)
	}

	// Process computed fields
	for fieldKey, val := range computed {
		field := fieldKeyToField[fieldKey]
		if field == nil || !field.IsComputed {
			continue
		}

		netAmount := val
		var gstAmount *float64
		var grossAmount *float64

		if field.TaxType != nil && *field.TaxType != "" {
			if method.TaxTreatment(*field.TaxType) == method.TaxTreatmentManual {
				var entryGST *float64
				for _, entry := range req.Values {
					if entry.FieldKey == fieldKey && entry.GstAmount != nil {
						entryGST = entry.GstAmount
						break
					}
				}

				if entryGST == nil {
					gst := 0.0
					gross := netAmount
					gstAmount = &gst
					grossAmount = &gross
				} else {
					gst := *entryGST
					gross := netAmount + gst
					gstAmount = &gst
					grossAmount = &gross
				}
			} else {
				taxResult, err := s.methodSvc.Calculate(ctx, method.TaxTreatment(*field.TaxType), &method.Input{Amount: val})
				if err != nil {
					return nil, fmt.Errorf("tax calc for field %s: %w", field.FieldKey, err)
				}
				net := taxResult.Amount
				gst := taxResult.GstAmount
				gross := taxResult.TotalAmount
				netAmount = net
				gstAmount = &gst
				grossAmount = &gross
			}
		} else {
			gross := netAmount
			grossAmount = &gross
		}

		fieldResults[fieldKey] = &RsPreviewFieldValue{
			FieldKey:      field.FieldKey,
			Label:         field.Label,
			IsComputed:    true,
			NetAmount:     &netAmount,
			GstAmount:     gstAmount,
			GrossAmount:   grossAmount,
			SectionType:   field.SectionType,
			TaxType:       field.TaxType,
			CoaID:         field.CoaID,
			SortOrder:     field.SortOrder,
			IsHighlighted: field.IsHighlighted,
		}
	}

	// Build all fields array sorted by sort_order
	allFields := make([]RsPreviewFieldValue, 0, len(fieldResults))
	for _, fieldValue := range fieldResults {
		allFields = append(allFields, *fieldValue)
	}

	// Sort by sort_order
	for i := 0; i < len(allFields); i++ {
		for j := i + 1; j < len(allFields); j++ {
			if allFields[i].SortOrder > allFields[j].SortOrder {
				allFields[i], allFields[j] = allFields[j], allFields[i]
			}
		}
	}

	// Calculate summary based on method
	summary, err := s.calculatePreviewSummaryFromRequest(ctx, req, allFields, fieldKeyToField)
	if err != nil {
		return nil, fmt.Errorf("calculate summary: %w", err)
	}

	return &RsFormPreview{
		Method:     req.Method,
		FormName:   "", // No form name yet since form is being created
		ClinicName: "", // TODO: Fetch clinic name if needed
		AllFields:  allFields,
		Summary:    summary,
	}, nil
}

func roundPreview(v float64) float64 {
	shifted := v * 100
	if shifted < 0 {
		shifted -= 0.5
	} else {
		shifted += 0.5
	}
	return float64(int(shifted)) / 100
}

// evaluatePreviewFormulas evaluates formulas for computed fields in preview mode
func (s *service) evaluatePreviewFormulas(_ context.Context, fields []RqPreviewField, keyValues map[string]float64, taxTypeByKey map[string]string, manualGSTByKey map[string]float64) (map[string]float64, error) {
	// Collect computed fields with formulas
	type computedField struct {
		fieldKey string
		expr     *formula.ExprNode
		field    *RqPreviewField
	}

	computedFields := make([]computedField, 0)
	fieldKeyToIdx := make(map[string]int)

	for i := range fields {
		field := &fields[i]
		if field.IsComputed && field.Formula != nil {
			fieldKeyToIdx[field.FieldKey] = len(computedFields)
			computedFields = append(computedFields, computedField{
				fieldKey: field.FieldKey,
				expr:     field.Formula,
				field:    field,
			})
		}
	}

	if len(computedFields) == 0 {
		return make(map[string]float64), nil
	}

	// Build dependency graph for topological sorting
	n := len(computedFields)
	deps := make([][]int, n)

	for i, cf := range computedFields {
		if cf.expr != nil {
			// Extract field dependencies from expression tree
			fieldDeps := extractFieldDependencies(cf.expr)
			for _, depKey := range fieldDeps {
				if j, ok := fieldKeyToIdx[depKey]; ok && j != i {
					deps[i] = append(deps[i], j)
				}
			}
		}
	}

	// Topological sort to evaluate in dependency order
	sorted := s.topoSort(n, deps)

	// Evaluate formulas in topological order
	vals := make(map[string]float64, len(keyValues))
	for k, v := range keyValues {
		vals[k] = v
	}
	
	result := make(map[string]float64, n)

	for _, i := range sorted {
		cf := computedFields[i]

		// Skip if no expression
		if cf.expr == nil {
			continue
		}

		// Evaluate the expression
		val, err := s.evalExpression(cf.expr, vals)
		if err != nil {
			return nil, fmt.Errorf("formula for field %q: %w", cf.fieldKey, err)
		}

		result[cf.fieldKey] = val

		// Feedback value: computed fields with tax type feed GROSS amount back for dependent formulas
		feedbackVal := val
		if taxType, hasTax := taxTypeByKey[cf.fieldKey]; hasTax {
			switch taxType {
			case "EXCLUSIVE":
				feedbackVal = val * 1.1 // Add 10% GST
			case "INCLUSIVE":
				feedbackVal = val // Already gross
			case "ZERO":
				feedbackVal = val // No GST
			case "MANUAL":
				// For MANUAL, val is NET amount
				// Add manually entered GST to get gross amount for dependent formulas
				if gst, hasGST := manualGSTByKey[cf.fieldKey]; hasGST {
					feedbackVal = val + gst // NET + GST = GROSS
				} else {
					feedbackVal = val // No GST provided, use NET
				}
			}
		}
		vals[cf.fieldKey] = feedbackVal
	}

	return result, nil
}

// topoSort returns indices in dependency-first order using Kahn's algorithm
func (s *service) topoSort(n int, deps [][]int) []int {
	// Build reverse adjacency: revAdj[j] = list of nodes that depend on j
	revAdj := make([][]int, n)
	inDegree := make([]int, n)
	for i, d := range deps {
		inDegree[i] = len(d)
		for _, j := range d {
			revAdj[j] = append(revAdj[j], i)
		}
	}

	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	order := make([]int, 0, n)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)
		for _, dependent := range revAdj[curr] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	return order
}

// extractFieldDependencies extracts all field keys referenced in an expression tree
func extractFieldDependencies(expr *formula.ExprNode) []string {
	if expr == nil {
		return nil
	}

	var deps []string
	switch expr.Type {
	case "field", "section":
		if expr.Key != "" {
			deps = append(deps, expr.Key)
		}
	case "operator":
		deps = append(deps, extractFieldDependencies(expr.Left)...)
		deps = append(deps, extractFieldDependencies(expr.Right)...)
	}
	return deps
}

// evalExpression evaluates an expression tree with given field values
func (s *service) evalExpression(expr *formula.ExprNode, vals map[string]float64) (float64, error) {
	if expr == nil {
		return 0, fmt.Errorf("expression is nil")
	}

	switch expr.Type {
	case "constant":
		if expr.Value == nil {
			return 0, fmt.Errorf("constant node has nil value")
		}
		return *expr.Value, nil

	case "field":
		if expr.Key == "" {
			return 0, fmt.Errorf("field node has empty key")
		}
		v, ok := vals[expr.Key]
		if !ok {
			return 0, fmt.Errorf("field key %q not found in values", expr.Key)
		}
		return v, nil

	case "section":
		// SECTION aggregates all fields with matching section_type
		// Section key format: "SECTION:COLLECTION", "SECTION:COST", "SECTION:OTHER_COST"
		if expr.Key == "" {
			return 0, fmt.Errorf("section node has empty key")
		}
		v, ok := vals[expr.Key]
		if !ok {
			return 0, fmt.Errorf("section key %q not found in values", expr.Key)
		}
		return v, nil

	case "text":
		// TEXT fields are non-numeric, return 0
		return 0, nil

	case "operator":
		if expr.Op == "" {
			return 0, fmt.Errorf("operator node has empty operator")
		}
		if expr.Left == nil || expr.Right == nil {
			return 0, fmt.Errorf("operator %q missing children", expr.Op)
		}

		left, err := s.evalExpression(expr.Left, vals)
		if err != nil {
			return 0, err
		}

		right, err := s.evalExpression(expr.Right, vals)
		if err != nil {
			return 0, err
		}

		switch expr.Op {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unknown operator %q", expr.Op)
		}

	default:
		return 0, fmt.Errorf("unknown node type %q", expr.Type)
	}
}

// calculatePreviewSummaryFromRequest calculates summary from request data
func (s *service) calculatePreviewSummaryFromRequest(_ context.Context, req *RqFormPreview, allFields []RsPreviewFieldValue, fieldKeyToField map[string]*RqPreviewField) (*PreviewSummary, error) {
	var incomeSum, expenseSum, otherCostSum float64
	var incomeGST, expenseGST float64
	var paidByOwnerSum float64

	// Aggregate amounts by section type
	for _, fieldValue := range allFields {
		if fieldValue.SectionType == nil || fieldValue.NetAmount == nil {
			continue
		}

		field, ok := fieldKeyToField[fieldValue.FieldKey]
		if !ok {
			continue
		}

		switch *fieldValue.SectionType {
		case "COLLECTION":
			incomeSum += *fieldValue.NetAmount
			// Only collect GST for MANUAL tax type fields
			if field.TaxType != nil && *field.TaxType == "MANUAL" && fieldValue.GstAmount != nil {
				incomeGST += *fieldValue.GstAmount
			}

		case "COST":
			if field.PaymentResponsibility != nil {
				switch *field.PaymentResponsibility {
				case "CLINIC":
					expenseSum += *fieldValue.NetAmount
					if fieldValue.GstAmount != nil {
						expenseGST += *fieldValue.GstAmount
					}
				case "OWNER":
					expenseSum += *fieldValue.NetAmount
					paidByOwnerSum += *fieldValue.NetAmount
				}
			}

		case "OTHER_COST":
			otherCostSum += *fieldValue.NetAmount
		}
	}

	netAmount := incomeSum - expenseSum

	summary := &PreviewSummary{
		NetAmount: roundPreview(netAmount),
	}

	// Calculate based on method
	switch req.Method {
	case "SERVICE_FEE":
		// SERVICE_FEE calculation
		clinicShare := float64(req.ClinicShare)
		serviceFee := netAmount * (clinicShare / 100)
		gstServiceFee := serviceFee * 0.1
		totalServiceFee := serviceFee + gstServiceFee
		remittedAmount := netAmount - totalServiceFee - otherCostSum + paidByOwnerSum + incomeGST

		sf := roundPreview(serviceFee)
		gsf := roundPreview(gstServiceFee)
		tsf := roundPreview(totalServiceFee)
		ra := roundPreview(remittedAmount)
		ceg := roundPreview(expenseGST)

		summary.ServiceFee = &sf
		summary.GstServiceFee = &gsf
		summary.TotalServiceFee = &tsf
		summary.RemittedAmount = &ra
		summary.ClinicExpenseGST = &ceg

	case "INDEPENDENT_CONTRACTOR":
		// INDEPENDENT_CONTRACTOR calculation
		ownerShare := float64(req.OwnerShare)
		totalRemuneration := netAmount * (ownerShare / 100)

		superDecimal := 0.0
		if req.SuperComponent != nil {
			superDecimal = *req.SuperComponent / 100
		}

		commissionBase := totalRemuneration
		var superAmount float64
		if superDecimal > 0 {
			superAmount = commissionBase * superDecimal
		}

		gstOnRemuneration := commissionBase * 0.10
		invoiceTotal := commissionBase + gstOnRemuneration + superAmount

		tr := roundPreview(totalRemuneration)
		gor := roundPreview(gstOnRemuneration)
		it := roundPreview(invoiceTotal)
		ocd := roundPreview(otherCostSum)

		summary.TotalRemuneration = &tr
		summary.GstOnRemuneration = &gor
		summary.InvoiceTotal = &it
		summary.OtherCostDeduction = &ocd

		if superDecimal > 0 {
			sa := roundPreview(superAmount)
			br := roundPreview(commissionBase)
			summary.SuperComponent = &sa
			summary.BaseRemuneration = &br
		}

		// IC-specific calculation
		commission := commissionBase
		gstOnCommission := gstOnRemuneration
		paymentReceived := invoiceTotal

		comm := roundPreview(commission)
		gstComm := roundPreview(gstOnCommission)
		pr := roundPreview(paymentReceived)

		summary.Commission = &comm
		summary.GstOnCommission = &gstComm
		summary.PaymentReceived = &pr
	}

	return summary, nil
}
