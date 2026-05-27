package template

import (
	"github.com/google/uuid"
)

func DefaultTemplates(clinicId uuid.UUID) []RqTemplate {
	return []RqTemplate{
		{
			ClinicId: clinicId,
			Name:     "Default Template",
			Html: `
<div class="invoice{{#if watermark_enabled}} watermark-on{{/if}}"{{#if watermark_enabled}} data-watermark="{{watermark_text}}"{{/if}}>
  {{#if letterhead_html}}<div class="letterhead">{{letterhead_html}}</div>{{/if}}

  <header class="doc-header">
    <div class="brand">
      {{#if show_logo_image}}
        <img class="brand-logo" src="{{logo_url}}" alt="{{clinic_name}}" />
      {{else}}
        {{#if show_logo}}<div class="brand-logo-placeholder">{{logo_initial}}</div>{{/if}}
      {{/if}}
      {{#if show_logo}}<h2 class="brand-name">{{clinic_name}}</h2>{{/if}}
    </div>
    <h1 class="doc-title">{{invoice_name}}</h1>
  </header>

  <section class="info-grid">
    <div class="info-block">
      <h4>Billed by</h4>
      <p class="name">{{bill_from.name}}</p>
      {{#if bill_from.address}}<p>{{bill_from.address}}</p>{{/if}}
      {{#if bill_from.abn}}<p>ABN: {{bill_from.abn}}</p>{{/if}}
      {{#if bill_from.email}}<p>{{bill_from.email}}</p>{{/if}}
      {{#if bill_from.phone}}<p>{{bill_from.phone}}</p>{{/if}}
    </div>
    <div class="info-block">
      <h4>Billed to</h4>
      <p class="name">{{bill_to.name}}</p>
      {{#if bill_to.address}}<p>{{bill_to.address}}</p>{{/if}}
      {{#if bill_to.abn}}<p>ABN: {{bill_to.abn}}</p>{{/if}}
      {{#if bill_to.email}}<p>{{bill_to.email}}</p>{{/if}}
      {{#if bill_to.phone}}<p>{{bill_to.phone}}</p>{{/if}}
    </div>
    <div class="info-block">
      <h4>Invoice details</h4>
      <div class="meta-row"><span class="label">Invoice #</span><span class="value">{{invoice_number}}</span></div>
      <div class="meta-row"><span class="label">Invoice date</span><span class="value">{{issue_date_display}}</span></div>
      <div class="meta-row"><span class="label">Due date</span><span class="value">{{due_date_display}}</span></div>
      <div class="meta-row due"><span class="label">Due amount</span><span class="value">{{format_currency grand_total}}</span></div>
      {{#if reference}}<div class="meta-row"><span class="label">Reference</span><span class="value">{{reference}}</span></div>{{/if}}
    </div>
  </section>

  <table class="items {{table_style_class}}">
    <thead>
      <tr>
        <th>Name</th>
        <th>Description</th>
        <th class="num">Price</th>
        <th class="num">Qty</th>
        <th class="num">Discount</th>
        {{#if show_tax}}<th class="num">Tax %</th><th class="num">Tax amount</th>{{/if}}
        <th class="num">Total</th>
      </tr>
    </thead>
    <tbody>
      {{#each items}}
      <tr>
        <td>{{name}}</td>
        <td>{{description}}</td>
        <td class="num">{{format_currency unit_price}}</td>
        <td class="num">{{qty}}</td>
        <td class="num">{{format_currency discount_amount}}</td>
        {{#if ../show_tax}}
        <td class="num">{{tax_percent}}%</td>
        <td class="num">{{format_currency tax_amount}}</td>
        {{/if}}
        <td class="num">{{format_currency line_total}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="lower">
    <div>
      {{#if terms_text}}
      <h4>Terms and conditions</h4>
      <div class="text-block">{{terms_text}}</div>
      {{/if}}
      {{#if notes}}
      <h4>Notes to customer</h4>
      <div class="text-block">{{notes}}</div>
      {{/if}}
    </div>
    <div class="summary">
      <p class="totals-caption" style="text-align:right;font-size:11px;color:#6b7280;margin:0 0 8px;">{{totals_amounts_caption}}</p>
      <div class="row"><span>{{totals_subtotal_label}}</span><span>{{format_currency subtotal}}</span></div>
      {{#if show_tax}}{{#if totals_tax_label}}
      <div class="row"><span>{{totals_tax_label}}</span><span>{{format_currency tax_total}}</span></div>
      {{/if}}{{/if}}
      <div class="row"><span>{{totals_discount_label}}</span><span>{{format_currency discount_total}}</span></div>
      <div class="total-due-box">
        <span class="label">{{totals_grand_label}}</span>
        <span class="amount">{{format_currency grand_total}}</span>
      </div>
      {{#if amount_in_words}}<p class="amount-words">{{amount_in_words}}</p>{{/if}}
    </div>
  </div>

  <section class="payment-section">
    <div>
      <p><span class="label">Payment method</span> {{payment_method_label}}</p>
      <p><span class="label">Tax method</span> {{tax_method_label}}</p>
    </div>
    <div class="qr-placeholder">QR / UPI<br>(coming soon)</div>
  </section>

  {{#if has_attachments}}
  <section class="attachments">
    <h4>Attachments</h4>
    <ul class="attachment-list">
      {{#each attachments}}
      <li>{{file_name}}</li>
      {{/each}}
    </ul>
  </section>
  {{/if}}

  {{#if footer_html}}<footer class="doc-footer">{{footer_html}}</footer>{{/if}}
</div>
`,
			Css: `:root {
  --invoice-primary: {{primary_color}};
  --invoice-accent: {{accent_color}};
  --invoice-font-body: {{body_font_family}};
  --invoice-font-header: {{header_font_family}};
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: var(--invoice-font-body), system-ui, -apple-system, sans-serif;
  color: #1f2937;
  font-size: 13px;
  line-height: 1.45;
}
.invoice {
  position: relative;
  max-width: 820px;
  margin: 0 auto;
  padding: 28px 32px 36px;
  background: #fff;
}
.invoice.watermark-on::before {
  content: attr(data-watermark);
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 72px;
  font-weight: 700;
  color: var(--invoice-primary);
  opacity: 0.06;
  transform: rotate(-28deg);
  pointer-events: none;
  z-index: 0;
  white-space: nowrap;
}
.invoice > * { position: relative; z-index: 1; }
.letterhead {
  margin-bottom: 12px;
  font-size: 12px;
  color: #6b7280;
  white-space: pre-wrap;
}
.letterhead:empty { display: none; }
.doc-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 24px;
  margin-bottom: 28px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.brand-logo {
  width: 48px;
  height: 48px;
  object-fit: contain;
  flex-shrink: 0;
}
.brand-logo-placeholder {
  width: 48px;
  height: 48px;
  background: var(--invoice-primary);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 20px;
  flex-shrink: 0;
}
.brand-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--invoice-primary);
  letter-spacing: 0.02em;
  margin: 0;
  font-family: var(--invoice-font-body), sans-serif;
}
.doc-title {
  font-family: var(--invoice-font-header), Georgia, "Times New Roman", serif;
  font-size: 42px;
  font-weight: 400;
  color: #d1d5db;
  margin: 0;
  line-height: 1;
  text-align: right;
}
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
  margin-bottom: 28px;
}
.info-block h4 {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #6b7280;
  margin: 0 0 10px;
}
.info-block p { margin: 3px 0; }
.info-block .name { font-weight: 700; color: #111827; }
.meta-row { display: flex; justify-content: space-between; gap: 8px; }
.meta-row .label { color: #6b7280; }
.meta-row .value { font-weight: 500; text-align: right; }
.meta-row.due .value { font-weight: 700; }
table.items {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 24px;
  font-size: 12px;
}
table.items thead th {
  background: #f3f4f6;
  color: #4b5563;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 10px 8px;
  text-align: left;
  border-bottom: 1px solid #e5e7eb;
}
table.items thead th.num { text-align: right; }
table.items tbody td {
  padding: 12px 8px;
  border-bottom: 1px solid #e5e7eb;
  vertical-align: top;
}
table.items tbody td.num { text-align: right; white-space: nowrap; }
table.items .line-no { color: #9ca3af; font-size: 11px; margin-right: 6px; }
table.items.striped tbody tr:nth-child(even) { background: #fafafa; }
table.items.bordered td,
table.items.bordered th { border: 1px solid #e5e7eb; }
.lower {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 28px;
  margin-bottom: 28px;
}
.lower h4 {
  font-size: 12px;
  font-weight: 600;
  margin: 0 0 8px;
  color: #374151;
}
.lower .text-block {
  font-size: 12px;
  color: #4b5563;
  white-space: pre-wrap;
  margin-bottom: 16px;
}
.summary .row {
  display: flex;
  justify-content: space-between;
  padding: 5px 0;
  font-size: 13px;
}
.summary .row span:first-child { color: #6b7280; }
.total-due-box {
  margin-top: 12px;
  background: var(--invoice-primary);
  color: #fff;
  padding: 14px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-radius: 2px;
}
.total-due-box .label { font-size: 13px; font-weight: 500; }
.total-due-box .amount { font-size: 22px; font-weight: 700; }
.amount-words {
  margin-top: 10px;
  font-size: 11px;
  color: #6b7280;
  font-style: italic;
}
.payment-section {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 24px;
  padding-top: 16px;
  border-top: 1px solid #e5e7eb;
  font-size: 12px;
}
.payment-section p { margin: 4px 0; }
.payment-section .label { color: #6b7280; display: inline-block; min-width: 140px; }
.qr-placeholder {
  width: 100px;
  height: 100px;
  border: 1px dashed #d1d5db;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: #9ca3af;
  text-align: center;
  padding: 8px;
}
.doc-footer {
  margin-top: 24px;
  padding-top: 12px;
  font-size: 11px;
  color: #9ca3af;
  text-align: center;
  white-space: pre-wrap;
}
.doc-footer:empty { display: none; }
.attachments { margin-top: 20px; font-size: 12px; }
.attachments h4 { font-size: 12px; font-weight: 600; margin: 0 0 8px; color: #374151; }
.attachment-list { margin: 0; padding-left: 18px; color: #4b5563; }
.attachment-list li { margin: 4px 0; }",
			IsDefault: true,
			IsActive:  true,
		},
	}
}`,
			IsDefault: true,
			IsActive:  true,
		}}
}
