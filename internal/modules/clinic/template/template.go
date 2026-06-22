package template

import (
	"fmt"

	"github.com/google/uuid"
)

func defaultTemplateHeader(title string, labelName string, addressBannerHTML string) string {
	return `<table class="layout-table" style="margin-bottom: 2px; width: 100%; border-collapse: collapse;">
  <tr>
    <td style="width: 55%; vertical-align: top; padding: 0;">
      {{if .logo_url}}
      <div style="line-height: 0; margin: 0 0 4px 0;">
        <img class="brand-logo" src="{{.logo_url}}" alt="{{.bill_from.name}}" />
      </div>
      {{end}}

      <div style="margin: 0; padding: 0;">
        <h2 class="hdr-clinic-name">{{.bill_from.name}}</h2>
        {{if .bill_from.address}}
        <p class="hdr-clinic-line">{{.bill_from.address}}</p>
        {{end}}
        <p class="hdr-clinic-contact">
          {{if .bill_from.abn}}ABN {{.bill_from.abn}}{{end}}{{if .bill_from.phone}} &nbsp;|&nbsp; Ph {{.bill_from.phone}}{{end}}{{if .bill_from.email}} &nbsp;|&nbsp; {{.bill_from.email}}{{end}}
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
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{.invoice_number}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Issue Date</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{.issue_date_display}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Billing Period</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{.billing_period}}</td>
          </tr>
          <tr>
            <td class="hm-lbl" style="text-align: left; padding: 2px 0;"><strong>Invoice Frequency</strong></td>
            <td class="hm-val" style="text-align: right; padding: 2px 0;">{{.invoice_frequency}}</td>
          </tr>
        </tbody>
      </table>
    </td>
  </tr>
</table>`
}

func sharedCSS() string {
	return `
@import url('https://fonts.googleapis.com/css2?family={{.template_settings.header_font_family}}:wght@400;700&display=swap');
@import url('https://fonts.googleapis.com/css2?family={{.template_settings.body_font_family}}:wght@400;700&display=swap');
@import url('https://fonts.googleapis.com/css2?family=Arial:wght@400;700&display=swap');

:root { 
  --primary-color: {{.template_settings.primary_color}};
  --accent-color: {{.template_settings.accent_color}};
  --bg-input-blue: #e8f1f5; 
  --bg-darker-blue: #d4e5ee;
  --text-dark: #000000;
  --pos-green: #007a3d;
  --bright-blue: #0000FF;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body { 
  font-family: '{{.template_settings.body_font_family}}', sans-serif; 
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
  padding: 14mm 16mm; 
  position: relative; 
  box-sizing: border-box; 
  page-break-after: always;
}

.invoice-page:last-child {
  page-break-after: avoid;
}

.invoice-page::before {
  content: "{{.template_settings.watermark_text}}";
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
  font-family: '{{.template_settings.header_font_family}}', sans-serif;
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
  font-family: '{{.template_settings.header_font_family}}', sans-serif;
  font-size: 20px; 
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
  width: 330px; 
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
}

.txt-blue-val {
  color: var(--bright-blue) !important;
  font-weight: bold;
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
`
}

func DefaultTemplates() []RqGlobalTemplate {
	calculationPreparedFor := `<div class="address-banner-box"><div class="banner-label">PREPARED FOR</div><div class="recipient-name">{{.bill_to.name}}</div><p class="recipient-line">{{.bill_to.address}}</p><p class="recipient-line">ABN {{.bill_to.abn}}</p></div>`
	taxInvoiceBillTo := `<div class="address-banner-box"><div class="banner-label">BILL TO</div><div class="recipient-name">{{.bill_to.name}}</div><p class="recipient-line">{{.bill_to.address}}</p><p class="recipient-line">ABN {{.bill_to.abn}}</p></div>`
	remittancePayee := `<div class="address-banner-box"><div class="banner-label">PAYEE</div><div class="recipient-name">{{.bill_to.name}}</div><p class="recipient-line">{{.bill_to.address}}</p><p class="recipient-line">ABN {{.bill_to.abn}}</p></div>`

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
      {{range .patient_fee_items}}
      <tr{{if .row_class}} class="{{.row_class}}"{{end}}>
        <td>{{.label}}</td>
        <td class="num{{if .value_class}} {{.value_class}}{{end}}"{{if .is_bold}} style="font-weight: bold;"{{end}}>{{.value}}</td>
        <td class="center">{{.bas_code}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">2. SERVICE & FACILITY FEE (see Tax Invoice &mdash; page 2)</th>
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
                Services rendered to you for the period, including:
              </td>
              <td style="padding: 0; font-weight: bold; width: 20%%; text-align: right; vertical-align: middle; white-space: nowrap;">
                Fee rate &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
                <span class="txt-blue-val" style="display: inline-block; min-width: 50px; text-align: right;">{{.custom_fee_rate}}%%</span>
              </td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
          <ul class="bullet-list" style="list-style-type: decimal; margin-top: 6px;">
            <li>Rent of dental surgery/room</li>
            <li>Patient booking & reception</li>
            <li>Fee collection & banking</li>
            <li>Equipment & instrument hire</li>
            <li>General administration & support staff</li>
          </ul>
        </td>
      </tr>
      {{range .service_fee_items}}
      <tr{{if .row_class}} class="{{.row_class}}"{{end}}>
        <td style="width: 65%%;">{{.label}}</td>
        <td class="num{{if .value_class}} {{.value_class}}{{end}}" style="width: 20%%;{{if .is_bold}} font-weight: bold;{{end}}">{{.value}}</td>
        <td class="center" style="width: 15%%;">{{.bas_code}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 85%%; text-align: left;">3. NET SETTLEMENT (see Remittance Advice &mdash; page 3)</th>
        <th style="width: 15%%; text-align: right;">Amount</th>
      </tr>
    </thead>
    <tbody>
      {{range .settlement_items}}
      <tr{{if .row_class}} class="{{.row_class}}"{{end}}>
        <td>{{.label}}</td>
        <td class="num{{if .value_class}} {{.value_class}}{{end}}"{{if .is_bold}} style="font-weight: bold;"{{end}}>{{if .is_negative}}({{.value}}){{else}}{{.value}}{{end}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <div class="footer-notes-box">
    <p style="font-style: italic; margin-bottom: 4px;">Notes: Total patient fees, GST collected (1A) and laboratory fees are sourced from the practice management system for the billing period. Highlighted rows indicate data input variables; all other figures are calculated. BAS codes are shown for the clinic's activity statement.</p>
    {{if .notes}}
    <p style="margin-top: 4px; font-weight: normal; color: var(--text-dark);"><strong>Notes:</strong> {{.notes}}</p>
    {{end}}
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
        <th style="width: 70%%; text-align: left;">SERVICE & FACILITY FEE</th>
        <th style="width: 15%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: right; padding-right: 8px;">GST</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td style="padding: 6px; line-height: 1.4;">
          <p style="margin-bottom: 2px;">Service and facility fee for the period {{.billing_period}},<br>calculated at the agreed rate on net patient fees, comprising:</p>
          <ul class="bullet-list" style="list-style-type: decimal;">
            <li>Rent of dental surgery/room</li>
            <li>Patient booking & reception</li>
            <li>Fee collection & banking</li>
            <li>Equipment & instrument hire</li>
            <li>General administration & support staff</li>
          </ul>
          <p style="color: var(--text-dark); margin-top: 6px; font-weight: normal;">Service & Facility Fee (per Calculation Statement)</p>
        </td>
        <td class="num amt-pos" style="vertical-align: bottom; font-weight: bold; width: 15%%;">{{.subtotal}}</td>
        <td class="num" style="vertical-align: bottom; font-weight: bold; width: 15%%; color: var(--text-dark); padding-right: 8px;">{{.tax_total}}</td>
      </tr>
    </tbody>
  </table>

  <table class="layout-table" style="margin-top: 4px;">
    <tr>
      <td style="width: 50%%;"></td>
      <td style="width: 50%%; padding: 0;">
        <table class="layout-table" style="font-size: 11px; line-height: 1.6;">
          <tr>
            <td style="padding: 3px 6px; text-align: left;">Subtotal (excl. GST)</td>
            <td class="num" style="padding: 3px 6px;">{{.subtotal}}</td>
          </tr>
          <tr>
            <td style="padding: 3px 6px; text-align: left;">GST (10%%)</td>
            <td class="num" style="padding: 3px 6px;">{{.tax_total}}</td>
          </tr>
          <tr style="font-weight: bold; background-color: var(--bg-input-blue);">
            <td style="padding: 5px 6px; border-top: 1px solid #000000; border-bottom: 2px solid #000000; text-align: left;">TOTAL (incl. GST)</td>
            <td class="num" style="padding: 5px 6px; border-top: 1px solid #000000; border-bottom: 2px solid #000000;">{{.grand_total}}</td>
          </tr>
        </table>
      </td>
    </tr>
  </table>

  {{if .terms_text}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p><strong>Payment terms:</strong> {{.terms_text}}</p>
  </div>
  {{else}}
    {{if .template_settings.terms_text}}
    <div class="footer-notes-box" style="margin-top: 24px;">
      <p><strong>Payment terms:</strong> {{.template_settings.terms_text}}</p>
    </div>
    {{end}}
  {{end}}
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
      <tr>
        <td>Total patient fees collected on your behalf (incl. GST)</td>
        <td class="num amt-pos">{{.custom_patient_fees_collected}}</td>
      </tr>
      <tr>
        <td>Less: laboratory fees (net of GST)</td>
        <td class="num">({{.custom_lab_fees}})</td>
      </tr>
      <tr>
        <td>Less: Service & Facility Fee incl. GST (Tax Invoice)</td>
        <td class="num">({{.grand_total}})</td>
      </tr>
      {{if .discount_total}}
      <tr>
        <td>Less: retainers / drawings previously paid this period</td>
        <td class="num">({{.discount_total}})</td>
      </tr>
      {{end}}
      <tr class="row-final-balance">
        <td>NET PAYABLE TO DENTIST</td>
        <td class="num amt-pos" style="font-size: 11.5px;">{{.custom_balance_remitted}}</td>
      </tr>
    </tbody>
  </table>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS</div>
    <table class="payment-details-table">
      <tbody>
        <tr>
          <td style="font-weight: bold; width: 45%%;">Payment method</td>
          <td style="width: 55%%;">Electronic funds transfer (EFT)</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Account name</td>
          <td>{{.bill_to.name}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">BSB / Account No.</td>
        <td>{{if .custom_payment_bsb}}{{.custom_payment_bsb}}{{else}}063-000{{end}} / {{if .custom_payment_account}}{{.custom_payment_account}}{{else}}12345678{{end}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment date</td>
          <td>{{.issue_date_display}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment reference</td>
          <td>{{.invoice_number}}</td>
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
		TableStyle:       nil,
	}
}

// DefaultMetadataForCalculationStatement returns field schema for Calculation Statement template
func DefaultMetadataForCalculationStatement() TemplateMetadata {
	return TemplateMetadata{
		FieldSchema: []FieldDefinition{
			{
				Key:      "custom_patient_fees_collected",
				Label:    "Patient Fees Collected (incl. GST)",
				Type:     "currency",
				Category: "calculation",
				Required: true,
				HelpText: stringPtr("Total patient fees from BAS code G1"),
			},
			{
				Key:      "custom_patient_fees_gst",
				Label:    "GST Collected on Patient Fees",
				Type:     "currency",
				Category: "calculation",
				Required: true,
				HelpText: stringPtr("GST from taxable services (BAS code 1A)"),
			},
			{
				Key:      "custom_lab_fees",
				Label:    "Laboratory Fees (net of GST)",
				Type:     "currency",
				Category: "calculation",
				Required: false,
				DefaultValue: stringPtr("0"),
			},
			{
				Key:          "custom_fee_rate",
				Label:        "Service & Facility Fee Rate (%)",
				Type:         "number",
				Category:     "calculation",
				Required:     true,
				DefaultValue: stringPtr("30"),
				HelpText:     stringPtr("Percentage rate for service and facility fees"),
			},
			{
				Key:      "discount_total",
				Label:    "Retainers/Drawings Previously Paid",
				Type:     "currency",
				Category: "calculation",
				Required: false,
				DefaultValue: stringPtr("0"),
			},
		},
		ComputedFields: []ComputedField{
			{
				Key:     "custom_patient_fees_gst_free",
				Label:   "GST-Free Sales",
				Formula: "custom_patient_fees_collected - (custom_patient_fees_gst * 11)",
			},
			{
				Key:     "custom_net_patient_fees",
				Label:   "Net Patient Fees",
				Formula: "custom_patient_fees_collected - custom_patient_fees_gst - custom_lab_fees",
			},
			{
				Key:     "subtotal",
				Label:   "Service & Facility Fee (excl. GST)",
				Formula: "custom_net_patient_fees * (custom_fee_rate / 100)",
			},
			{
				Key:     "tax_total",
				Label:   "GST on Service Fee (10%)",
				Formula: "subtotal * 0.10",
			},
			{
				Key:     "grand_total",
				Label:   "Total Service & Facility Fee (incl. GST)",
				Formula: "subtotal + tax_total",
			},
			{
				Key:     "custom_amount_due_to_dentist",
				Label:   "Amount Due to Dentist",
				Formula: "custom_net_patient_fees - subtotal - tax_total",
			},
			{
				Key:     "custom_balance_remitted",
				Label:   "Balance Remitted to Dentist",
				Formula: "custom_amount_due_to_dentist - discount_total",
			},
		},
	}
}

// DefaultMetadataForTaxInvoice returns field schema for Tax Invoice template
func DefaultMetadataForTaxInvoice() TemplateMetadata {
	return TemplateMetadata{
		FieldSchema: []FieldDefinition{
			{
				Key:      "subtotal",
				Label:    "Service & Facility Fee (excl. GST)",
				Type:     "currency",
				Category: "totals",
				Required: true,
				HelpText: stringPtr("From Calculation Statement"),
			},
			{
				Key:      "tax_total",
				Label:    "GST (10%)",
				Type:     "currency",
				Category: "totals",
				Required: true,
			},
		},
		ComputedFields: []ComputedField{
			{
				Key:     "grand_total",
				Label:   "Total (incl. GST)",
				Formula: "subtotal + tax_total",
			},
		},
	}
}

// DefaultMetadataForRemittanceAdvice returns field schema for Remittance Advice template
func DefaultMetadataForRemittanceAdvice() TemplateMetadata {
	return TemplateMetadata{
		FieldSchema: []FieldDefinition{
			{
				Key:      "custom_patient_fees_collected",
				Label:    "Total Patient Fees Collected (incl. GST)",
				Type:     "currency",
				Category: "calculation",
				Required: true,
			},
			{
				Key:      "custom_lab_fees",
				Label:    "Laboratory Fees (net of GST)",
				Type:     "currency",
				Category: "calculation",
				Required: false,
				DefaultValue: stringPtr("0"),
			},
			{
				Key:      "grand_total",
				Label:    "Service & Facility Fee (incl. GST)",
				Type:     "currency",
				Category: "calculation",
				Required: true,
				HelpText: stringPtr("From Tax Invoice"),
			},
			{
				Key:      "discount_total",
				Label:    "Retainers/Drawings Previously Paid",
				Type:     "currency",
				Category: "calculation",
				Required: false,
				DefaultValue: stringPtr("0"),
			},
			{
				Key:      "custom_payment_bsb",
				Label:    "BSB Number",
				Type:     "text",
				Category: "payment",
				Required: false,
				DefaultValue: stringPtr("063-000"),
			},
			{
				Key:      "custom_payment_account",
				Label:    "Account Number",
				Type:     "text",
				Category: "payment",
				Required: false,
				DefaultValue: stringPtr("12345678"),
			},
		},
		ComputedFields: []ComputedField{
			{
				Key:     "custom_balance_remitted",
				Label:   "Net Payable to Dentist",
				Formula: "custom_patient_fees_collected - custom_lab_fees - grand_total - discount_total",
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
