package template

import (
	"fmt"

	"github.com/google/uuid"
)

// defaultTemplateHeader takes custom settings into account dynamically via Handlebars tags
func defaultTemplateHeader(title string, labelName string, addressBannerHTML string) string {
	titleHTML := title
	if title == "RECIPIENT CREATED TAX INVOICE" {
		titleHTML = `RCTI<br><span style="font-size: 12px; letter-spacing: 0.2px; font-weight: normal; font-style: italic; text-transform: none; color: gray; display: block; margin-top: 2px;">Recipient Created Tax Invoice</span>`
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
        {{else if address}}
        <p class="hdr-clinic-line">{{address}}</p>
        {{/if}}
        <p class="hdr-clinic-contact">
          {{#if bill_from.abn}}ABN {{bill_from.abn}}{{/if}}{{#if bill_from.phone}} &nbsp;|&nbsp; Ph {{bill_from.phone}}{{/if}}{{#if bill_from.email}} &nbsp;|&nbsp; {{bill_from.email}}{{/if}}
        </p>
      </div>

      ` + addressBannerHTML + `
    </td>
    <td style="width: 45%; vertical-align: top; text-align: right; padding: 0;">
      <h1 class="hdr-doc-title" style="line-height: 1.2;">` + titleHTML + `</h1>
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
@page {
  size: A4 portrait;
  margin: 0;
}

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
  --accent-color: {{#if template_settings.accent_color}}{{template_settings.accent_color}}{{else}}#5f96b4{{/if}};
  --text-dark: #000000;
  --pos-green: #007a3d;
  --neg-red: #c50505;
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
  height: 297mm;
  min-height: 297mm;
  margin: 0 auto; 
  background: #ffffff; 
  padding: 20mm 20mm 20mm 20mm; 
  position: relative; 
  box-sizing: border-box; 
  page-break-after: always;
  break-after: page;
}

.invoice-page:last-child {
  page-break-after: avoid;
  break-after: avoid;
}

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
  display: block;
  width: 100%;
  max-width: 100%; 
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

func DefaultTemplates() []RqGlobalTemplate {
	calculationPreparedFor := `<div class="address-banner-box"><div class="banner-label">PREPARED FOR</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`
	taxInvoiceBillTo := `<div class="address-banner-box"><div class="banner-label">BILL TO</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`
	rctiSupplier := `<div class="address-banner-box"><div class="banner-label">SUPPLIER (DENTIST)</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}<p class="recipient-line" style="margin-top: 4px; font-size: 10px; font-style: italic; color: gray !important">Recipient: {{bill_from.name}} • ABN {{bill_from.abn}}<br>Issued by the recipient under an RCTI agreement between the parties.</p></div>`
	remittancePayee := `<div class="address-banner-box"><div class="banner-label">PAYEE</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`

	return []RqGlobalTemplate{
		{
			Name:      "Calculation Statement",
			IsDefault: true,
			IsActive:  true,
			Html: fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">1. PATIENT FEES</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each patient_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}">{{format_table_amount this}}</td>
        <td class="center">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 50%%; text-align: left;">2. TREATMENT COSTS</th>
        <th style="width: 20%%; text-align: center;">Paid By</th>
        <th style="width: 15%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each treatment_cost_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="center" style="text-transform: uppercase;">{{paid_by}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}">{{format_table_amount this}}</td>
        <td class="center">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">3. NET PATIENT FEES</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each net_patient_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td>{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}">{{format_table_amount this}}</td>
        <td class="center">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">{{#if is_method_c}}4. DENTIST COMMISSION (Independent Contractor){{else}}4. SERVICE &amp; FACILITY FEE{{/if}}</th>
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
                  {{#if is_method_c}}Commission rate{{else}}Fee rate{{/if}}&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
              </td>
              <td class="num" style="width: 20%%; padding: 0; text-align: right; vertical-align: middle;">{{service_fee_rate_intro.amount_display}}</td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
          {{#unless is_method_c}}
            {{#if service_description_items}}
            <ol style="margin: 6px 0 0 18px; padding: 0; list-style-type: decimal; font-size: 11px; line-height: 1.5; color: var(--text-dark);">
              {{#each service_description_items}}
              <li style="margin-bottom: 2px;">{{this}}</li>
              {{/each}}
            </ol>
            {{/if}}
          {{/unless}}
        </td>
      </tr>
      {{#each service_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
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
			Html: fmt.Sprintf(`{{#unless is_method_c}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

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
      
      {{#each invoice_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%; text-align: right;">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <table class="layout-table" style="margin-top: 4px;">
    <tr>
      <td style="width: 50%%; vertical-align: top; padding: 0;">
        {{#if is_method_b}}
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
                <td>{{#if clinic_payment_details}}{{clinic_payment_details}}{{else}}083-000 / 98765432{{/if}}</td>
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
        {{/if}}
      </td>
      <td style="width: 50%%; padding: 0; vertical-align: top;">
        <table class="layout-table" style="font-size: 11px; line-height: 1.6; margin-left: auto; width: 100%%;">
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
        </table>
      </td>
    </tr>
  </table>

 {{#if payment_terms}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">Payment terms: {{payment_terms}}</p>
  </div>
  {{else if template_settings.payment_terms}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">Payment terms: {{template_settings.payment_terms}}</p>
  </div>
  {{else}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic; margin-bottom: 4px;">
      {{#if is_method_b}}
      Patient fees for the period were collected directly by the dentist. This tax invoice is the clinic's service &amp; facility fee (plus any costs paid by the clinic) and is payable by the dentist to the clinic at the account above.
      {{else}}
      Payment terms: This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you.
      {{/if}}
    </p>
  </div>
  {{/if}}
</div>{{/unless}}`, defaultTemplateHeader("TAX INVOICE", "Invoice No.", taxInvoiceBillTo)),
			Css: sharedCSS(),
		},
		{
			Name:      "Recipient Created Tax Invoice",
			IsDefault: false,
			IsActive:  true,
			Html: fmt.Sprintf(`{{#if is_method_c}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table" style="margin-top: 4px;">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">DENTAL SERVICES SUPPLIED - COMMISSION</th>
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
                Professional dental services for the period {{billing_period}}, remunerated at the agreed commission rate on net patient fees.
              </td>
              <td style="width: 20%%; padding: 0;"></td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
        </td>
      </tr>
      
      {{#each rcti_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%; text-align: right;">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div style="width: 100%%; display: block; margin-top: 4px;">
    <table class="layout-table" style="font-size: 11px; line-height: 1.6; margin-left: auto; width: 50%%;">
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
    </table>
  </div>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS - PAY TO DENTIST</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}">
      <tbody>
        <tr>
          <td style="font-weight: bold; width: 45%%;">Account</td>
          <td style="width: 55%%;">{{bill_to.name}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">BSB / Acc No.</td>
          <td>{{#if dentist_payment_details.bsb}}{{dentist_payment_details.bsb}} / {{dentist_payment_details.account}}{{else}}063-000 / 12345678{{/if}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Reference</td>
          <td>{{invoice_number}}</td>
        </tr>
      </tbody>
    </table>
  </div>

  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic; margin-bottom: 4px;">
      This RCTI is created by the clinic (recipient) on behalf of the dentist (supplier). The dentist must not issue a separate tax invoice for this supply. See Remittance Advice for the net amount paid.
    </p>
  </div>
</div>{{/if}}`, defaultTemplateHeader("RECIPIENT CREATED TAX INVOICE", "RCTI No.", rctiSupplier)),
			Css: sharedCSS(),
		},
		{
			Name:      "Remittance Advice",
			IsDefault: false,
			IsActive:  true,
			Html: fmt.Sprintf(`{{#unless is_method_b}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">NET AMOUNT PAYABLE TO YOU</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each remittance_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;">{{#if is_negative}}({{format_currency amount}}){{else}}{{format_currency amount}}{{/if}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}">
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

  <div class="footer-notes-box">
    <p style="text-transform: lowercase; font-style: italic;"><span style="text-transform: none; font-style: italic;">This remittance advice is issued</span> {{invoice_frequency}} <span style="text-transform: none; font-style: italic;">together with the Calculation Statement (page 1) and {{#if is_method_c}}RCTI (page 2){{else}}Tax Invoice (page 2){{/if}}. Please retain for your records and provide to your accountant at year end.</span></p>
  </div>
</div>{{/unless}}`, defaultTemplateHeader("REMITTANCE ADVICE", "Reference", remittancePayee)),
			Css: sharedCSS(),
		},
	}
}

func DefaultSettings(templateId uuid.UUID) Setting {
	termText := "This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you."
	paymentTerms := "This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you."
	waterMarkText := "PAID"
	tableStyle := "simple"

	return Setting{
		InvoiceId:        nil,
		PrimaryColor:     "#1f4e5f",
		AccentColor:      "#5f96b4",
		BodyFontFamily:   "Arial",
		HeaderFontFamily: "Arial",
		IsLogo:           true,
		LogoId:           nil,
		TermText:         &termText,
		PaymentTerms:     &paymentTerms,
		IsWaterMark:      false,
		WaterMarkText:    &waterMarkText,
		TableStyle:       &tableStyle,
	}
}
