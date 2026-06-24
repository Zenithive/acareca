package template

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
)

// RqGlobalTemplate represents incoming global template structural creation/updates
type RqGlobalTemplate struct {
	Name      string `json:"name" validate:"required,min=1,max=100"`
	Html      string `json:"html" validate:"required"`
	Css       string `json:"css" validate:"required"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

type RqTemplate struct {
	Id          uuid.UUID `json:"-"`
	Description *string   `json:"description"`
	Name        string    `json:"name" validate:"required,min=1,max=100"`
	Html        string    `json:"html" validate:"required"`
	Css         string    `json:"css" validate:"required"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`
}

func (rq *RqTemplate) ToDB(encryptionKey []byte) (Template, error) {
	htmlBlob, err := crypto.EncryptAndCompress(rq.Html, encryptionKey)
	if err != nil {
		return Template{}, err
	}

	cssBlob, err := crypto.EncryptAndCompress(rq.Css, encryptionKey)
	if err != nil {
		return Template{}, err
	}

	return Template{
		Id:          rq.Id,
		Name:        rq.Name,
		Description: rq.Description,
		Html:        htmlBlob,
		Css:         cssBlob,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}, nil
}

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

type RqUpdateSetting struct {
	Id               uuid.UUID  `json:"id"`
	TemplateId       uuid.UUID  `json:"template_id"`
	MappingId        *uuid.UUID `json:"mapping_id"`
	PrimaryColor     string     `json:"primary_color"`
	AccentColor      string     `json:"accent_color"`
	BodyFontFamily   string     `json:"body_font_family"`
	HeaderFontFamily string     `json:"header_font_family"`
	IsLogo           bool       `json:"is_logo"`
	Logo             *uuid.UUID `json:"logo"`
	LetterHead       *uuid.UUID `json:"letter_head"`
	Footer           *uuid.UUID `json:"footer"`
	TermText         *string    `json:"term_text"`
	IsWaterMark      bool       `json:"is_water_mark"`
	WaterMarkText    *string    `json:"water_mark_text"`
	IsTax            bool       `json:"is_tax"`
	TableStyle       string     `json:"table_style"`
}

func (rq *RqUpdateSetting) ToDB() Setting {
	var tableStyle *string
	if rq.TableStyle != "" {
		ts := rq.TableStyle
		tableStyle = &ts
	}

	return Setting{
		Id:               rq.Id,
		TemplateId:       rq.TemplateId,
		MappingId:        rq.MappingId,
		PrimaryColor:     rq.PrimaryColor,
		AccentColor:      rq.AccentColor,
		BodyFontFamily:   rq.BodyFontFamily,
		HeaderFontFamily: rq.HeaderFontFamily,
		IsLogo:           rq.IsLogo,
		LogoId:           rq.Logo,
		LetterHeadId:     rq.LetterHead,
		FooterId:         rq.Footer,
		TermText:         rq.TermText,
		IsWaterMark:      rq.IsWaterMark,
		WaterMarkText:    rq.WaterMarkText,
		IsTax:            rq.IsTax,
		TableStyle:       tableStyle,
	}
}

type Setting struct {
	Id               uuid.UUID  `db:"id"`
	TemplateId       uuid.UUID  `db:"template_id"`
	MappingId        *uuid.UUID `db:"mapping_id"`
	PrimaryColor     string     `db:"primary_color"`
	AccentColor      string     `db:"accent_color"`
	BodyFontFamily   string     `db:"body_font_family"`
	HeaderFontFamily string     `db:"header_font_family"`
	IsLogo           bool       `db:"is_logo"`
	LogoId           *uuid.UUID `db:"logo_id"`
	LetterHeadId     *uuid.UUID `db:"letterhead_id"`
	FooterId         *uuid.UUID `db:"footer_id"`
	TermText         *string    `db:"terms_text"`
	IsWaterMark      bool       `db:"is_watermark"`
	WaterMarkText    *string    `db:"watermark_text"`
	IsTax            bool       `db:"is_tax"`
	TableStyle       *string    `db:"table_style"`

	Logo       *file.Document `db:"-"`
	LetterHead *file.Document `db:"-"`
	Footer     *file.Document `db:"-"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (st *Setting) ToRs() RsSetting {
	var logo *file.RsDocument
	if st.Logo != nil {
		logo = st.Logo.ToRsDocument()
	}

	var letterhead *file.RsDocument
	if st.LetterHead != nil {
		letterhead = st.LetterHead.ToRsDocument()
	}

	var footer *file.RsDocument
	if st.Footer != nil {
		footer = st.Footer.ToRsDocument()
	}

	tableStyle := ""
	if st.TableStyle != nil {
		tableStyle = *st.TableStyle
	}

	return RsSetting{
		Id:               st.Id,
		TemplateId:       st.TemplateId,
		MappingId:        st.MappingId,
		PrimaryColor:     st.PrimaryColor,
		AccentColor:      st.AccentColor,
		BodyFontFamily:   st.BodyFontFamily,
		HeaderFontFamily: st.HeaderFontFamily,
		IsLogo:           st.IsLogo,
		Logo:             logo,
		LetterHead:       letterhead,
		Footer:           footer,
		TermText:         st.TermText,
		IsWaterMark:      st.IsWaterMark,
		WaterMarkText:    st.WaterMarkText,
		IsTax:            st.IsTax,
		TableStyle:       tableStyle,
		CreatedAt:        st.CreatedAt,
		UpdatedAt:        st.UpdatedAt,
	}
}

type RsSetting struct {
	Id               uuid.UUID        `json:"id"`
	TemplateId       uuid.UUID        `json:"template_id"`
	MappingId        *uuid.UUID       `json:"mapping_id,omitempty"`
	PrimaryColor     string           `json:"primary_color"`
	AccentColor      string           `json:"accent_color"`
	BodyFontFamily   string           `json:"body_font_family"`
	HeaderFontFamily string           `json:"header_font_family"`
	IsLogo           bool             `json:"is_logo"`
	Logo             *file.RsDocument `json:"logo"`
	LetterHead       *file.RsDocument `json:"letter_head"`
	Footer           *file.RsDocument `json:"footer"`
	TermText         *string          `json:"term_text"`
	IsWaterMark      bool             `json:"is_water_mark"`
	WaterMarkText    *string          `json:"water_mark_text"`
	IsTax            bool             `json:"is_tax"`
	TableStyle       string           `json:"table_style"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        *time.Time       `json:"updated_at"`
}

type RqGeneratePDF struct {
	TemplateId uuid.UUID
	ClinicId   uuid.UUID
	Data       InvoiceData
}

type InvoiceData struct {
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
	ShowTax              bool                   `json:"show_tax"`
	LetterheadHTML       string                 `json:"letterhead_html"`
	FooterHTML           string                 `json:"footer_html"`
	Notes                string                 `json:"notes"`
	AmountInWords        string                 `json:"amount_in_words"`
	HasAttachments       bool                   `json:"has_attachments"`
	BillFrom             PartyInfo              `json:"bill_from"`
	BillTo               PartyInfo              `json:"bill_to"`
	Items                []LineItem             `json:"items"`
	GrandTotal           float64                `json:"grand_total"`
	Subtotal             float64                `json:"subtotal"`
	TaxTotal             float64                `json:"tax_total"`
	TotalsAmountsCaption string                 `json:"totals_amounts_caption"`
	TotalsGrandLabel     string                 `json:"totals_grand_label"`
	TableStyleClass      string                 `json:"table_style_class"`
	TemplateSettings     map[string]interface{} `json:"template_settings"`
	Attachments          []Attachment           `json:"attachments"`

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

type invoiceCollections struct {
	patientFeeItems []map[string]interface{}
	serviceFeeItems []map[string]interface{}
	settlementItems []map[string]interface{}
	remittanceItems []map[string]interface{}

	serviceFeeRateIntro     map[string]interface{}
	serviceDescriptionItems []string

	subtotal   float64
	taxTotal   float64
	grandTotal float64

	customFeeRate string
}

type invoicePaymentMeta struct {
	paymentMethod    string
	accountName      string
	bsb              string
	accountNumber    string
	paymentDate      string
	paymentReference string
}

type PartyInfo struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	ABN           string `json:"abn"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	PaymentMethod string `json:"payment_method"`
}

type LineItem struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Amount       float64 `json:"amount"`
	RunningTotal float64 `json:"running_total"`
}

type Attachment struct {
	FileName string `json:"file_name"`
}

type InvoiceResponse struct {
	ID                uuid.UUID      `json:"id"`
	ClinicID          uuid.UUID      `json:"clinic_id"`
	ClinicName        string         `json:"clinic_name"`
	TemplateID        uuid.UUID      `json:"template_id"`
	BillingPeriodFrom string         `json:"billing_period_from"`
	BillingPeriodTo   string         `json:"billing_period_to"`
	InvoiceFrequency  string         `json:"invoice_frequency"`
	IssueDate         string         `json:"issue_date"`
	DueDate           string         `json:"due_date"`
	Status            string         `json:"status"`
	SentBy            InvoiceContact `json:"sent_by"`
	SentTo            InvoiceContact `json:"sent_to"`
	Items             []InvoiceItem  `json:"items"`
	InvoiceNumber     string         `json:"invoice_number"`
}

type InvoiceContact struct {
	ID      uuid.UUID `db:"id" json:"id"`
	FName   string    `db:"fname" json:"fname"`
	LName   string    `db:"lname" json:"lname"`
	Phone   string    `db:"phone" json:"phone"`
	Email   string    `db:"email" json:"email"`
	ABN     string    `db:"abn" json:"abn"`
	Address []string  `db:"address" json:"address"`
}

type InvoiceSectionMeta struct {
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

type InvoiceItem struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Amount      float64   `db:"amount" json:"amount"`
	BASCode     *string   `db:"bas_code" json:"bas_code"`
	EntryType   string    `db:"entry_type" json:"entry_type"`
	SectionType string    `db:"section_type" json:"section_type"`
	FieldKey    *string   `db:"field_key" json:"field_key"`
	IsFinal     bool      `db:"is_final" json:"is_final"`
}

// InvoiceToData works for external callers
func InvoiceToData(inv *InvoiceResponse) InvoiceData {
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

	billTo := PartyInfo{
		Name:  inv.SentTo.FName + " " + inv.SentTo.LName,
		Email: inv.SentTo.Email,
		Phone: inv.SentTo.Phone,
		ABN:   inv.SentTo.ABN,
	}
	if len(inv.SentTo.Address) > 0 {
		billTo.Address = inv.SentTo.Address[0]
	}

	billFrom := PartyInfo{
		Name:  inv.ClinicName,
		Email: inv.SentBy.Email,
		Phone: inv.SentBy.Phone,
		ABN:   inv.SentBy.ABN,
	}
	if len(inv.SentBy.Address) > 0 {
		billFrom.Address = inv.SentBy.Address[0]
	}

	formattedIssueDate := FormatDateString(inv.IssueDate)

	var formattedBillingPeriod string
	if inv.BillingPeriodFrom != "" && inv.BillingPeriodTo != "" {
		formattedBillingPeriod = fmt.Sprintf("%s to %s", FormatDateString(inv.BillingPeriodFrom), FormatDateString(inv.BillingPeriodTo))
	} else {
		formattedBillingPeriod = inv.BillingPeriodFrom + " to " + inv.BillingPeriodTo
	}

	return InvoiceData{
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
		Attachments:          []Attachment{},
	}
}

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

// invoiceRow acts as a private alias so template/repository.go compiles cleanly
type invoiceRow = InvoiceRow

func FormatDateString(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("02 January 2006")
}

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
