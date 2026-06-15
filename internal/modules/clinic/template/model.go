package template

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
)

type RqTemplate struct {
	Id          uuid.UUID `json:"-"`
	ClinicId    uuid.UUID `json:"clinic_id"`
	Description *string   `json:"description"`
	Name        string    `json:"name"`
	Html        string    `json:"html"`
	Css         string    `json:"css"`
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
		ClinicId:    rq.ClinicId,
		Description: rq.Description,
		Html:        htmlBlob,
		Css:         cssBlob,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}, nil
}

type Template struct {
	Id          uuid.UUID `db:"id"`
	ClinicId    uuid.UUID `db:"clinic_id"`
	Description *string   `db:"description"`
	Name        string    `db:"name"`
	Html        []byte    `db:"html"`
	Css         []byte    `db:"css"`
	IsDefault   bool      `db:"is_default"`
	IsActive    bool      `db:"is_active"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (tp *Template) ToRs() RsTemplate {
	return RsTemplate{
		Id:          tp.Id,
		ClinicId:    tp.ClinicId,
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
	Id          uuid.UUID `json:"id"`
	ClinicId    uuid.UUID `json:"clinic_id"`
	Description *string   `json:"description"`
	Name        string    `json:"name"`
	Html        string    `json:"html"`
	Css         string    `json:"css"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type RqUpdateSetting struct {
	Id               uuid.UUID  `json:"id"`
	TemplateId       uuid.UUID  `json:"template_id"`
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
		tableStyle = &rq.TableStyle
	}

	return Setting{
		Id:               rq.Id,
		TemplateId:       rq.TemplateId,
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

	// These are populated separately via joins or additional queries
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
		rs := st.Logo.ToRsDocument()
		logo = rs
	}

	var letterhead *file.RsDocument
	if st.LetterHead != nil {
		rs := st.LetterHead.ToRsDocument()
		letterhead = rs
	}

	var footer *file.RsDocument
	if st.Footer != nil {
		rs := st.Footer.ToRsDocument()
		footer = rs
	}

	tableStyle := ""
	if st.TableStyle != nil {
		tableStyle = *st.TableStyle
	}

	return RsSetting{
		Id:               st.Id,
		TemplateId:       st.TemplateId,
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

		CreatedAt: st.CreatedAt,
		UpdatedAt: st.UpdatedAt,
	}
}

type RsSetting struct {
	Id               uuid.UUID        `json:"id"`
	TemplateId       uuid.UUID        `json:"template_id"`
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

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type RqGeneratePDF struct {
	TemplateId uuid.UUID
	ClinicId   uuid.UUID
	Data       InvoiceData
}

// InvoiceData holds all Handlebars variables the templates expect
type InvoiceData struct {
	ClinicName       string `json:"clinic_name"`
	IssueDateDisplay string `json:"issue_date_display"`
	DueDateDisplay   string `json:"due_date_display"`
	BillingPeriod    string `json:"billing_period"`
	InvoiceFrequency string `json:"invoice_frequency"`
	ShowLogo         bool   `json:"show_logo"`
	ShowLogoImage    bool   `json:"show_logo_image"`
	LogoURL          string `json:"logo_url"`
	LogoInitial      string `json:"logo_initial"`
	WatermarkEnabled bool   `json:"watermark_enabled"`
	WatermarkText    string `json:"watermark_text"`
	ShowTax          bool   `json:"show_tax"`
	LetterheadHTML   string `json:"letterhead_html"`
	FooterHTML       string `json:"footer_html"`
	Notes            string `json:"notes"`
	AmountInWords    string `json:"amount_in_words"`
	HasAttachments   bool   `json:"has_attachments"`

	BillFrom PartyInfo  `json:"bill_from"`
	BillTo   PartyInfo  `json:"bill_to"`
	Items    []LineItem `json:"items"`

	GrandTotal float64 `json:"grand_total"`

	TotalsAmountsCaption string `json:"totals_amounts_caption"`
	TotalsGrandLabel     string `json:"totals_grand_label"`

	TableStyleClass string `json:"table_style_class"`

	Attachments []Attachment `json:"attachments"`

	// Settings-derived — injected by service before render
	PrimaryColor     string `json:"-"`
	AccentColor      string `json:"-"`
	BodyFontFamily   string `json:"-"`
	HeaderFontFamily string `json:"-"`
}

type PartyInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	ABN     string `json:"abn"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
}

type LineItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	UnitPrice   float64 `json:"unit_price"`
	Qty         int     `json:"qty"`
	LineTotal   float64 `json:"line_total"`
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
	SentTo            InvoiceContact `json:"sent_to"`
	Items             []InvoiceItem  `json:"items"`
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

type InvoiceItem struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Quantity    int       `db:"quantity" json:"quantity"`
	UnitPrice   float64   `db:"unit_price" json:"unit_price"`
	TotalAmount float64   `db:"total_amount" json:"total_amount"`
}

func invoiceToData(inv *InvoiceResponse) InvoiceData {
	items := make([]LineItem, len(inv.Items))
	var grandTotal float64

	for i, it := range inv.Items {
		grandTotal += it.TotalAmount
		items[i] = LineItem{
			Name:        it.Name,
			Description: it.Description,
			UnitPrice:   it.UnitPrice,
			Qty:         it.Quantity,
			LineTotal:   it.TotalAmount,
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

	return InvoiceData{
		ClinicName:       inv.ClinicName,
		IssueDateDisplay: inv.IssueDate,
		DueDateDisplay:   inv.DueDate,
		BillingPeriod:    inv.BillingPeriodFrom + " to " + inv.BillingPeriodTo,
		InvoiceFrequency: inv.InvoiceFrequency,
		BillFrom: PartyInfo{
			Name: inv.ClinicName,
		},
		BillTo:               billTo,
		Items:                items,
		GrandTotal:           grandTotal,
		TotalsAmountsCaption: "All amounts in INR",
		TotalsGrandLabel:     "Total Due",
		HasAttachments:       false,
		Attachments:          []Attachment{},
	}
}

type invoiceRow struct {
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
