package common

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// Constants for size limits
const (
	MaxTemplateSizeBytes = 5 * 1024 * 1024  // 5MB per template
	MaxTotalSizeBytes    = 10 * 1024 * 1024 // 10MB total
	MaxTemplateCount     = 10               // Max templates per request
)

type TemplateMethod struct {
	Name          string
	TemplateNames []string
	PageOrder     map[string]int
}

var (
	MethodSFAClinicCollects = TemplateMethod{
		Name: "SFA_CLINIC_COLLECTS",
		TemplateNames: []string{
			"Calculation Statement",
			"Tax Invoice",
			"Remittance Advice",
		},
		PageOrder: map[string]int{
			"Calculation Statement": 1,
			"Tax Invoice":           2,
			"Remittance Advice":     3,
		},
	}

	MethodSFADentistCollects = TemplateMethod{
		Name: "SFA_DENTIST_COLLECTS",
		TemplateNames: []string{
			"Calculation Statement",
			"Tax Invoice",
		},
		PageOrder: map[string]int{
			"Calculation Statement": 1,
			"Tax Invoice":           2,
		},
	}

	MethodIndependentContractor = TemplateMethod{
		Name: "INDEPENDENT_CONTRACTOR",
		TemplateNames: []string{
			"Calculation Statement",
			"Recipient Created Tax Invoice",
			"Remittance Advice",
		},
		PageOrder: map[string]int{
			"Calculation Statement":         1,
			"Recipient Created Tax Invoice": 2,
			"Remittance Advice":             3,
		},
	}

	MethodRegistry = map[string]TemplateMethod{
		"SFA_CLINIC_COLLECTS":    MethodSFAClinicCollects,
		"SFA_DENTIST_COLLECTS":   MethodSFADentistCollects,
		"INDEPENDENT_CONTRACTOR": MethodIndependentContractor,
	}
)

func GetMethod(name string) (TemplateMethod, bool) {
	method, ok := MethodRegistry[name]
	return method, ok
}

func GetPageOrder(methodName string) map[string]int {
	method, ok := GetMethod(methodName)
	if !ok {
		return MethodSFAClinicCollects.PageOrder
	}
	return method.PageOrder
}

func GetTemplateNames(methodName string) []string {
	method, ok := GetMethod(methodName)
	if !ok {
		return nil
	}
	return method.TemplateNames
}

func AllMethods() []string {
	methods := make([]string, 0, len(MethodRegistry))
	for name := range MethodRegistry {
		methods = append(methods, name)
	}
	return methods
}

type RsTemplate struct {
	Id          uuid.UUID  `json:"id"`
	Description *string    `json:"description"`
	Name        string     `json:"name"`
	Html        string     `json:"html"`
	Css         string     `json:"css"`
	IsDefault   bool       `json:"is_default"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

// Template represents a template entity
type Template struct {
	Id          uuid.UUID  `db:"id"`
	Description *string    `db:"description"`
	Name        string     `db:"name"`
	Html        []byte     `db:"html"`
	Css         []byte     `db:"css"`
	IsDefault   bool       `db:"is_default"`
	IsActive    bool       `db:"is_active"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (tp *Template) ToRs() RsTemplate {
	return RsTemplate{
		Id:          tp.Id,
		Description: tp.Description,
		Name:        tp.Name,
		Html:        "",
		Css:         "",
		IsDefault:   tp.IsDefault,
		IsActive:    tp.IsActive,
		CreatedAt:   tp.CreatedAt,
		UpdatedAt:   tp.UpdatedAt,
	}
}

// Setting represents template settings
type Setting struct {
	Id               uuid.UUID  `db:"id"`
	InvoiceId        *uuid.UUID `db:"invoice_id"`
	PrimaryColor     string     `db:"primary_color"`
	AccentColor      string     `db:"accent_color"`
	BodyFontFamily   string     `db:"body_font_family"`
	HeaderFontFamily string     `db:"header_font_family"`
	IsLogo           bool       `db:"is_logo"`
	LogoId           *uuid.UUID `db:"logo_id"`
	TermText         *string    `db:"terms_text"`
	PaymentTerms     *string    `db:"payment_terms"`
	IsWaterMark      bool       `db:"is_watermark"`
	WaterMarkText    *string    `db:"watermark_text"`
	TableStyle       *string    `db:"table_style"`

	Logo *Document `db:"-"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type RsSetting struct {
	Id               uuid.UUID   `json:"id"`
	InvoiceId        *uuid.UUID  `json:"invoice_id,omitempty"`
	PrimaryColor     string      `json:"primary_color"`
	AccentColor      string      `json:"accent_color"`
	BodyFontFamily   string      `json:"body_font_family"`
	HeaderFontFamily string      `json:"header_font_family"`
	IsLogo           bool        `json:"is_logo"`
	Logo             *RsDocument `json:"logo"`
	TermText         *string     `json:"term_text"`
	PaymentTerms     *string     `json:"payment_terms"`
	IsWaterMark      bool        `json:"is_water_mark"`
	WaterMarkText    *string     `json:"water_mark_text"`
	TableStyle       string      `json:"table_style"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        *time.Time  `json:"updated_at"`
}

func (st *Setting) ToRs() RsSetting {
	var logo *RsDocument
	if st.Logo != nil {
		logo = st.Logo.ToRsDocument()
	}

	tableStyle := ""
	if st.TableStyle != nil {
		tableStyle = *st.TableStyle
	}

	return RsSetting{
		Id:               st.Id,
		InvoiceId:        st.InvoiceId,
		PrimaryColor:     st.PrimaryColor,
		AccentColor:      st.AccentColor,
		BodyFontFamily:   st.BodyFontFamily,
		HeaderFontFamily: st.HeaderFontFamily,
		IsLogo:           st.IsLogo,
		Logo:             logo,
		TermText:         st.TermText,
		PaymentTerms:     st.PaymentTerms,
		IsWaterMark:      st.IsWaterMark,
		WaterMarkText:    st.WaterMarkText,
		TableStyle:       tableStyle,
		CreatedAt:        st.CreatedAt,
		UpdatedAt:        st.UpdatedAt,
	}
}

// Mapping represents template-setting mappings
type Mapping struct {
	ID         uuid.UUID  `db:"id"`
	ClinicID   *uuid.UUID `db:"clinic_id"`
	InvoiceID  *uuid.UUID `db:"invoice_id"`
	TemplateID uuid.UUID  `db:"template_id"`
	SettingID  uuid.UUID  `db:"setting_id"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at"`
}

// InvoiceContact represents contact information for invoices
type InvContact struct {
	ID      uuid.UUID `db:"id" json:"id"`
	FName   string    `db:"fname" json:"fname"`
	LName   string    `db:"lname" json:"lname"`
	Phone   string    `db:"phone" json:"phone"`
	Email   string    `db:"email" json:"email"`
	ABN     string    `db:"abn" json:"abn"`
	Address []string  `db:"address" json:"address"`
}

// InvoiceItem represents an item on an invoice
type InvItem struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Amount      float64   `db:"amount" json:"amount"`
	BASCode     *string   `db:"bas_code" json:"bas_code"`
	EntryType   string    `db:"entry_type" json:"entry_type"`
	SectionType string    `db:"section_type" json:"section_type"`
	FieldKey    *string   `db:"field_key" json:"field_key"`
	Expression  *string   `db:"expression" json:"expression"`
	IsFinal     bool      `db:"is_final" json:"is_final"`
	SortOrder   int       `db:"sort_order" json:"sort_order"`
}

// InvoiceResponse represents a complete invoice with all related data
type RsInvoice struct {
	ID                uuid.UUID  `json:"id"`
	ClinicID          uuid.UUID  `json:"clinic_id"`
	ClinicName        string     `json:"clinic_name"`
	TemplateID        uuid.UUID  `json:"template_id"`
	BillingPeriodFrom string     `json:"billing_period_from"`
	BillingPeriodTo   string     `json:"billing_period_to"`
	InvoiceFrequency  string     `json:"invoice_frequency"`
	IssueDate         string     `json:"issue_date"`
	DueDate           string     `json:"due_date"`
	Status            string     `json:"status"`
	SentBy            InvContact `json:"sent_by"`
	SentTo            InvContact `json:"sent_to"`
	Items             []InvItem  `json:"items"`
	InvoiceNumber     string     `json:"invoice_number"`
}

// InvoiceSectionMeta represents invoice section metadata
type RsInvoiceSectionMeta struct {
	ID               uuid.UUID `db:"id"`
	SectionType      string    `db:"section_type"`
	DocumentNumber   string    `db:"document_number"`
	PaymentMethod    *string   `db:"payment_method"`
	AccountName      *string   `db:"account_name"`
	Bsb              *string   `db:"bsb_number"`
	AccountNumber    *string   `db:"account_number"`
	PaymentDate      *string   `db:"payment_date"`
	PaymentReference *string   `db:"payment_reference"`
}

// InvoiceRow represents a database row for invoice queries
type InvoiceRow struct {
	Id                uuid.UUID `db:"id"`
	ClinicId          uuid.UUID `db:"clinic_id"`
	TemplateId        uuid.UUID `db:"template_id"`
	BillingPeriodFrom string    `db:"billing_period_from"`
	BillingPeriodTo   string    `db:"billing_period_to"`
	InvoiceFrequency  string    `db:"invoice_frequency"`
	IssueDate         string    `db:"issue_date"`
	DueDate           string    `db:"due_date"`
	Status            string    `db:"status"`
	FName             string    `db:"fname"`
	LName             string    `db:"lname"`
	Email             string    `db:"email"`
	Phone             string    `db:"phone"`
	ABN               string    `db:"abn"`
	ClinicName        string    `db:"clinic_name"`
	AddressLine1      string    `db:"address_line1"`
	City              string    `db:"city"`
	State             string    `db:"state"`
	PostalCode        string    `db:"postal_code"`
	Country           string    `db:"country"`
}

type Invoice struct {
	Id                   uuid.UUID              `json:"invoice_id"`
	InvoiceNumber        string                 `json:"invoice_number"`
	ClinicName           string                 `json:"clinic_name"`
	IssueDateDisplay     string                 `json:"issue_date_display"`
	DueDateDisplay       string                 `json:"due_date_display"`
	BillingPeriod        string                 `json:"billing_period"`
	InvoiceFrequency     string                 `json:"invoice_frequency"`
	ShowLogo             bool                   `json:"show_logo"`
	ShowLogoImage        bool                   `json:"show_logo_image"`
	LogoURL              string                 `json:"logo_url"`
	LogoInitial          string                 `json:"logo_initial"`
	WatermarkEnabled     bool                   `json:"watermark_enabled"`
	WatermarkText        string                 `json:"watermark_text"`
	Notes                string                 `json:"notes"`
	AmountInWords        string                 `json:"amount_in_words"`
	HasAttachments       bool                   `json:"has_attachments"`
	BillFrom             InvContact             `json:"bill_from"`
	BillTo               InvContact             `json:"bill_to"`
	Items                []LineItem             `json:"items"`
	GrandTotal           float64                `json:"grand_total"`
	Subtotal             float64                `json:"subtotal"`
	TaxTotal             float64                `json:"tax_total"`
	TotalsAmountsCaption string                 `json:"totals_amounts_caption"`
	TotalsGrandLabel     string                 `json:"totals_grand_label"`
	TableStyleClass      string                 `json:"table_style_class"`
	TemplateSettings     map[string]interface{} `json:"template_settings"`

	// Exported styling fields targetable by structural assignments
	PrimaryColor     string `json:"primary_color"`
	AccentColor      string `json:"accent_color"`
	BodyFontFamily   string `json:"body_font_family"`
	HeaderFontFamily string `json:"header_font_family"`

	// Calculation Sheet Collections
	PatientFeeItems         []map[string]interface{} `json:"patient_fee_items"`
	ServiceFeeItems         []map[string]interface{} `json:"service_fee_items"`
	SettlementItems         []map[string]interface{} `json:"settlement_items"`
	ServiceFeeRateIntro     map[string]interface{}   `json:"service_fee_rate_intro"`
	ServiceDescriptionItems []string                 `json:"service_description_items"`
	CustomFeeRate           string                   `json:"custom_fee_rate"`

	// Tax Invoice Collections
	TaxInvoiceItems []map[string]interface{} `json:"tax_invoice_items"`
	TermsText       string                   `json:"terms_text"`

	// Remittance Advice Collections
	RemittanceItems          []map[string]interface{} `json:"remittance_items"`
	CustomPaymentMethod      string                   `json:"custom_payment_method"`
	PaymentMethodLabel       string                   `json:"payment_method_label"`
	CustomPaymentAccountName string                   `json:"custom_payment_account_name"`
	CustomPaymentBsb         string                   `json:"custom_payment_bsb"`
	CustomPaymentAccount     string                   `json:"custom_payment_account"`
	PaymentDateDisplay       string                   `json:"payment_date_display"`
	// Tax Invoice fee rate display (e.g. "38.5%")
	CustomFeeRateDisplay string `json:"custom_fee_rate_display"`
}

type LineItem struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Amount       float64 `json:"amount"`
	RunningTotal float64 `json:"running_total"`
}

func InvoiceToData(inv *RsInvoice) Invoice {
	items := make([]LineItem, len(inv.Items))
	var grandTotal float64

	for i, it := range inv.Items {
		grandTotal += it.Amount
		items[i] = LineItem{
			Name:         it.Name,
			Description:  it.Description,
			Amount:       it.Amount,
			RunningTotal: grandTotal,
		}
	}

	billTo := InvContact{
		FName: inv.SentTo.FName,
		LName: inv.SentTo.LName,
		Email: inv.SentTo.Email,
		Phone: inv.SentTo.Phone,
		ABN:   inv.SentTo.ABN,
	}
	if len(inv.SentTo.Address) > 0 {
		billTo.Address = inv.SentTo.Address
	}

	billFrom := InvContact{
		FName: inv.SentBy.FName,
		LName: inv.SentBy.LName,
		Email: inv.SentBy.Email,
		Phone: inv.SentBy.Phone,
		ABN:   inv.SentBy.ABN,
	}
	if len(inv.SentBy.Address) > 0 {
		billFrom.Address = inv.SentBy.Address
	}

	formattedIssueDate := util.FormatDateString(inv.IssueDate)

	var formattedBillingPeriod string
	if inv.BillingPeriodFrom != "" && inv.BillingPeriodTo != "" {
		formattedBillingPeriod = fmt.Sprintf("%s to %s", util.FormatDateString(inv.BillingPeriodFrom), util.FormatDateString(inv.BillingPeriodTo))
	} else {
		formattedBillingPeriod = inv.BillingPeriodFrom + " to " + inv.BillingPeriodTo
	}

	return Invoice{
		ClinicName:           inv.ClinicName,
		IssueDateDisplay:     formattedIssueDate,
		DueDateDisplay:       inv.DueDate,
		BillingPeriod:        formattedBillingPeriod,
		InvoiceFrequency:     inv.InvoiceFrequency,
		BillFrom:             billFrom,
		BillTo:               billTo,
		Items:                items,
		GrandTotal:           grandTotal,
		TotalsAmountsCaption: "All amounts in AUD",
		TotalsGrandLabel:     "Total Due",
		HasAttachments:       false,
	}
}
