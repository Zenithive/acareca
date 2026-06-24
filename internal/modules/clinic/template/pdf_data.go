package template

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ApplyPDFCollections maps invoice line items into InvoiceData fields used by Handlebars templates.
func ApplyPDFCollections(
	data *InvoiceData,
	items []InvoiceItem,
	sections []InvoiceSectionMeta,
	fallbackInvoiceNumber string,
) {
	collections, invoiceNumber, paymentMeta :=
		buildInvoiceCollections(
			items,
			sections,
			fallbackInvoiceNumber,
		)

	if invoiceNumber != "" {
		data.InvoiceNumber = invoiceNumber
	}

	data.PatientFeeItems = collections.patientFeeItems
	data.ServiceFeeItems = collections.serviceFeeItems
	data.SettlementItems = collections.settlementItems
	data.RemittanceItems = collections.remittanceItems

	data.ServiceFeeRateIntro = collections.serviceFeeRateIntro
	data.ServiceDescriptionItems = collections.serviceDescriptionItems

	data.Subtotal = collections.subtotal
	data.TaxTotal = collections.taxTotal
	data.GrandTotal = collections.grandTotal

	data.CustomFeeRate = collections.customFeeRate

	if collections.customFeeRate != "" {
		data.CustomFeeRateDisplay =
			collections.customFeeRate + "%"
	}

	data.CustomPaymentMethod =
		paymentMeta.paymentMethod

	data.PaymentMethodLabel =
		paymentMeta.paymentMethod

	data.CustomPaymentAccountName =
		paymentMeta.accountName

	data.CustomPaymentBsb =
		paymentMeta.bsb

	data.CustomPaymentAccount =
		paymentMeta.accountNumber

	if paymentMeta.paymentDate != "" {

		if parsed, err := time.Parse(
			"2006-01-02",
			paymentMeta.paymentDate,
		); err == nil {

			data.PaymentDateDisplay =
				parsed.Format("02 January 2006")
		}
	}
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

	dataMap["invoice_number"] = data.InvoiceNumber
	dataMap["clinic_name"] = data.ClinicName
	dataMap["issue_date_display"] = data.IssueDateDisplay
	dataMap["due_date_display"] = data.DueDateDisplay
	dataMap["billing_period"] = data.BillingPeriod
	dataMap["invoice_frequency"] = data.InvoiceFrequency
	dataMap["logo_url"] = data.LogoURL
	dataMap["notes"] = data.Notes
	dataMap["terms_text"] = data.TermsText

	dataMap["bill_from"] = map[string]interface{}{
		"name":    data.BillFrom.Name,
		"address": data.BillFrom.Address,
		"abn":     data.BillFrom.ABN,
		"email":   data.BillFrom.Email,
		"phone":   data.BillFrom.Phone,
	}
	dataMap["bill_to"] = map[string]interface{}{
		"name":    data.BillTo.Name,
		"address": data.BillTo.Address,
		"abn":     data.BillTo.ABN,
		"email":   data.BillTo.Email,
		"phone":   data.BillTo.Phone,
	}

	if data.TemplateSettings != nil {
		dataMap["template_settings"] = data.TemplateSettings
	}

	dataMap["custom_fee_rate"] = data.CustomFeeRate
	dataMap["custom_fee_rate_display"] = data.CustomFeeRateDisplay
	dataMap["grand_total"] = data.GrandTotal
	dataMap["subtotal"] = data.Subtotal
	dataMap["tax_total"] = data.TaxTotal

	dataMap["patient_fee_items"] = orEmptySlice(data.PatientFeeItems)
	dataMap["service_fee_items"] = orEmptySlice(data.ServiceFeeItems)
	dataMap["settlement_items"] = orEmptySlice(data.SettlementItems)
	dataMap["remittance_items"] = orEmptySlice(data.RemittanceItems)
	dataMap["tax_invoice_items"] = orEmptySlice(data.TaxInvoiceItems)

	dataMap["custom_payment_method"] = data.CustomPaymentMethod
	dataMap["payment_method_label"] = data.PaymentMethodLabel
	dataMap["custom_payment_account_name"] = data.CustomPaymentAccountName
	dataMap["custom_payment_bsb"] = data.CustomPaymentBsb
	dataMap["custom_payment_account"] = data.CustomPaymentAccount
	dataMap["payment_date_display"] = data.PaymentDateDisplay

	if data.ServiceFeeRateIntro != nil {
		dataMap["service_fee_rate_intro"] = data.ServiceFeeRateIntro
	} else {
		dataMap["service_fee_rate_intro"] = map[string]interface{}{
			"label":            "Service & Facility Fee",
			"fee_rate_display": data.CustomFeeRateDisplay,
			"amount_display":   data.CustomFeeRate,
		}
	}

	if data.ServiceDescriptionItems != nil {
		dataMap["service_description_items"] = data.ServiceDescriptionItems
	} else {
		dataMap["service_description_items"] = []string{}
	}

	return dataMap, nil
}

func orEmptySlice(items []map[string]interface{}) []map[string]interface{} {
	if items == nil {
		return []map[string]interface{}{}
	}
	return items
}

func buildInvoiceCollections(items []InvoiceItem, sections []InvoiceSectionMeta, fallbackInvoiceNumber string) (invoiceCollections, string, invoicePaymentMeta) {
	var c invoiceCollections
	var payment invoicePaymentMeta

	invoiceNumber := fallbackInvoiceNumber

	// Extract dynamic account values from sections metadata directly
	for _, sec := range sections {
		switch strings.ToUpper(sec.SectionType) {
		case "SFA_INVOICE":
			if sec.DocumentNumber != "" {
				invoiceNumber = sec.DocumentNumber
			}
		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			if sec.PaymentMethod != nil {
				payment.paymentMethod = *sec.PaymentMethod
			}
			if sec.AccountName != nil {
				payment.accountName = *sec.AccountName
			}
			if sec.Bsb != nil {
				payment.bsb = *sec.Bsb
			}
			if sec.AccountNumber != nil {
				payment.accountNumber = *sec.AccountNumber
			}
			if sec.PaymentDate != nil {
				payment.paymentDate = *sec.PaymentDate
			}
		}
	}

	var incomeTotal float64
	var deductionTotal float64

	// PASS 1 - Parse and clean Fee Rate attributes dynamically
	for _, item := range items {
		if item.FieldKey == nil {
			continue
		}

		if strings.EqualFold(*item.FieldKey, "FEE_RATE") {
			feeRate := item.Amount
			if feeRate <= 1 {
				feeRate *= 100
			}
			c.customFeeRate = fmt.Sprintf("%.1f", feeRate)

			// Setup structural variables for dynamic injection into Handlebars Context
			c.serviceFeeRateIntro = map[string]interface{}{
				"label":            "Services rendered to you for the period, including:",
				"fee_rate_display": c.customFeeRate + "%",
				"amount_display":   "",
			}

			if item.Description != "" {
				lines := strings.Split(item.Description, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || strings.Contains(strings.ToLower(line), "here's a list of services") {
						continue
					}
					c.serviceDescriptionItems = append(c.serviceDescriptionItems, line)
				}
			}
		}
	}

	// PASS 2 - Build data arrays mapping sequentially across layout scopes
	for _, item := range items {
		sectionType := strings.ToUpper(item.SectionType)

		switch sectionType {
		case "CALCULATION_STATEMENT":
			if strings.EqualFold(item.EntryType, "CREDIT") {
				incomeTotal += item.Amount
				c.patientFeeItems = append(c.patientFeeItems, map[string]interface{}{
					"label":    item.Name,
					"amount":   item.Amount,
					"bas_code": "G1",
				})
			}

			if strings.EqualFold(item.EntryType, "DEBIT") {
				deductionTotal += item.Amount
				c.patientFeeItems = append(c.patientFeeItems, map[string]interface{}{
					"label":       "Less : " + item.Name,
					"amount":      item.Amount,
					"bas_code":    "1A",
					"value_class": "txt-blue-val",
				})
			}

		case "SFA_INVOICE", "TAX_INVOICE":
			if item.FieldKey != nil && strings.EqualFold(*item.FieldKey, "FEE_RATE") {
				continue
			}

			row := map[string]interface{}{
				"label":     item.Name,
				"amount":    item.Amount,
				"bas_code":  "",
				"is_bold":   item.IsFinal,
				"row_class": "",
			}

			if item.BASCode != nil {
				row["bas_code"] = *item.BASCode
			}

			if item.IsFinal {
				row["row_class"] = "row-total"
				c.grandTotal = item.Amount
				c.subtotal = item.Amount / 1.1
				c.taxTotal = item.Amount - c.subtotal
			}

			c.serviceFeeItems = append(c.serviceFeeItems, row)

		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			isDebit := strings.EqualFold(item.EntryType, "DEBIT")
			amtVal := item.Amount

			row := map[string]interface{}{
				"label":       item.Name,
				"amount":      amtVal,
				"is_negative": isDebit || amtVal < 0,
			}

			if item.IsFinal {
				row["row_class"] = "row-final-balance"
			}

			c.remittanceItems = append(c.remittanceItems, row)
		}
	}

	// 1. Append Patient Fees aggregated row calculation mapping to [G1 - 1A]
	patientNet := incomeTotal - deductionTotal
	c.patientFeeItems = append(c.patientFeeItems, map[string]interface{}{
		"label":     "Total",
		"amount":    patientNet,
		"bas_code":  "G11",
		"row_class": "bg-sky-row",
		"is_bold":   true,
	})

	// 2. Clear balance definition avoiding double-subtractions
	netBalance := incomeTotal - deductionTotal - c.grandTotal

	c.settlementItems = []map[string]interface{}{
		{
			"label":  "Total Income",
			"amount": incomeTotal,
		},
		{
			"label":       "Less : Total Expenses",
			"amount":      deductionTotal,
			"is_negative": true,
		},
		{
			"label":       "Total Service & Facility Fee (incl. GST)",
			"amount":      c.grandTotal,
			"is_negative": true,
			"row_class":   "bg-sky-row",
		},
		{
			"label":     "Net balance",
			"amount":    netBalance,
			"row_class": "row-final-balance",
			"is_bold":   true,
		},
	}

	return c, invoiceNumber, payment
}
