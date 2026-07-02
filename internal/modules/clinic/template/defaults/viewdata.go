package defaults

// BillingMethodView holds every value that varies by billing method
// (a/b/c), resolved once in Go instead of branched inline in Handlebars.
// Add a new field + switch case here when a new method is introduced —
// no template file needs to change.
type BillingMethodView struct {
	PatientFeesLabel       string
	ServiceFeeSectionLabel string
	RateLabel              string // "Fee rate" vs "Commission rate"
	TaxInvoiceIntro        string
	HideFeeRate            bool
	ShowServiceDescription bool
	ShowPaymentDetails     bool
	ShowRemittance         bool
	TaxInvoiceTitle        string
	InvoiceNumberLabel     string
	BillToLabel            string // "BILL TO" vs "SUPPLIER (DENTIST)"
	ShowRCTINote           bool
	PageTwoLabel           string // "Tax Invoice (page 2)" vs "RCTI (page 2)"
	DefaultFooterNote      string // used only if no custom notes/terms supplied
}

func ResolveBillingMethod(method string) BillingMethodView {
	switch method {
	case "b":
		return BillingMethodView{
			PatientFeesLabel:       "1. PATIENT FEES",
			ServiceFeeSectionLabel: "2. SERVICE & FACILITY FEE",
			RateLabel:              "Fee rate",
			TaxInvoiceIntro:        "", // filled from service_fee_rate_intro.label in template data
			ShowServiceDescription: true,
			ShowPaymentDetails:     true,
			ShowRemittance:         false, // method B has no remittance doc
			TaxInvoiceTitle:        "TAX INVOICE",
			InvoiceNumberLabel:     "Invoice No.",
			BillToLabel:            "BILL TO",
			ShowRCTINote:           false,
			PageTwoLabel:           "Tax Invoice (page 2)",
			DefaultFooterNote:      "Patient fees for the period were collected directly by the dentist. This tax invoice is the clinic's service & facility fee (plus any costs paid by the clinic) and is payable by the dentist to the clinic at the account above.",
		}
	case "c":
		return BillingMethodView{
			PatientFeesLabel:       "1. PATIENT FEES COLLECTED ON YOUR BEHALF",
			ServiceFeeSectionLabel: "2. DENTIST COMMISSION (Independent Contractor)",
			RateLabel:              "Commission rate",
			TaxInvoiceIntro:        "Professional dental services for the period {{billing_period}}, remunerated at the agreed commission rate on net patient fees.",
			HideFeeRate:            true,
			ShowServiceDescription: false,
			ShowPaymentDetails:     false,
			ShowRemittance:         true,
			TaxInvoiceTitle:        "RECIPIENT CREATED TAX INVOICE",
			InvoiceNumberLabel:     "RCTI No.",
			BillToLabel:            "SUPPLIER (DENTIST)",
			ShowRCTINote:           true,
			PageTwoLabel:           "RCTI (page 2)",
			DefaultFooterNote:      "This RCTI is created by the clinic (recipient) on behalf of the dentist (supplier). The dentist must not issue a separate tax invoice for this supply. See Remittance Advice for the net amount paid.",
		}
	default: // method a
		return BillingMethodView{
			PatientFeesLabel:       "1. PATIENT FEES COLLECTED ON YOUR BEHALF",
			ServiceFeeSectionLabel: "2. SERVICE & FACILITY FEE",
			RateLabel:              "Fee rate",
			ShowServiceDescription: true,
			ShowPaymentDetails:     false,
			ShowRemittance:         true,
			TaxInvoiceTitle:        "TAX INVOICE",
			InvoiceNumberLabel:     "Invoice No.",
			BillToLabel:            "BILL TO",
			ShowRCTINote:           false,
			PageTwoLabel:           "Tax Invoice (page 2)",
			DefaultFooterNote:      "Payment terms: This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you.",
		}
	}
}

// resolveFooterNote picks the Calculation Statement footer text:
// explicit invoice notes > template setting terms text > method default.
func resolveFooterNote(notes, templateTermsText, methodDefault string) string {
	if notes != "" {
		return notes
	}
	if templateTermsText != "" {
		return templateTermsText
	}
	return methodDefault
}

// resolvePaymentTerms picks the Tax Invoice payment-terms text:
// explicit payment_terms > template setting payment terms > "" (template
// renders nothing when empty, matching the old {{else}} with no fallback text).
func resolvePaymentTerms(paymentTerms, templateSettingsPaymentTerms string) string {
	if paymentTerms != "" {
		return paymentTerms
	}
	return templateSettingsPaymentTerms
}

// BuildTemplateData assembles the full data map handed to Handlebars.Exec
// for a given invoice render. This is the single place billing-method
// branching, fallback resolution, and template variables converge —
// every .go template file downstream only reads flat fields off this map.
func BuildTemplateData(method string, notes, templateTermsText, paymentTerms, templateSettingsPaymentTerms string, base map[string]interface{}) map[string]interface{} {
	bm := ResolveBillingMethod(method)

	data := make(map[string]interface{}, len(base)+3)
	for k, v := range base {
		data[k] = v
	}

	data["billing_method"] = bm
	data["footer_note"] = resolveFooterNote(notes, templateTermsText, bm.DefaultFooterNote)
	data["payment_terms_resolved"] = resolvePaymentTerms(paymentTerms, templateSettingsPaymentTerms)

	return data
}
