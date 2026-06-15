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
  {{#if letterhead_url}}
    <div class="letterhead-banner"><img src="{{letterhead_url}}" alt="Letterhead" class="letterhead-banner-img" /></div>
  {{else}}
    {{#if letterhead_html}}<div class="letterhead-placeholder"><div class="letterhead">{{letterhead_html}}</div></div>{{else}}<div class="letterhead-placeholder letterhead-placeholder-empty"></div>{{/if}}
  {{/if}}

  <div class="invoice-body">

  <!-- ══════════════════════════════════════════
       HEADER — two-column grid
       Left : clinic name + address + contact
       Right: "CALCULATION STATEMENT" + meta rows
       Below left: PREPARED FOR banner + recipient
       Below right: meta rows continue alongside
  ══════════════════════════════════════════ -->
  <div class="doc-header">

    <!-- top row: clinic block (left) | title (right) -->
    <div class="header-clinic">
      <h2 class="brand-name">{{bill_from.name}}</h2>
      {{#if bill_from.address}}<p class="header-line">{{bill_from.address}}</p>{{/if}}
      <p class="header-contact">{{#if bill_from.abn}}ABN {{bill_from.abn}}{{/if}}{{#if bill_from.phone}}&nbsp;&nbsp;|&nbsp;&nbsp;Ph {{bill_from.phone}}{{/if}}{{#if bill_from.email}}&nbsp;&nbsp;|&nbsp;&nbsp;{{bill_from.email}}{{/if}}</p>
    </div>

    <div class="header-right">
      <p class="doc-title">CALCULATION STATEMENT</p>
      <table class="header-meta">
        <tr><td class="hm-label">Statement No.</td><td class="hm-value">{{invoice_number}}</td></tr>
        <tr><td class="hm-label">Issue Date</td><td class="hm-value">{{issue_date_display}}</td></tr>
        <tr><td class="hm-label">Billing Period</td><td class="hm-value">{{due_date_display}}</td></tr>
        <tr><td class="hm-label">Invoice Frequency</td><td class="hm-value">{{payment_method_label}}</td></tr>
        {{#if reference}}<tr><td class="hm-label">Reference</td><td class="hm-value">{{reference}}</td></tr>{{/if}}
      </table>
    </div>

    <!-- prepared-for banner spans left column only -->
    <div class="prepared-col">
      <div class="prepared-banner">PREPARED FOR</div>
      <div class="prepared-body">
        <p class="prepared-name">{{bill_to.name}}</p>
        {{#if bill_to.address}}<p class="prepared-line">{{bill_to.address}}</p>{{/if}}
        {{#if bill_to.abn}}<p class="prepared-line">ABN {{bill_to.abn}}</p>{{/if}}
        {{#if bill_to.email}}<p class="prepared-line">{{bill_to.email}}</p>{{/if}}
        {{#if bill_to.phone}}<p class="prepared-line">{{bill_to.phone}}</p>{{/if}}
      </div>
    </div>

    <!-- right column spacer — keeps grid aligned -->
    <div class="prepared-right-spacer"></div>

  </div>

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

  </div><!-- /invoice-body -->

  {{#if footer_html}}
  <footer class="doc-footer-banner"><img src="{{footer_html}}" alt="Footer" class="doc-footer-banner-img" /></footer>
  {{else}}
  <footer class="doc-footer-placeholder"></footer>
  {{/if}}
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
  padding: 0;
  background: #fff;
  display: flex;
  flex-direction: column;
  min-height: 100%;
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
.letterhead-banner {
  width: 100%;
  margin: 0;
  padding: 0;
  line-height: 0;
}
.letterhead-banner-img {
  width: 100%;
  height: 120px;
  max-height: 130px;
  object-fit: cover;
  display: block;
}
.letterhead-placeholder {
  padding: 16px 32px 0;
  min-height: 40px;
}
.letterhead-placeholder-empty {
  min-height: 28px;
  padding: 0;
}
.letterhead {
  font-size: 12px;
  color: #6b7280;
  white-space: pre-wrap;
}
/* ══════════════════════════════════════
   HEADER  —  CSS Grid, 2 columns
   col 1 (left ~55%): clinic info + prepared-for
   col 2 (right ~45%): doc title + meta table
══════════════════════════════════════ */
.doc-header {
  display: grid;
  grid-template-columns: 55fr 45fr;
  gap: 0 32px;
  padding: 28px 32px 24px;
  border-bottom: 2px solid #e5e7eb;
  margin-bottom: 0;
}

/* ── LEFT col top: clinic name / address / contact ── */
.header-clinic {
  grid-column: 1;
  grid-row: 1;
}
.brand-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--invoice-primary);
  margin: 0 0 4px;
  white-space: nowrap;
  font-family: var(--invoice-font-body), sans-serif;
}
.header-line {
  font-size: 12px;
  color: #374151;
  margin: 2px 0;
  white-space: nowrap;
}
.header-contact {
  font-size: 12px;
  color: #374151;
  margin: 4px 0 0;
  white-space: nowrap;
}

/* ── RIGHT col: doc title + meta ── */
.header-right {
  grid-column: 2;
  grid-row: 1 / 3;
  text-align: right;
}
.doc-title {
  font-family: var(--invoice-font-header), Georgia, "Times New Roman", serif;
  font-size: 22px;
  font-weight: 800;
  color: var(--invoice-primary);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  margin: 0 0 10px;
  line-height: 1.1;
  white-space: nowrap;
}
.header-meta {
  border-collapse: collapse;
  margin-left: auto;
  font-size: 12px;
}
.header-meta td { padding: 2px 0; vertical-align: top; }
.hm-label {
  font-weight: 700;
  color: #374151;
  text-align: right;
  padding-right: 14px;
  white-space: nowrap;
}
.hm-value {
  color: #1a2332;
  text-align: right;
  white-space: nowrap;
}

/* ── LEFT col bottom: PREPARED FOR ── */
.prepared-col {
  grid-column: 1;
  grid-row: 2;
  margin-top: 18px;
}
.prepared-banner {
  background: var(--invoice-primary);
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  padding: 7px 14px;
  display: block;
}
.prepared-body {
  padding: 10px 0 0;
}
.prepared-name {
  font-size: 14px;
  font-weight: 700;
  color: #111827;
  margin: 0 0 4px;
}
.prepared-line {
  font-size: 12px;
  color: #374151;
  margin: 2px 0;
}

/* spacer keeps grid rows balanced */
.prepared-right-spacer {
  grid-column: 2;
  grid-row: 2;
}
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
.doc-footer-banner {
  width: 100%;
  margin: 0;
  padding: 0;
  line-height: 0;
}
.doc-footer-banner-img {
  width: 100%;
  height: 100px;
  max-height: 120px;
  object-fit: cover;
  display: block;
}
.doc-footer-placeholder {
  min-height: 28px;
  display: block;
}
.invoice-body {
  flex-grow: 1;
  padding: 0 32px 28px;
}
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
		},
		{
			ClinicId: clinicId,
			Name:     "MediCare Invoice",
			Html: `<div class="inv-root">
  {{#if letterhead_url}}
    <div class="inv-letterhead-banner"><img src="{{letterhead_url}}" alt="Letterhead" class="inv-letterhead-banner-img" /></div>
  {{else}}
    <div class="inv-top-stripe"></div>
    {{#if letterhead_html}}<div class="inv-letterhead-text-wrap"><div class="inv-letterhead">{{letterhead_html}}</div></div>{{else}}<div class="inv-letterhead-placeholder"></div>{{/if}}
  {{/if}}

  <div class="inv-header">
    <div class="inv-clinic-block">
      {{#if show_logo_image}}
        <div class="inv-logo-circle"><img class="brand-logo" src="{{logo_url}}" alt="{{clinic_name}}" /></div>
      {{else}}
        {{#if show_logo}}<div class="inv-logo-circle"><div class="inv-logo-cross"></div></div>{{/if}}
      {{/if}}
      {{#if show_logo}}<div>
        <p class="inv-clinic-name">{{clinic_name}}</p>
        <p class="inv-clinic-tagline">Medical &amp; Healthcare Services</p>
      </div>{{/if}}
    </div>
    <div class="inv-doc-badge">
      <p class="inv-doc-number">No. {{invoice_number}}</p>
    </div>
  </div>

  <div class="inv-status-ribbon-wrap">
    <div class="inv-status-ribbon">
      <span class="inv-status-left">Patient / Client Billing Summary</span>
      <div class="inv-status-right">
        <span class="inv-status-pill">{{payment_method_label}}</span>
      </div>
    </div>
  </div>

  <div class="inv-body inv-watermark-wrap {{#if watermark_enabled}}watermark-on{{/if}}" {{#if watermark_enabled}}data-watermark="{{watermark_text}}"{{/if}}>

    <div class="inv-info-grid">
      <div class="inv-info-card">
        <p class="inv-info-card-title">Billed by</p>
        <p class="inv-info-name">{{bill_from.name}}</p>
        {{#if bill_from.address}}<p class="inv-info-line">{{bill_from.address}}</p>{{/if}}
        {{#if bill_from.abn}}<p class="inv-info-line">ABN: {{bill_from.abn}}</p>{{/if}}
        {{#if bill_from.email}}<p class="inv-info-line">{{bill_from.email}}</p>{{/if}}
        {{#if bill_from.phone}}<p class="inv-info-line">{{bill_from.phone}}</p>{{/if}}
      </div>

      <div class="inv-info-card">
        <p class="inv-info-card-title">Billed to</p>
        <p class="inv-info-name">{{bill_to.name}}</p>
        {{#if bill_to.address}}<p class="inv-info-line">{{bill_to.address}}</p>{{/if}}
        {{#if bill_to.abn}}<p class="inv-info-line">ABN: {{bill_to.abn}}</p>{{/if}}
        {{#if bill_to.email}}<p class="inv-info-line">{{bill_to.email}}</p>{{/if}}
        {{#if bill_to.phone}}<p class="inv-info-line">{{bill_to.phone}}</p>{{/if}}
      </div>

      <div class="inv-info-card">
        <p class="inv-info-card-title">Invoice details</p>
        <div class="inv-detail-row">
          <span class="inv-detail-label">Invoice #</span>
          <span class="inv-detail-value">{{invoice_number}}</span>
        </div>
        <div class="inv-detail-row">
          <span class="inv-detail-label">Invoice date</span>
          <span class="inv-detail-value">{{issue_date_display}}</span>
        </div>
        <div class="inv-detail-row">
          <span class="inv-detail-label">Due date</span>
          <span class="inv-detail-value">{{due_date_display}}</span>
        </div>
        {{#if reference}}
        <div class="inv-detail-row">
          <span class="inv-detail-label">Reference</span>
          <span class="inv-detail-value">{{reference}}</span>
        </div>
        {{/if}}
        <div class="inv-detail-row">
          <span class="inv-detail-label">Amount due</span>
          <span class="inv-detail-value highlight">{{format_currency grand_total}}</span>
        </div>
      </div>
    </div>

    <div class="inv-table-wrap">
      <table class="inv-table">
        <thead>
          <tr>
            <th style="width:24%">Service / Item</th>
            <th>Description</th>
            <th class="num" style="width:9%">Price</th>
            <th class="num" style="width:6%">Qty</th>
            <th class="num" style="width:10%">Discount</th>
            {{#if show_tax}}
            <th class="num" style="width:8%">Tax %</th>
            <th class="num" style="width:10%">Tax</th>
            {{/if}}
            <th class="num" style="width:10%">Total</th>
          </tr>
        </thead>
        <tbody>
          {{#each items}}
          <tr>
            <td>
              <div class="inv-item-name">{{name}}</div>
            </td>
            <td><span class="inv-item-desc">{{description}}</span></td>
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
    </div>

    <div class="inv-lower">
      <div class="inv-notes-block">
        {{#if terms_text}}
        <p class="inv-notes-head">Terms &amp; Conditions</p>
        <div class="inv-notes-text">{{terms_text}}</div>
        {{/if}}
        {{#if notes}}
        <p class="inv-notes-head">Notes to Patient</p>
        <div class="inv-notes-text">{{notes}}</div>
        {{/if}}
      </div>
      <div class="inv-totals-panel">
        <p class="inv-totals-caption">{{totals_amounts_caption}}</p>
        <div class="inv-totals-row">
          <span class="inv-totals-label">{{totals_subtotal_label}}</span>
          <span class="inv-totals-val">{{format_currency subtotal}}</span>
        </div>
        {{#if show_tax}}{{#if totals_tax_label}}
        <div class="inv-totals-row">
          <span class="inv-totals-label">{{totals_tax_label}}</span>
          <span class="inv-totals-val">{{format_currency tax_total}}</span>
        </div>
        {{/if}}{{/if}}
        <div class="inv-totals-row">
          <span class="inv-totals-label">{{totals_discount_label}}</span>
          <span class="inv-totals-val">{{format_currency discount_total}}</span>
        </div>
        <div class="inv-grand-total-box">
          <span class="inv-grand-label">{{totals_grand_label}}</span>
          <span class="inv-grand-amount">{{format_currency grand_total}}</span>
        </div>
        {{#if amount_in_words}}
        <p class="inv-amount-words">{{amount_in_words}}</p>
        {{/if}}
      </div>
    </div>

  </div>

  {{#if has_attachments}}
  <div class="inv-attachments">
    <p class="inv-attach-head">Attachments</p>
    <ul class="inv-attach-list">
      {{#each attachments}}
      <li class="inv-attach-item">{{file_name}}</li>
      {{/each}}
    </ul>
  </div>
  {{/if}}

  <div class="inv-footer-anchor">
    <div class="inv-payment-footer">
      <div>
        <p class="inv-pay-block-label">Payment method</p>
        <p class="inv-pay-block-value">{{payment_method_label}}</p>
      </div>
      <div>
        <p class="inv-pay-block-label">Tax method</p>
        <p class="inv-pay-block-value">{{tax_method_label}}</p>
      </div>
      <div class="inv-qr-box">QR / UPI<br>(coming soon)</div>
    </div>

    {{#if footer_html}}
    <div class="inv-doc-footer-graphic">
      <img src="{{footer_html}}" alt="Footer Graphic" class="footer-img" />
    </div>
    {{else}}
    <div class="inv-doc-footer-placeholder"></div>
    {{/if}}
  </div>

</div>`,
			Css: `@import url('https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&family=Fraunces:ital,wght@0,400;0,600;1,400&display=swap');

:root {
  --inv-primary: {{primary_color}};
  --inv-accent: {{accent_color}};
  --inv-font-body: {{body_font_family}};
  --inv-font-header: {{header_font_family}};
}

* { box-sizing: border-box; }

html, body {
  margin: 0;
  padding: 0;
  height: 100%;
}

.inv-root {
  font-family: var(--inv-font-body), 'Plus Jakarta Sans', system-ui, sans-serif;
  font-size: 13px;
  line-height: 1.5;
  color: #1a2332;
  background: #ffffff;
  max-width: 780px;
  margin: 0 auto;
  min-height: 100%;
  display: flex;
  flex-direction: column;
}

.inv-top-stripe {
  height: 4px;
  background: var(--inv-primary);
  width: 100%;
}

.inv-letterhead-banner {
  width: 100%;
  margin: 0;
  padding: 0;
  line-height: 0;
}

.inv-letterhead-banner-img {
  width: 100%;
  height: 120px;
  max-height: 130px;
  object-fit: cover;
  display: block;
}

.inv-letterhead-placeholder {
  min-height: 20px;
}

.inv-letterhead-media {
  width: 100%;
  margin: 0;
  padding: 0;
}

.inv-letterhead-media .letterhead-img {
  width: 100%;
  height: 120px;
  max-height: 130px;
  object-fit: cover;
  display: block;
}

.inv-letterhead-text-wrap {
  padding: 10px 36px 0;
}

.inv-letterhead {
  font-size: 12px; color: #6b8299; white-space: pre-wrap;
}

.inv-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 24px 36px 20px;
  border-bottom: 1px solid #e8edf2;
}

.inv-clinic-block { display: flex; align-items: center; gap: 14px; }

.inv-logo-circle {
  width: 52px; height: 52px;
  background: var(--inv-primary);
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}

.inv-logo-cross { position: relative; width: 22px; height: 22px; }
.inv-logo-cross::before, .inv-logo-cross::after {
  content: ''; position: absolute;
  background: #ffffff; border-radius: 2px;
}
.inv-logo-cross::before { width: 6px; height: 22px; left: 8px; top: 0; }
.inv-logo-cross::after  { width: 22px; height: 6px; left: 0; top: 8px; }

.brand-logo {
  width: 52px;
  height: 52px;
  max-width: 180px;
  max-height: 52px;
  object-fit: contain;
  flex-shrink: 0;
}
.inv-logo-circle .brand-logo {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.inv-clinic-name {
  font-size: 18px; font-weight: 700;
  color: #1a2332; margin: 0 0 2px;
}
.inv-clinic-tagline {
  font-size: 11px; color: #6b8299;
  letter-spacing: 0.06em; text-transform: uppercase; font-weight: 500;
}

.inv-doc-badge { text-align: right; }
.inv-doc-number { font-size: 12px; color: #6b8299; margin-top: 4px; letter-spacing: 0.04em; }

.inv-status-ribbon-wrap {
  padding: 0 36px;
  margin-top: 14px;
}

.inv-status-ribbon {
  background: #f0f8f5;
  border-left: 3px solid var(--inv-primary);
  padding: 10px 16px;
  display: flex; justify-content: space-between; align-items: center;
  border-radius: 0 6px 6px 0;
}
.inv-status-left { font-size: 12px; color: #3a6b5a; font-weight: 500; }
.inv-status-right { display: flex; gap: 24px; }
.inv-status-pill {
  font-size: 11px; font-weight: 600;
  background: var(--inv-primary); color: #ffffff;
  padding: 3px 12px; border-radius: 20px;
  letter-spacing: 0.04em; text-transform: uppercase;
}

.inv-body { 
  padding: 24px 36px; 
  flex-grow: 1;
}

.inv-watermark-wrap { position: relative; overflow: hidden; }
.inv-watermark-wrap.watermark-on::after {
  content: attr(data-watermark);
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
  font-size: 80px; font-weight: 700;
  color: var(--inv-primary); opacity: 0.04;
  transform: rotate(-25deg);
  pointer-events: none; z-index: 0; white-space: nowrap;
}
.inv-watermark-wrap > * { position: relative; z-index: 1; }

.inv-info-grid {
  display: grid; grid-template-columns: repeat(3, 1fr);
  gap: 20px; margin-bottom: 24px;
  padding-bottom: 24px; border-bottom: 1px solid #e8edf2;
}
.inv-info-card {
  background: #f8fafc;
  border: 1px solid #e8edf2;
  border-radius: 8px; padding: 14px 16px;
}
.inv-info-card-title {
  font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.1em;
  color: var(--inv-primary); margin: 0 0 10px;
  display: flex; align-items: center; gap: 6px;
}
.inv-info-card-title::before {
  content: ''; display: inline-block;
  width: 3px; height: 12px;
  background: var(--inv-primary); border-radius: 2px;
}
.inv-info-name { font-size: 14px; font-weight: 700; color: #1a2332; margin: 0 0 5px; }
.inv-info-line { font-size: 12px; color: #6b8299; margin: 3px 0; line-height: 1.5; }

.inv-detail-row {
  display: flex; justify-content: space-between;
  padding: 4px 0; font-size: 12px;
  border-bottom: 1px dashed #e8edf2;
}
.inv-detail-row:last-child { border-bottom: none; }
.inv-detail-label { color: #6b8299; }
.inv-detail-value { font-weight: 600; color: #1a2332; }
.inv-detail-value.highlight { color: var(--inv-primary); font-size: 13px; }

.inv-table-wrap { margin-bottom: 24px; }
.inv-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.inv-table thead { background: #1a2332; }
.inv-table thead th {
  color: #a8bccf; font-size: 10px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.08em;
  padding: 11px 12px; text-align: left;
}
.inv-table thead th:first-child { border-radius: 6px 0 0 0; }
.inv-table thead th:last-child  { border-radius: 0 6px 0 0; text-align: right; }
.inv-table thead th.num { text-align: right; }
.inv-table tbody tr { border-bottom: 1px solid #e8edf2; }
.inv-table tbody tr:nth-child(even) { background: #f8fafc; }
.inv-table tbody td { padding: 12px; vertical-align: top; }
.inv-table tbody td.num { text-align: right; color: #1a2332; }
.inv-item-name { font-weight: 600; font-size: 13px; color: #1a2332; margin: 0 0 2px; }
.inv-item-desc { font-size: 11px; color: #8fa3b4; }

.inv-table.striped tbody tr:nth-child(even) { background: #f0f8f5; }
.inv-table.bordered td, .inv-table.bordered th { border: 1px solid #e8edf2; }

.inv-lower {
  display: grid; grid-template-columns: 1fr 280px;
  gap: 28px;
}
.inv-notes-head {
  font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.1em;
  color: var(--inv-primary); margin: 0 0 8px;
  display: flex; align-items: center; gap: 6px;
}
.inv-notes-head::before {
  content: ''; display: inline-block;
  width: 3px; height: 12px;
  background: var(--inv-primary); border-radius: 2px;
}
.inv-notes-text {
  font-size: 12px; color: #6b8299;
  white-space: pre-wrap; line-height: 1.7;
  background: #f8fafc; border: 1px solid #e8edf2;
  border-radius: 6px; padding: 12px; margin-bottom: 16px;
}

.inv-totals-caption { font-size: 10px; color: #8fa3b4; text-align: right; margin: 0 0 6px; }
.inv-totals-row {
  display: flex; justify-content: space-between;
  padding: 7px 0; font-size: 12px;
  border-bottom: 1px solid #e8edf2;
}
.inv-totals-row:last-of-type { border-bottom: none; }
.inv-totals-label { color: #6b8299; }
.inv-totals-val { font-weight: 500; color: #1a2332; }

.inv-grand-total-box {
  background: #1a2332; border-radius: 8px;
  padding: 16px 18px;
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 12px;
}
.inv-grand-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.1em; color: #6b8299; }
.inv-grand-amount {
  font-family: var(--inv-font-header), 'Fraunces', Georgia, serif;
  font-size: 28px; font-weight: 600;
  color: var(--inv-primary); letter-spacing: -0.01em;
}
.inv-amount-words { font-size: 10px; color: #8fa3b4; text-align: right; margin-top: 8px; font-style: italic; }

.inv-attachments { padding: 0 36px 20px; }
.inv-attach-head { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em; color: var(--inv-primary); margin: 0 0 8px; }
.inv-attach-list { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 8px; }
.inv-attach-item {
  font-size: 11px; background: #f0f8f5; border: 1px solid #b2ddd0;
  color: var(--inv-primary); border-radius: 4px; padding: 4px 10px; font-weight: 500;
}

.inv-footer-anchor {
  margin-top: auto;
  width: 100%;
}

.inv-payment-footer {
  background: #f8fafc; border-top: 1px solid #e8edf2;
  padding: 16px 36px;
  display: grid; grid-template-columns: 1fr 1fr auto;
  gap: 20px; align-items: center;
}
.inv-pay-block-label {
  font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.08em;
  color: var(--inv-primary); margin-bottom: 4px;
}
.inv-pay-block-value { font-size: 12px; color: #1a2332; font-weight: 500; }
.inv-qr-box {
  width: 72px; height: 72px;
  border: 1.5px dashed #c8d8e4; border-radius: 6px;
  display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  font-size: 9px; color: #a8bccf; text-align: center; gap: 4px;
}

.inv-doc-footer-graphic {
  width: 100%;
  margin: 0;
  padding: 0;
  line-height: 0;
}

.inv-doc-footer-graphic .footer-img {
  width: 100%;
  height: 100px;
  max-height: 120px;
  object-fit: cover;
  display: block;
}

.inv-doc-footer-placeholder {
  min-height: 28px;
  display: block;
}
`,
			IsDefault: false,
			IsActive:  false,
		},
	}
}

func DefaultSettings(templateId uuid.UUID) Setting {
	termText := "Payment is due within 30 days of the invoice date. Late payments may incur a 2% monthly charge. All services rendered are non-refundable. For disputes, contact our billing department within 7 days."
	waterMarkText := "PAID"

	return Setting{
		TemplateId:       templateId,
		PrimaryColor:     "#1a6b5a",           // Deep clinic green — trust, health, care
		AccentColor:      "#2dd4a4",           // Mint accent — modern, fresh
		BodyFontFamily:   "Plus Jakarta Sans", // Clean, modern, highly legible
		HeaderFontFamily: "Fraunces",          // Elegant serif for invoice title & totals
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
