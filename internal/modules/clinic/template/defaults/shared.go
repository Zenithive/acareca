// Package defaults holds the raw HTML/CSS bodies for the platform's
// built-in invoice document templates. This package is intentionally
// dependency-free (no reference to template.RqGlobalTemplate) so the
// parent template package can import it without a cycle.
package defaults

// AddressBannerConfig defines configuration for address banner sections
type AddressBannerConfig struct {
	Label        string
	NameField    string
	AddressField string
	ABNField     string
	ExtraHTML    string // Optional extra HTML after ABN line (e.g., RCTI note)
}

// AddressBanner generates an address banner HTML block with the given configuration
func AddressBanner(cfg AddressBannerConfig) string {
	html := `<div class="address-banner-box"><div class="banner-label">` + cfg.Label + `</div><div class="recipient-name">{{` + cfg.NameField + `}}</div>{{#if ` + cfg.AddressField + `}}<p class="recipient-line">{{` + cfg.AddressField + `}}</p>{{/if}}{{#if ` + cfg.ABNField + `}}<p class="recipient-line">ABN {{` + cfg.ABNField + `}}</p>{{/if}}`
	if cfg.ExtraHTML != "" {
		html += cfg.ExtraHTML
	}
	html += `</div>`
	return html
}

// DefaultPreparedForBanner returns the standard "PREPARED FOR" address banner
func DefaultPreparedForBanner() string {
	return AddressBanner(AddressBannerConfig{
		Label:        "PREPARED FOR",
		NameField:    "bill_to.name",
		AddressField: "bill_to.address",
		ABNField:     "bill_to.abn",
	})
}

// TaxInvoiceBillToBanner returns the address banner for Tax Invoice with RCTI support
func TaxInvoiceBillToBanner() string {
	return AddressBanner(AddressBannerConfig{
		Label:        "{{billing_method.bill_to_label}}",
		NameField:    "bill_to.name",
		AddressField: "bill_to.address",
		ABNField:     "bill_to.abn",
		ExtraHTML:    `{{#if billing_method.show_rcti_note}}<p class="recipient-line" style="margin-top: 4px; font-size: 10px; font-style: italic;">Recipient: {{bill_from.name}} ABN {{bill_from.abn}}<br>Issued by the recipient under an RCTI agreement between the parties.</p>{{/if}}`,
	})
}

// RemittancePayeeBanner returns the address banner for Remittance Advice
func RemittancePayeeBanner() string {
	return AddressBanner(AddressBannerConfig{
		Label:        "PAYEE",
		NameField:    "bill_to.name",
		AddressField: "bill_to.address",
		ABNField:     "bill_to.abn",
	})
}

// FooterNotesSection returns the footer notes section HTML
func FooterNotesSection(noteContent string) string {
	return `<div class="footer-notes-box">
    <p style="font-style: italic; margin-bottom: 4px;{{#if footer_note}} font-style: normal;{{/if}}"><strong>Notes:</strong> ` + noteContent + `</p>
  </div>`
}

// PaymentDetailsSection returns the payment details table HTML for Tax Invoice
func PaymentDetailsSection() string {
	return `{{#if billing_method.show_payment_details}}
        <div class="payment-details-container" style="margin-top: 0px;">
          <div class="payment-details-header">PAYMENT DETAILS - PAY TO CLINIC</div>
          <table class="payment-details-table payment-details-table-bordered" style="width: 100%%; border-collapse: collapse;">
            <tbody>
              <tr>
                <td style="font-weight: bold; width: 40%%;">Account</td>
                <td style="width: 60%%;">{{bill_from.name}}</td>
              </tr>
              <tr>
                <td style="font-weight: bold;">BSB / Acc No.</td>
                <td>{{coalesce clinic_payment_details "083-000 / 98765432"}}</td>
              </tr>
              <tr>
                <td style="font-weight: bold;">Due Date</td>
                <td>14 days from issue</td>
              </tr>
              <tr>
                <td style="font-weight: bold;">Reference</td>
                <td>{{invoice_number}}</td>
              </tr>
            </tbody>
          </table>
        </div>
        {{/if}}`
}

// RemittancePaymentDetailsTable returns the payment details table for Remittance
func RemittancePaymentDetailsTable() string {
	return `<div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}">
      <tbody>
        <tr>
          <td style="font-weight: bold; width: 45%%;">Payment method</td>
          <td style="width: 55%%;">{{coalesce custom_payment_method payment_method_label "Electronic funds transfer (EFT)"}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Account name</td>
          <td>{{coalesce custom_payment_account_name bill_to.name}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">BSB / Account No.</td>
          <td>{{coalesce custom_payment_bsb "063-000"}} / {{coalesce custom_payment_account "12345678"}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment date</td>
          <td>{{coalesce payment_date_display issue_date_display}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment reference</td>
          <td>{{invoice_number}}</td>
        </tr>
      </tbody>
    </table>
  </div>`
}

// TaxSummarySection returns the tax summary table HTML
func TaxSummarySection() string {
	return `<table class="layout-table" style="font-size: 11px; line-height: 1.6;">
          <tr>
            <td style="padding: 3px 6px; text-align: left;">Subtotal (excl. GST)</td>
            <td class="num" style="padding: 3px 6px;">{{format_currency subtotal}}</td>
          </tr>
          <tr>
            <td style="padding: 3px 6px; text-align: left;">GST (10%%)</td>
            <td class="num" style="padding: 3px 6px;">{{format_currency tax_total}}</td>
          </tr>
          <tr style="font-weight: bold; background-color: rgb(from var(--accent-color) r g b / 0.45) !important;">
            <td style="padding: 5px 6px; border-top: 1px solid var(--primary-color) !important; border-bottom: 2px solid var(--primary-color) !important; text-align: left;">TOTAL (incl. GST)</td>
            <td class="num" style="padding: 5px 6px; border-top: 1px solid var(--primary-color) !important; border-bottom: 2px solid var(--primary-color) !important;">{{format_currency grand_total}}</td>
          </tr>
        </table>`
}

// ServiceFeeIntroRow returns the service fee introduction row HTML for Tax Invoice
func ServiceFeeIntroRow() string {
	return `<tr>
        <td colspan="3" style="border-bottom: none; padding-top: 5px; padding-bottom: 4px;">
          <table class="layout-table" style="width: 100%%; border-collapse: collapse;">
            <tr>
              <td style="padding: 0; color: var(--text-dark); width: 65%%; vertical-align: middle;">
                {{billing_method.tax_invoice_intro}}
                {{#unless billing_method.hide_fee_rate}}
                <span style="float: right; font-weight: bold; white-space: nowrap; margin-left: 8px;">
                  {{billing_method.rate_label}}&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
                {{/unless}}
              </td>
              <td style="width: 20%%; padding: 0;"></td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
          {{#if billing_method.show_service_description}}
            {{#if service_description_items}}
            <ol style="margin: 6px 0 0 18px; padding: 0; list-style-type: decimal; font-size: 11px; line-height: 1.5; color: var(--text-dark);">
              {{#each service_description_items}}
              <li style="margin-bottom: 2px;">{{this}}</li>
              {{/each}}
            </ol>
            {{/if}}
          {{/if}}
        </td>
      </tr>`
}

// TableConfig defines configuration for data table sections
type TableConfig struct {
	Title         string
	Columns       []TableColumn
	ItemsVariable string
}

// TableColumn defines a column in a data table
type TableColumn struct {
	Header    string
	Width     string
	Align     string // left, right, center
	FieldType string // text, amount, bas_code
}

// DataTable generates a standardized data table HTML structure
func DataTable(cfg TableConfig) string {
	html := `<table class="data-table">
    <thead>
      <tr>`

	for _, col := range cfg.Columns {
		align := col.Align
		if align == "" {
			align = "left"
		}
		html += `
        <th style="width: ` + col.Width + `; text-align: ` + align + `;">` + col.Header + `</th>`
	}

	html += `
      </tr>
    </thead>
    <tbody>
      {{#each ` + cfg.ItemsVariable + `}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>`

	for _, col := range cfg.Columns {
		align := col.Align
		if align == "" {
			align = "left"
		}

		var cellClass string
		var cellContent string

		switch col.FieldType {
		case "amount":
			cellClass = ` class="num{{#if value_class}} {{value_class}}{{/if}}"`
			cellContent = `{{format_table_amount this}}`
		case "bas_code":
			cellClass = ` class="center"`
			cellContent = `{{bas_code}}`
		default:
			cellContent = `{{label}}`
		}

		html += `
        <td` + cellClass + `>` + cellContent + `</td>`
	}

	html += `
      </tr>
      {{/each}}
    </tbody>
  </table>`

	return html
}

// Header returns the shared document header markup, parameterized by
// title, the invoice-number label, and the address banner block.
func Header(title, labelName, addressBannerHTML string) string {
	return `<table class="layout-table" style="margin-bottom: 2px; width: 100%; border-collapse: collapse;">
  <tr>
    <td style="width: 55%; vertical-align: top; padding: 0;">
      {{#if template_settings.is_logo}}
        {{#if logo_url}}
        <div style="line-height: 0; margin: 0 0 4px 0;">
          <img class="brand-logo" src="{{logo_url}}" alt="{{bill_from.name}}" />
        </div>
        {{/if}}
      {{/if}}

      <div style="margin: 0; padding: 0;">
        <h2 class="hdr-clinic-name">{{bill_from.name}}</h2>
        {{#if bill_from.address}}
        <p class="hdr-clinic-line">{{bill_from.address}}</p>
        {{/if}}
        <p class="hdr-clinic-contact">
          {{#if bill_from.abn}}ABN {{bill_from.abn}}{{/if}}{{#if bill_from.phone}} &nbsp;|&nbsp; Ph {{bill_from.phone}}{{/if}}{{#if bill_from.email}} &nbsp;|&nbsp; {{bill_from.email}}{{/if}}
        </p>
      </div>

      ` + addressBannerHTML + `
    </td>
    <td style="width: 45%; vertical-align: top; text-align: right; padding: 0;">
      <h1 class="hdr-doc-title">` + title + `</h1>
      <table class="hdr-meta" style="margin-left: auto; width: 100%; max-width: 240px; border-collapse: collapse;">
        <tbody>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>` + labelName + `</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{invoice_number}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Issue Date</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{issue_date_display}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Billing Period</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{billing_period}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Invoice Frequency</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{invoice_frequency}}</td>
          </tr>
        </tbody>
      </table>
    </td>
  </tr>
</table>`
}

// HeaderConfig defines configuration for building custom headers
type HeaderConfig struct {
	Title              string
	LabelName          string
	AddressBannerHTML  string
	ShowBillingPeriod  bool
	ShowInvoiceFreq    bool
	CustomMetadataRows []MetadataRow
}

// MetadataRow represents a custom metadata row in the header
type MetadataRow struct {
	Label string
	Value string
}

// CalculationStatementHeader returns a header specifically for Calculation Statements
// with all standard metadata fields
func CalculationStatementHeader(addressBannerHTML string) string {
	return Header("CALCULATION STATEMENT", "Statement No.", addressBannerHTML)
}

// RCTIHeader returns a header specifically for RCTI (Recipient Created Tax Invoice)
// Uses dynamic title and label from billing_method
func RCTIHeader(addressBannerHTML string) string {
	return Header("{{billing_method.tax_invoice_title}}", "{{billing_method.invoice_number_label}}", addressBannerHTML)
}

// RemittanceHeader returns a header specifically for Remittance Advice
func RemittanceHeader(addressBannerHTML string) string {
	return Header("REMITTANCE ADVICE", "Reference", addressBannerHTML)
}

// CustomHeader builds a header with configurable metadata rows
func CustomHeader(cfg HeaderConfig) string {
	metadataRows := ""

	// Add invoice number row
	metadataRows += `
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>` + cfg.LabelName + `</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{invoice_number}}</td>
          </tr>`

	// Add issue date row
	metadataRows += `
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Issue Date</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{issue_date_display}}</td>
          </tr>`

	// Conditionally add billing period
	if cfg.ShowBillingPeriod {
		metadataRows += `
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Billing Period</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{billing_period}}</td>
          </tr>`
	}

	// Conditionally add invoice frequency
	if cfg.ShowInvoiceFreq {
		metadataRows += `
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Invoice Frequency</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{invoice_frequency}}</td>
          </tr>`
	}

	// Add custom metadata rows
	for _, row := range cfg.CustomMetadataRows {
		metadataRows += `
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>` + row.Label + `</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">` + row.Value + `</td>
          </tr>`
	}

	return `<table class="layout-table" style="margin-bottom: 2px; width: 100%; border-collapse: collapse;">
  <tr>
    <td style="width: 55%; vertical-align: top; padding: 0;">
      {{#if template_settings.is_logo}}
        {{#if logo_url}}
        <div style="line-height: 0; margin: 0 0 4px 0;">
          <img class="brand-logo" src="{{logo_url}}" alt="{{bill_from.name}}" />
        </div>
        {{/if}}
      {{/if}}

      <div style="margin: 0; padding: 0;">
        <h2 class="hdr-clinic-name">{{bill_from.name}}</h2>
        {{#if bill_from.address}}
        <p class="hdr-clinic-line">{{bill_from.address}}</p>
        {{/if}}
        <p class="hdr-clinic-contact">
          {{#if bill_from.abn}}ABN {{bill_from.abn}}{{/if}}{{#if bill_from.phone}} &nbsp;|&nbsp; Ph {{bill_from.phone}}{{/if}}{{#if bill_from.email}} &nbsp;|&nbsp; {{bill_from.email}}{{/if}}
        </p>
      </div>

      ` + cfg.AddressBannerHTML + `
    </td>
    <td style="width: 45%; vertical-align: top; text-align: right; padding: 0;">
      <h1 class="hdr-doc-title">` + cfg.Title + `</h1>
      <table class="hdr-meta" style="margin-left: auto; width: 100%; max-width: 240px; border-collapse: collapse;">
        <tbody>` + metadataRows + `
        </tbody>
      </table>
    </td>
  </tr>
</table>`
}

// CSS returns the shared stylesheet used by all default document templates.
// Font-family fallback chains use the `coalesce` Handlebars helper instead
// of nested {{#if}} blocks — see registerHelpers.js / helpers.go for setup.
func CSS() string {
	return `
{{#if template_settings.header_font_family}}
@import url('https://fonts.googleapis.com/css2?family={{template_settings.header_font_family}}:wght@400;700&display=swap');
{{/if}}
{{#if template_settings.body_font_family}}
@import url('https://fonts.googleapis.com/css2?family={{template_settings.body_font_family}}:wght@400;700&display=swap');
{{/if}}
@import url('https://fonts.googleapis.com/css2?family=Arial:wght@400;700&display=swap');

:root {
  --primary-color: {{coalesce template_settings.primary_color "#1f4e5f"}};
  --accent-color: {{coalesce template_settings.accent_color "#5f96b4"}};
  --text-dark: #000000;
  --pos-green: #007a3d;
  --neg-red: #c50505;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
  font-family: '{{coalesce template_settings.body_font_family_css template_settings.body_font_family "Arial"}}', sans-serif;
  font-size: 11px;
  color: var(--text-dark);
  background: #ffffff;
  line-height: 1.4;
  -webkit-print-color-adjust: exact;
  print-color-adjust: exact;
}

.invoice-page {
  width: 210mm;
  min-height: 297mm;
  margin: 0 auto;
  background: #ffffff;
  padding: 8mm 10mm;
  position: relative;
  box-sizing: border-box;
  page-break-after: always;
}

.invoice-page:last-child {
  page-break-after: avoid;
}

{{#if template_settings.is_watermark}}
.invoice-page::before {
  content: "{{coalesce template_settings.watermark_text "PAID"}}";
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%) rotate(-45deg);
  font-size: 130px;
  font-weight: bold;
  color: rgba(0, 0, 0, 0.06);
  z-index: 9999;
  pointer-events: none;
  white-space: nowrap;
}
{{/if}}

.layout-table {
  width: 100%;
  border-collapse: collapse;
  border: none;
}

.brand-logo {
  display: block;
  max-height: 44px;
  max-width: 140px;
  object-fit: contain;
}

.hdr-clinic-name {
  font-family: '{{coalesce template_settings.header_font_family_css template_settings.header_font_family "Arial"}}', sans-serif;
  font-size: 16px;
  font-weight: bold;
  color: var(--primary-color);
  margin: 0 0 2px 0;
}

.hdr-clinic-line {
  font-size: 11px;
  color: var(--text-dark);
  margin: 0;
}

.hdr-clinic-contact {
  font-size: 11px;
  color: var(--text-dark);
  margin: 0;
}

.hdr-doc-title {
  font-family: '{{coalesce template_settings.header_font_family_css template_settings.header_font_family "Arial"}}', sans-serif;
  font-size: 18px;
  font-weight: bold;
  color: var(--primary-color);
  margin-bottom: 6px;
  text-transform: uppercase;
}

.hdr-meta {
  border-collapse: collapse;
  font-size: 11px;
}

.address-banner-box {
  width: 100%;
  margin-top: 10px;
  margin-bottom: 14px;
}

.banner-label {
  font-size: 11px;
  font-weight: bold;
  color: #ffffff;
  background: var(--primary-color);
  padding: 3px 6px;
  display: inline-block;
  width: 420px;
  box-sizing: border-box;
  margin-bottom: 4px;
}

.recipient-name {
  font-size: 12px;
  font-weight: bold;
  color: var(--text-dark);
  margin-bottom: 1px;
}

.recipient-line {
  font-size: 11px;
  color: var(--text-dark);
  line-height: 1.3;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
  margin-bottom: 14px;
}

.data-table th {
  color: #ffffff;
  font-weight: bold;
  padding: 5px 6px;
  background: var(--primary-color);
  font-size: 11px;
}

.data-table td {
  padding: 5px 6px;
  border-bottom: none;
  vertical-align: middle;
  color: var(--text-dark);
}

.data-table .num {
  text-align: right;
}

.data-table .center {
  text-align: center;
}

.bg-sky-row td {
  background-color: rgb(from var(--accent-color) r g b / 0.14) !important;
  padding-top: 1px !important;
  padding-bottom: 1px !important;
  margin-bottom: 2px !important;
}

.txt-blue-val {
  color: var(--text-dark) !important;
  font-weight: normal !important;
}

.amt-pos { color: var(--pos-green) !important; }

.amt-neg { color: var(--neg-red) !important; }

.row-bold td { font-weight: bold; }
.row-total td {
  font-weight: bold;
  border-top: 1px solid var(--primary-color) !important;
  border-bottom: 1px solid var(--primary-color) !important;
  background-color: rgb(from var(--accent-color) r g b / 0.14) !important;
  padding-top: 1px !important;
  padding-bottom: 1px !important;
}

.row-final-balance td {
  font-weight: bold;
  background-color: rgb(from var(--accent-color) r g b / 0.20) !important;
  border-top: 2.5px solid var(--primary-color) !important;
  border-bottom: 2.5px solid var(--primary-color) !important;
}

.bullet-list {
  margin: 4px 0 4px 18px;
  font-size: 11px;
  line-height: 1.4;
}

.footer-notes-box {
  margin-top: 12px;
  font-size: 10px;
  color: #4b5563;
  line-height: 1.4;
}

.payment-details-container {
  margin-top: 16px;
  width: 100%;
}

.payment-details-header {
  background: var(--primary-color);
  color: #ffffff;
  font-weight: bold;
  font-size: 11px;
  padding: 5px 6px;
  text-transform: uppercase;
}

.payment-details-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
}

.payment-details-table td {
  padding: 5px 6px;
  vertical-align: middle;
  border-bottom: none;
}

body .payment-details-table-bordered {
  border: 1px solid var(--primary-color) !important;
  border-collapse: collapse !important;
}

body .payment-details-table-bordered td {
  border: 1px solid var(--primary-color) !important;
}

body .payment-details-table-striped tr:nth-child(even) {
  background-color: rgb(from var(--accent-color) r g b / 0.22) !important;
}
`
}
