package template

import (
	"encoding/json"
	"time"
)

// ApplyPDFCollections maps invoice line items into InvoiceData fields used by Handlebars templates.
func ApplyPDFCollections(data *InvoiceData, items []InvoiceItem, sections []InvoiceSectionMeta, fallbackInvoiceNumber string) {
	collections, invoiceNumber, paymentMeta := buildInvoiceCollections(items, sections, fallbackInvoiceNumber)

	if invoiceNumber != "" {
		data.InvoiceNumber = invoiceNumber
	}

	data.PatientFeeItems = collections.patientFeeItems
	data.ServiceFeeItems = collections.serviceFeeItems
	data.SettlementItems = collections.settlementItems
	data.RemittanceItems = collections.remittanceItems
	data.Subtotal = collections.subtotal
	data.TaxTotal = collections.taxTotal
	data.GrandTotal = collections.grandTotal
	data.CustomFeeRate = collections.customFeeRate
	if collections.customFeeRate != "" {
		data.CustomFeeRateDisplay = collections.customFeeRate + "%"
	}

	if len(collections.serviceFeeRateIntro) > 0 {
		data.ServiceFeeRateIntro = collections.serviceFeeRateIntro
	}
	data.ServiceDescriptionItems = collections.serviceDescriptionItems

	data.CustomPaymentMethod = paymentMeta.paymentMethod
	data.PaymentMethodLabel = paymentMeta.paymentMethod
	data.CustomPaymentAccountName = paymentMeta.accountName
	data.CustomPaymentBsb = paymentMeta.bsb
	data.CustomPaymentAccount = paymentMeta.accountNumber

	if paymentMeta.paymentDate != "" {
		if parsedTime, err := time.Parse("2006-01-02 15:04:05.999999-07", paymentMeta.paymentDate); err == nil {
			data.PaymentDateDisplay = parsedTime.Format("02 January 2006")
		} else if parsedTime, err := time.Parse("2006-01-02", paymentMeta.paymentDate); err == nil {
			data.PaymentDateDisplay = parsedTime.Format("02 January 2006")
		} else {
			data.PaymentDateDisplay = paymentMeta.paymentDate
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
