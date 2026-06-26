package template

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Helper function to convert raw expression JSON into a clean strings
func parseExpressionFormula(exprStr string) string {
	if exprStr == "" {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(exprStr), &data); err != nil {
		return ""
	}

	// Helper to extract key or value from operands safely
	extractOperand := func(operand interface{}) string {
		m, ok := operand.(map[string]interface{})
		if !ok {
			return ""
		}
		if val, exists := m["key"]; exists && val != nil {
			return fmt.Sprintf("%v", val)
		}
		if val, exists := m["value"]; exists && val != nil {
			return fmt.Sprintf("%v", val)
		}
		return ""
	}

	left := extractOperand(data["left"])
	op, _ := data["op"].(string)
	right := extractOperand(data["right"])

	if left != "" && op != "" && right != "" {
		return fmt.Sprintf("%s %s %s", left, op, right)
	}
	return ""
}

// ApplyPDFCollections maps raw items into structured contexts sorted by SortOrder completely dynamically.
func ApplyPDFCollections(
	data *InvoiceData,
	items []InvoiceItem,
	sections []InvoiceSectionMeta,
	fallbackInvoiceNumber string,
) {
	data.PatientFeeItems = []map[string]interface{}{}
	data.ServiceFeeItems = []map[string]interface{}{}
	data.SettlementItems = []map[string]interface{}{}
	data.RemittanceItems = []map[string]interface{}{}
	data.ServiceDescriptionItems = []string{}

	var calculationDocNo string
	var taxInvoiceDocNo string
	var remittanceDocNo string

	for _, sec := range sections {
		secType := strings.ToUpper(strings.TrimSpace(sec.SectionType))

		switch secType {
		case "CALCULATION_STATEMENT":
			calculationDocNo = sec.DocumentNumber

		case "SFA_INVOICE", "TAX_INVOICE":
			taxInvoiceDocNo = sec.DocumentNumber

		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			remittanceDocNo = sec.DocumentNumber

			if sec.PaymentMethod != nil {
				data.CustomPaymentMethod = *sec.PaymentMethod
				data.PaymentMethodLabel = *sec.PaymentMethod
			}
			if sec.AccountName != nil {
				data.CustomPaymentAccountName = *sec.AccountName
			}
			if sec.Bsb != nil {
				data.CustomPaymentBsb = *sec.Bsb
			}
			if sec.AccountNumber != nil {
				data.CustomPaymentAccount = *sec.AccountNumber
			}

			if sec.PaymentDate != nil && *sec.PaymentDate != "" {
				if parsed, err := time.Parse("2006-01-02", *sec.PaymentDate); err == nil {
					data.PaymentDateDisplay = parsed.Format("02 January 2006")
				} else if parsed, err := time.Parse("02 January 2006", *sec.PaymentDate); err == nil {
					data.PaymentDateDisplay = parsed.Format("02 January 2006")
				} else {
					data.PaymentDateDisplay = *sec.PaymentDate
				}
			}
		}
	}

	data.InvoiceNumber = fallbackInvoiceNumber

	// Explicitly sort ALL items by SortOrder across all sections
	sort.Slice(items, func(i, j int) bool {
		return items[i].SortOrder < items[j].SortOrder
	})

	seenDescriptionLines := make(map[string]bool)

	// PASS 1: Extract block descriptions for bulleted lists dynamically
	for _, item := range items {
		if item.FieldKey == nil {
			continue
		}

		keyUpper := strings.ToUpper(strings.TrimSpace(*item.FieldKey))
		if keyUpper != "FEE_RATE" && keyUpper != "SERVICE_FEE_RATE" {
			continue
		}

		feeRateVal := item.Amount
		if feeRateVal > 0 && feeRateVal <= 1 {
			feeRateVal *= 100
		}

		data.CustomFeeRate = fmt.Sprintf("%.1f", feeRateVal)
		data.CustomFeeRateDisplay = data.CustomFeeRate + "%"

		if item.Description == "" {
			continue
		}

		normalized := strings.ReplaceAll(item.Description, "\r\n", "\n")
		lines := strings.Split(normalized, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)

			if seenDescriptionLines[line] {
				continue
			}
			seenDescriptionLines[line] = true
			data.ServiceDescriptionItems = append(data.ServiceDescriptionItems, line)
		}
	}

	// PASS 2: Map fields cleanly by section type and sort order with zero hardcoding
	for _, item := range items {
		// Skip calculation layouts from line-item listing structures
		if item.FieldKey != nil {
			keyUpper := strings.ToUpper(strings.TrimSpace(*item.FieldKey))
			if keyUpper == "FEE_RATE" || keyUpper == "SERVICE_FEE_RATE" {
				continue
			}
		}

		trimEntry := strings.TrimSpace(item.EntryType)
		trimBas := ""
		if item.BASCode != nil {
			trimBas = strings.TrimSpace(*item.BASCode)
		}

		// Route directly from the field present on the InvoiceItem struct
		secType := strings.ToUpper(strings.TrimSpace(item.SectionType))

		isDebit := strings.EqualFold(trimEntry, "DEBIT")
		isNegative := isDebit || item.Amount < 0

		rowClass := ""
		valueClass := ""
		hasFormula := item.Expression != nil && *item.Expression != ""
		isBold := item.IsFinal || hasFormula

		if item.IsFinal && !hasFormula {
			rowClass = "row-final-balance"
		} else if item.IsFinal && hasFormula {
			rowClass = "bg-sky-row"
		}

		if !item.IsFinal && !hasFormula && (isDebit || strings.EqualFold(trimEntry, "CREDIT")) {
			valueClass = "txt-blue-val"
		}

		// Convert structural JSON expression string into clear formatted string: Name [ Formula ]
		displayLabel := item.Name
		if hasFormula {
			if cleanFormula := parseExpressionFormula(*item.Expression); cleanFormula != "" {
				displayLabel = fmt.Sprintf("%s [ %s ]", item.Name, cleanFormula)
			}
		}

		row := map[string]interface{}{
			"label":       displayLabel,
			"amount":      item.Amount,
			"bas_code":    trimBas,
			"is_bold":     isBold,
			"is_negative": isNegative,
			"row_class":   rowClass,
			"value_class": valueClass,
		}

		// --- SECTIONS ROUTING ---
		switch secType {
		case "CALCULATION_STATEMENT":
			data.PatientFeeItems = append(data.PatientFeeItems, row)

		case "SFA_INVOICE", "TAX_INVOICE":
			// Structurally identify subtotal variables without hardcoding the BAS code strings
			if item.IsFinal {
				data.GrandTotal = item.Amount
			} else if hasFormula {
				if isNegative || isDebit {
					data.TaxTotal = item.Amount
				} else {
					data.Subtotal = item.Amount
				}
			}
			data.ServiceFeeItems = append(data.ServiceFeeItems, row)

		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			data.RemittanceItems = append(data.RemittanceItems, row)

			data.SettlementItems = append(data.SettlementItems, map[string]interface{}{
				"label":       displayLabel,
				"amount":      item.Amount,
				"is_bold":     isBold,
				"is_negative": isNegative,
				"row_class":   rowClass,
				"value_class": valueClass,
			})
		}
	}

	if data.TemplateSettings == nil {
		data.TemplateSettings = map[string]interface{}{}
	}

	if calculationDocNo == "" {
		calculationDocNo = fallbackInvoiceNumber
	}
	if taxInvoiceDocNo == "" {
		taxInvoiceDocNo = fallbackInvoiceNumber
	}
	if remittanceDocNo == "" {
		remittanceDocNo = fallbackInvoiceNumber
	}

	data.TemplateSettings["calculation_invoice_number"] = calculationDocNo
	data.TemplateSettings["tax_invoice_number"] = taxInvoiceDocNo
	data.TemplateSettings["remittance_invoice_number"] = remittanceDocNo
}

func invoiceDataToMap(data InvoiceData) (map[string]interface{}, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(bytes, &dataMap); err != nil {
		return nil, err
	}

	dataMap["service_fee_items"] = orEmptySlice(data.ServiceFeeItems)
	dataMap["patient_fee_items"] = orEmptySlice(data.PatientFeeItems)
	dataMap["settlement_items"] = orEmptySlice(data.SettlementItems)
	dataMap["remittance_items"] = orEmptySlice(data.RemittanceItems)
	dataMap["service_description_items"] = data.ServiceDescriptionItems

	dataMap["service_fee_rate_intro"] = map[string]interface{}{
		"label":            "Services rendered to you for the period, including:",
		"fee_rate_display": data.CustomFeeRateDisplay,
		"amount_display":   "",
	}

	return dataMap, nil
}

func orEmptySlice(items []map[string]interface{}) []map[string]interface{} {
	if items == nil {
		return []map[string]interface{}{}
	}
	return items
}
