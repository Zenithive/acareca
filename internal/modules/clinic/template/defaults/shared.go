// Package defaults holds the raw HTML/CSS bodies for the platform's
// built-in invoice document templates. This package is intentionally
// dependency-free (no reference to template.RqGlobalTemplate) so the
// parent template package can import it without a cycle.
package defaults

// AddressBannerConfig defines configuration for address banner sections
type AddressBannerConfig struct {
	Label       string
	NameField   string
	AddressField string
	ABNField    string
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

// DefaultBilledToBanner returns the standard "BILLED TO" address banner
func DefaultBilledToBanner() string {
	return AddressBanner(AddressBannerConfig{
		Label:        "BILLED TO",
		NameField:    "bill_to.name",
		AddressField: "bill_to.address",
		ABNField:     "bill_to.abn",
	})
}

// AddressBanner generates an address banner HTML block with the given configuration
func AddressBanner(cfg AddressBannerConfig) string {
	return `<div class="address-banner-box"><div class="banner-label">` + cfg.Label + `</div><div class="recipient-name">{{` + cfg.NameField + `}}</div>{{#if ` + cfg.AddressField + `}}<p class="recipient-line">{{` + cfg.AddressField + `}}</p>{{/if}}{{#if ` + cfg.ABNField + `}}<p class="recipient-line">ABN {{` + cfg.ABNField + `}}</p>{{/if}}</div>`
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
