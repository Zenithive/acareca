package templates

type BillingMethod string

const (
	MethodSFA_CLINIC_COLLECTS    BillingMethod = "SFA_CLINIC_COLLECTS"
	MethodSFA_DENTIST_COLLECTS   BillingMethod = "SFA_DENTIST_COLLECTS"
	MethodINDEPENDENT_CONTRACTOR BillingMethod = "INDEPENDENT_CONTRACTOR"
)

type BillingMethodView struct {
	PatientFeesLabel       string `json:"patient_fees_label"`
	ServiceFeeSectionLabel string `json:"service_fee_section_label"`

	// Rate display
	RateLabel   string `json:"rate_label"` // "Fee rate" vs "Commission rate"
	HideFeeRate bool   `json:"hide_fee_rate"`

	// Tax invoice specific
	TaxInvoiceIntro    string `json:"tax_invoice_intro"`
	TaxInvoiceTitle    string `json:"tax_invoice_title"`
	InvoiceNumberLabel string `json:"invoice_number_label"`

	// Address banner
	BillToLabel  string `json:"bill_to_label"` // "BILL TO" vs "SUPPLIER (DENTIST)"
	ShowRCTINote bool   `json:"show_rcti_note"`

	// Features
	ShowServiceDescription bool `json:"show_service_description"`
	ShowPaymentDetails     bool `json:"show_payment_details"`
	ShowRemittance         bool `json:"show_remittance"`

	// References
	PageTwoLabel string `json:"page_two_label"` // "Tax Invoice (page 2)" vs "RCTI (page 2)"

	// Default text
	DefaultFooterNote string `json:"default_footer_note"` // used only if no custom notes/terms supplied
}

func MethodSfaClinicCollectConfig() BillingMethodView {
	return BillingMethodView{
		PatientFeesLabel:       "1. PATIENT FEES COLLECTED ON YOUR BEHALF",
		ServiceFeeSectionLabel: "2. SERVICE & FACILITY FEE",
		RateLabel:              "Fee rate",
		TaxInvoiceIntro:        "",
		HideFeeRate:            false,
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

func MethodBDentistCollectConfig() BillingMethodView {
	return BillingMethodView{
		PatientFeesLabel:       "1. PATIENT FEES",
		ServiceFeeSectionLabel: "2. SERVICE & FACILITY FEE",
		RateLabel:              "Fee rate",
		TaxInvoiceIntro:        "", // filled from service_fee_rate_intro.label in template data
		HideFeeRate:            false,
		ShowServiceDescription: true,
		ShowPaymentDetails:     true,
		ShowRemittance:         false,
		TaxInvoiceTitle:        "TAX INVOICE",
		InvoiceNumberLabel:     "Invoice No.",
		BillToLabel:            "BILL TO",
		ShowRCTINote:           false,
		PageTwoLabel:           "Tax Invoice (page 2)",
		DefaultFooterNote:      "Patient fees for the period were collected directly by the dentist. This tax invoice is the clinic's service & facility fee (plus any costs paid by the clinic) and is payable by the dentist to the clinic at the account above.",
	}
}

func MethodCIndependentContractorConfig() BillingMethodView {
	return BillingMethodView{
		PatientFeesLabel:       "1. PATIENT FEES COLLECTED ON YOUR BEHALF",
		ServiceFeeSectionLabel: "4. DENTIST COMMISSION (Independent Contractor)",
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
}

func MapBillingMethod(method string) BillingMethodView {
	switch BillingMethod(method) {
	case MethodSFA_DENTIST_COLLECTS:
		return MethodBDentistCollectConfig()
	case MethodINDEPENDENT_CONTRACTOR:
		return MethodCIndependentContractorConfig()
	default: // MethodSFA_CLINIC_COLLECTS or unrecognized defaults to method A
		return MethodSfaClinicCollectConfig()
	}
}

type TextResolutionStrategy func(values ...string) string

func CoalesceText(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type FooterNoteResolver struct {
	Notes             string
	TemplateTermsText string
	MethodDefault     string
}

func (r FooterNoteResolver) Resolve() string {
	return CoalesceText(r.Notes, r.TemplateTermsText, r.MethodDefault)
}

type PaymentTermsResolver struct {
	PaymentTerms                 string
	TemplateSettingsPaymentTerms string
}

func (r PaymentTermsResolver) Resolve() string {
	return CoalesceText(r.PaymentTerms, r.TemplateSettingsPaymentTerms)
}

type TemplateDataBuilder struct {
	Method                       string
	Notes                        string
	TemplateTermsText            string
	PaymentTerms                 string
	TemplateSettingsPaymentTerms string
	BaseData                     map[string]interface{}
}

func (b TemplateDataBuilder) Build() map[string]interface{} {
	bm := MapBillingMethod(b.Method)
	if bm.ServiceFeeSectionLabel == "" {
		bm.ServiceFeeSectionLabel = "4. SERVICE & FACILITY FEE"
	}

	data := make(map[string]interface{}, len(b.BaseData)+4)
	for k, v := range b.BaseData {
		data[k] = v
	}

	footerNoteResolver := FooterNoteResolver{
		Notes:             b.Notes,
		TemplateTermsText: b.TemplateTermsText,
		MethodDefault:     bm.DefaultFooterNote,
	}

	paymentTermsResolver := PaymentTermsResolver{
		PaymentTerms:                 b.PaymentTerms,
		TemplateSettingsPaymentTerms: b.TemplateSettingsPaymentTerms,
	}

	data["billing_method"] = bm
	data["footer_note"] = footerNoteResolver.Resolve()
	data["payment_terms_resolved"] = paymentTermsResolver.Resolve()

	// TaxInvoiceHTML branches on {{#if is_method_b}} directly (in addition to
	// billing_method.show_payment_details), so it must be set explicitly here
	// rather than left to whatever GetInvoiceRenderData did or didn't set.
	data["is_method_b"] = BillingMethod(b.Method) == MethodSFA_DENTIST_COLLECTS
	data["is_method_c"] = BillingMethod(b.Method) == MethodINDEPENDENT_CONTRACTOR

	return data
}
