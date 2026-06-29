package template

import (
	"fmt"

	"github.com/google/uuid"
)

// defaultTemplateHeader takes custom settings into account dynamically via Handlebars tags
func defaultTemplateHeader(title string, labelName string, addressBannerHTML string) string {
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

// sharedCSS maps template variables directly from the dynamic configuration pipeline to control visual attributes
func sharedCSS() string {
	return `
/* Dynamically imports Google Fonts selected inside the dropdown panel */
{{#if template_settings.header_font_family}}
@import url('https://fonts.googleapis.com/css2?family={{template_settings.header_font_family}}:wght@400;700&display=swap');
{{/if}}
{{#if template_settings.body_font_family}}
@import url('https://fonts.googleapis.com/css2?family={{template_settings.body_font_family}}:wght@400;700&display=swap');
{{/if}}
@import url('https://fonts.googleapis.com/css2?family=Arial:wght@400;700&display=swap');

:root { 
  --primary-color: {{#if template_settings.primary_color}}{{template_settings.primary_color}}{{else}}#1f4e5f{{/if}}; 
  --accent-color: {{#if template_settings.accent_color}}{{template_settings.accent_color}}{{else}}#1f4e5f{{/if}};
  --bg-input-blue: #e8f1f5; 
  --bg-darker-blue: #d4e5ee;
  --text-dark: #000000;
  --pos-green: #007a3d;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body { 
  font-family: {{#if template_settings.body_font_family_css}}'{{template_settings.body_font_family_css}}'{{else}}{{#if template_settings.body_font_family}}'{{template_settings.body_font_family}}'{{else}}'Arial'{{/if}}{{/if}}, sans-serif; 
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

/* Background watermark styling driven cleanly by frontend toggle context */
{{#if template_settings.is_watermark}}
.invoice-page::before {
  content: "{{#if template_settings.watermark_text}}{{template_settings.watermark_text}}{{else}}PAID{{/if}}";
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
  font-family: {{#if template_settings.header_font_family_css}}'{{template_settings.header_font_family_css}}'{{else}}{{#if template_settings.header_font_family}}'{{template_settings.header_font_family}}'{{else}}'Arial'{{/if}}{{/if}}, sans-serif;
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
  font-family: {{#if template_settings.header_font_family_css}}'{{template_settings.header_font_family_css}}'{{else}}{{#if template_settings.header_font_family}}'{{template_settings.header_font_family}}'{{else}}'Arial'{{/if}}{{/if}}, sans-serif;
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
  background-color: var(--bg-input-blue) !important;
  padding-top: 1px !important;
  padding-bottom: 1px !important;
  margin-bottom: 2px !important;
}

.txt-blue-val {
  color: var(--text-dark) !important;
  font-weight: normal !important;
}

.amt-pos { color: var(--pos-green) !important; }

.row-bold td { font-weight: bold; }
.row-total td { font-weight: bold; border-top: 1px solid #000000; border-bottom: 1px solid #000000; }

.row-final-balance td {
  font-weight: bold;
  background-color: var(--bg-darker-blue) !important;
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
  border: 1px solid #9ca3af !important; 
  border-collapse: collapse !important;
}

body .payment-details-table-bordered td {
  border: 1px solid #9ca3af !important; 
}

body .payment-details-table-striped tr:nth-child(even) {
  background-color: #9ca3af !important; 
}
`
}

func DefaultTemplates() []RqGlobalTemplate {
	calculationPreparedFor := `<div class="address-banner-box"><div class="banner-label">PREPARED FOR</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`
	taxInvoiceBillTo := `<div class="address-banner-box"><div class="banner-label">BILL TO</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`
	remittancePayee := `<div class="address-banner-box"><div class="banner-label">PAYEE</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`

	return []RqGlobalTemplate{
		{
			Name:      "Calculation Statement",
			IsDefault: true,
			IsActive:  true,
			Html: fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">1. PATIENT FEES COLLECTED ON YOUR BEHALF</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each patient_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}">{{format_currency amount}}</td>
        <td class="center">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">2. SERVICE &amp; FACILITY FEE</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td colspan="3" style="border-bottom: none; padding-top: 5px; padding-bottom: 4px;">
          <table class="layout-table" style="width: 100%%; border-collapse: collapse;">
            <tr>
              <td style="padding: 0; color: black; width: 65%%; vertical-align: middle;">
                {{service_fee_rate_intro.label}}
                <span style="float: right; font-weight: bold; white-space: nowrap; margin-left: 8px;">
                  Fee rate&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
              </td>
              <td class="num" style="width: 20%%; padding: 0; text-align: right; vertical-align: middle;">{{service_fee_rate_intro.amount_display}}</td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
          {{#if service_description_items}}
          <ol style="margin: 6px 0 0 18px; padding: 0; list-style-type: decimal; font-size: 11px; line-height: 1.5; color: var(--text-dark);">
            {{#each service_description_items}}
            <li style="margin-bottom: 2px;">{{this}}</li>
            {{/each}}
          </ol>
          {{/if}}
        </td>
      </tr>
      {{#each service_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;{{#if is_bold}} font-weight: bold;{{/if}}">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 85%%; text-align: left;">3. NET SETTLEMENT</th>
        <th style="width: 15%%; text-align: right;">Amount</th>
      </tr>
    </thead>
    <tbody>
      {{#each settlement_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}"{{#if is_bold}} style="font-weight: bold;"{{/if}}">{{#if is_negative}}({{format_currency amount}}){{else}}{{format_currency amount}}{{/if}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="footer-notes-box">
    {{#if notes}}
    <p style="margin-top: 4px; font-weight: normal; color: var(--text-dark);"><strong>Notes:</strong> {{notes}}</p>
    {{else}}
      {{#if template_settings.terms_text}}
      <p style="margin-top: 4px; font-weight: normal; color: var(--text-dark);"><strong>Notes:</strong> {{template_settings.terms_text}}</p>
      {{else}}
      <p style="font-style: italic; margin-bottom: 4px;">Notes: Total patient fees, GST collected (1A) and laboratory fees are sourced from the practice management system for the billing period. Highlighted rows indicate data input variables; all other figures are calculated. BAS codes are shown for the clinic's activity statement.</p>
      {{/if}}
    {{/if}}
  </div>
</div>`, defaultTemplateHeader("CALCULATION STATEMENT", "Statement No.", calculationPreparedFor)),
			Css: sharedCSS(),
		},
		{
			Name:      "Tax Invoice",
			IsDefault: false,
			IsActive:  true,
			Html: fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table" style="margin-top: 4px;">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">SERVICE &amp; FACILITY FEE</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td colspan="3" style="border-bottom: none; padding-top: 5px; padding-bottom: 4px;">
          <table class="layout-table" style="width: 100%%; border-collapse: collapse;">
            <tr>
              <td style="padding: 0; color: var(--text-dark); width: 65%%; vertical-align: middle;">
                {{service_fee_rate_intro.label}}
                <span style="float: right; font-weight: bold; white-space: nowrap; margin-left: 8px;">
                  Fee rate&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
              </td>
              <td style="width: 20%%; padding: 0;"></td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
          {{#if service_description_items}}
          <ol style="margin: 6px 0 0 18px; padding: 0; list-style-type: decimal; font-size: 11px; line-height: 1.5; color: var(--text-dark);">
            {{#each service_description_items}}
            <li style="margin-bottom: 2px;">{{this}}</li>
            {{/each}}
          </ol>
          {{/if}}
        </td>
      </tr>
      
      {{#each service_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%; text-align: right;{{#if is_bold}} font-weight: bold;{{/if}}">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="layout-table" style="margin-top: 4px;">
    <tr>
      <td style="width: 50%%;"></td>
      <td style="width: 50%%; padding: 0;">
        <table class="layout-table" style="font-size: 11px; line-height: 1.6;">
          <tr>
            <td style="padding: 3px 6px; text-align: left;">Subtotal (excl. GST)</td>
            <td class="num" style="padding: 3px 6px;">{{format_currency subtotal}}</td>
          </tr>
          <tr>
            <td style="padding: 3px 6px; text-align: left;">GST (10%%)</td>
            <td class="num" style="padding: 3px 6px;">{{format_currency tax_total}}</td>
          </tr>
          <tr style="font-weight: bold; background-color: var(--bg-input-blue);">
            <td style="padding: 5px 6px; border-top: 1px solid #000000; border-bottom: 2px solid #000000; text-align: left;">TOTAL (incl. GST)</td>
            <td class="num" style="padding: 5px 6px; border-top: 1px solid #000000; border-bottom: 2px solid #000000;">{{format_currency grand_total}}</td>
          </tr>
        </table>
      </td>
    </tr>
  </table>

  {{#if terms_text}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p><strong>Payment terms:</strong> {{terms_text}}</p>
  </div>
  {{else}}
    {{#if template_settings.terms_text}}
    <div class="footer-notes-box" style="margin-top: 24px;">
      <p><strong>Payment terms:</strong> {{template_settings.terms_text}}</p>
    </div>
    {{/if}}
  {{/if}}
</div>`, defaultTemplateHeader("TAX INVOICE", "Invoice No.", taxInvoiceBillTo)),
			Css: sharedCSS(),
		},
		{
			Name:      "Remittance Advice",
			IsDefault: false,
			IsActive:  true,
			Html: fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 80%%; text-align: left;">NET AMOUNT PAYABLE TO YOU</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
      </tr>
    </thead>
    <tbody>
      {{#each remittance_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}">{{#if is_negative}}({{format_currency amount}}){{else}}{{format_currency amount}}{{/if}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS</div>
    <table class="payment-details-table{{#if (eq template_settings.table_style 'bordered')}} payment-details-table-bordered{{else}}{{#if (eq template_settings.table_style 'striped')}} payment-details-table-striped{{/if}}{{/if}}">
      <tbody>
        <tr>
          <td style="font-weight: bold; width: 45%%;">Payment method</td>
          <td style="width: 55%%;">{{#if custom_payment_method}}{{custom_payment_method}}{{else}}{{#if payment_method_label}}{{payment_method_label}}{{else}}Electronic funds transfer (EFT){{/if}}{{/if}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Account name</td>
          <td>{{#if custom_payment_account_name}}{{custom_payment_account_name}}{{else}}{{bill_to.name}}{{/if}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">BSB / Account No.</td>
          <td>{{#if custom_payment_bsb}}{{custom_payment_bsb}}{{else}}063-000{{/if}} / {{#if custom_payment_account}}{{custom_payment_account}}{{else}}12345678{{/if}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment date</td>
          <td>{{#if payment_date_display}}{{payment_date_display}}{{else}}{{issue_date_display}}{{/if}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment reference</td>
          <td>{{invoice_number}}</td>
        </tr>
      </tbody>
    </table>
  </div>

  <p style="margin-top: 30px; font-size: 11px; color: #4b5563; text-align: center; line-height: 1.5;">
    This remittance advice is issued monthly together with the Calculation Statement (page 1) and Tax Invoice (page 2).<br>Please retain for your records and provide to your accountant at year end.
  </p>
</div>`, defaultTemplateHeader("REMITTANCE ADVICE", "Reference", remittancePayee)),
			Css: sharedCSS(),
		},
	}
}

func DefaultSettings(templateId uuid.UUID) Setting {
	termText := "This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you."
	waterMarkText := "PAID"
	tableStyle := "simple"

	return Setting{
		TemplateId:       templateId,
		MappingId:        nil,
		PrimaryColor:     "#1f4e5f",
		AccentColor:      "#1f4e5f",
		BodyFontFamily:   "Arial",
		HeaderFontFamily: "Arial",
		IsLogo:           true,
		LogoId:           nil,
		LetterHeadId:     nil,
		FooterId:         nil,
		TermText:         &termText,
		IsWaterMark:      false,
		WaterMarkText:    &waterMarkText,
		IsTax:            true,
		TableStyle:       &tableStyle,
	}
}
