package defaults

// BillingMethod represents the type of billing arrangement
type BillingMethod string

const (
	// MethodSFA_CLINIC_COLLECTS - Patient fees collected by clinic, dentist gets remittance
	MethodSFA_CLINIC_COLLECTS BillingMethod = "SFA_CLINIC_COLLECTS"
	// MethodSFA_DENTIST_COLLECTS - Patient fees collected by dentist directly
	MethodSFA_DENTIST_COLLECTS BillingMethod = "SFA_DENTIST_COLLECTS"
	// MethodINDEPENDENT_CONTRACTOR - RCTI (Recipient Created Tax Invoice) with commission
	MethodINDEPENDENT_CONTRACTOR BillingMethod = "INDEPENDENT_CONTRACTOR"
)

// BillingMethodView holds every value that varies by billing method
// (a/b/c), resolved once in Go instead of branched inline in Handlebars.
// Add a new field + switch case here when a new method is introduced —
// no template file needs to change.
type BillingMethodView struct {
	// Section labels
	PatientFeesLabel       string
	ServiceFeeSectionLabel string

	// Rate display
	RateLabel   string // "Fee rate" vs "Commission rate"
	HideFeeRate bool

	// Tax invoice specific
	TaxInvoiceIntro    string
	TaxInvoiceTitle    string
	InvoiceNumberLabel string

	// Address banner
	BillToLabel  string // "BILL TO" vs "SUPPLIER (DENTIST)"
	ShowRCTINote bool

	// Features
	ShowServiceDescription bool
	ShowPaymentDetails     bool
	ShowRemittance         bool

	// References
	PageTwoLabel string // "Tax Invoice (page 2)" vs "RCTI (page 2)"

	// Default text
	DefaultFooterNote string // used only if no custom notes/terms supplied
}

// MethodSfaClinicCollectConfig returns configuration for Method A billing
// func MethodSfaClinicCollectConfig() BillingMethodView {
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

// MethodBConfig returns configuration for Method B billing
func MethodBDentistCollectConfig() BillingMethodView {
	return BillingMethodView{
		PatientFeesLabel:       "1. PATIENT FEES",
		ServiceFeeSectionLabel: "2. SERVICE & FACILITY FEE",
		RateLabel:              "Fee rate",
		TaxInvoiceIntro:        "", // filled from service_fee_rate_intro.label in template data
		HideFeeRate:            false,
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
}

// MethodCConfig returns configuration for Method C billing (RCTI)
func MethodCIndependentContractorConfig() BillingMethodView {
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
}

// ResolveBillingMethod returns the appropriate billing method configuration
func ResolveBillingMethod(method string) BillingMethodView {
	switch BillingMethod(method) {
	case MethodSFA_DENTIST_COLLECTS:
		return MethodBDentistCollectConfig()
	case MethodINDEPENDENT_CONTRACTOR:
		return MethodCIndependentContractorConfig()
	default: // MethodSFA_CLINIC_COLLECTS or unrecognized defaults to method A
		return MethodSfaClinicCollectConfig()
	}
}

// TextResolutionStrategy defines the priority order for text resolution
type TextResolutionStrategy func(values ...string) string

// CoalesceText returns the first non-empty string from the provided values
func CoalesceText(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// FooterNoteResolver resolves footer note text with fallback chain:
// explicit invoice notes > template setting terms text > method default
type FooterNoteResolver struct {
	Notes             string
	TemplateTermsText string
	MethodDefault     string
}

// Resolve returns the resolved footer note
func (r FooterNoteResolver) Resolve() string {
	return CoalesceText(r.Notes, r.TemplateTermsText, r.MethodDefault)
}

// PaymentTermsResolver resolves payment terms text with fallback chain:
// explicit payment_terms > template setting payment terms
type PaymentTermsResolver struct {
	PaymentTerms                 string
	TemplateSettingsPaymentTerms string
}

// Resolve returns the resolved payment terms
func (r PaymentTermsResolver) Resolve() string {
	return CoalesceText(r.PaymentTerms, r.TemplateSettingsPaymentTerms)
}

// TemplateDataBuilder builds the complete data map for Handlebars template execution
type TemplateDataBuilder struct {
	Method                       string
	Notes                        string
	TemplateTermsText            string
	PaymentTerms                 string
	TemplateSettingsPaymentTerms string
	BaseData                     map[string]interface{}
}

// Build assembles the full data map handed to Handlebars.Exec
func (b TemplateDataBuilder) Build() map[string]interface{} {
	bm := ResolveBillingMethod(b.Method)

	data := make(map[string]interface{}, len(b.BaseData)+3)
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

	return data
}

// BuildTemplateData assembles the full data map handed to Handlebars.Exec
// for a given invoice render. This is the single place billing-method
// branching, fallback resolution, and template variables converge —
// every .go template file downstream only reads flat fields off this map.
//
// Deprecated: Use TemplateDataBuilder instead for better testability
func BuildTemplateData(method string, notes, templateTermsText, paymentTerms, templateSettingsPaymentTerms string, base map[string]interface{}) map[string]interface{} {
	builder := TemplateDataBuilder{
		Method:                       method,
		Notes:                        notes,
		TemplateTermsText:            templateTermsText,
		PaymentTerms:                 paymentTerms,
		TemplateSettingsPaymentTerms: templateSettingsPaymentTerms,
		BaseData:                     base,
	}
	return builder.Build()
}
