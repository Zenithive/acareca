package template

import (
	"github.com/google/uuid"
)

// defaultTemplateHeader returns the HTML fragment for the two-column
// Calculation Statement header (clinic info + title + meta + PREPARED FOR).
// Keeping it separate makes it easy to test and reuse independently.
func defaultTemplateHeader() string {
	return `
  {{!-- ══════════════════════════════════════════════════════════════
       HEADER — CSS Grid, 2 columns × 2 rows
       ┌─────────────────────────────┬──────────────────────────────┐
       │ col-1 row-1                 │ col-2 row-1 + row-2          │
       │ Clinic name                 │ CALCULATION STATEMENT (h1)   │
       │ Address                     │                              │
       │ ABN | Ph | email            │ Statement No.  CS-2026-0042  │
       ├─────────────────────────────│ Issue Date     05 Jun 2026   │
       │ col-1 row-2                 │ Billing Period 1-31 May 2026 │
       │ ██ PREPARED FOR ██████████  │ Invoice Freq.  Monthly       │
       │ Recipient name              │                              │
       │ Address                     │                              │
       │ ABN                         │                              │
       └─────────────────────────────┴──────────────────────────────┘
  ══════════════════════════════════════════════════════════════════ --}}
  <div class="doc-header">

    {{!-- LEFT col, row 1 — clinic identity --}}
    <div class="hdr-clinic">
      <p class="hdr-clinic-name">{{bill_from.name}}</p>
      {{#if bill_from.address}}<p class="hdr-clinic-line">{{bill_from.address}}</p>{{/if}}
      <p class="hdr-clinic-contact">
        {{#if bill_from.abn}}ABN {{bill_from.abn}}{{/if}}
        {{#if bill_from.phone}}&nbsp;&nbsp;|&nbsp;&nbsp;Ph {{bill_from.phone}}{{/if}}
        {{#if bill_from.email}}&nbsp;&nbsp;|&nbsp;&nbsp;{{bill_from.email}}{{/if}}
      </p>
    </div>

    {{!-- RIGHT col, spans both rows — title + meta table --}}
    <div class="hdr-right">
      <h1 class="hdr-doc-title">CALCULATION STATEMENT</h1>
      <table class="hdr-meta">
        <tbody>
          <tr>
            <td class="hm-lbl">Statement No.</td>
            <td class="hm-val">{{invoice_number}}</td>
          </tr>
          <tr>
            <td class="hm-lbl">Issue Date</td>
            <td class="hm-val">{{issue_date_display}}</td>
          </tr>
          <tr>
            <td class="hm-lbl">Billing Period</td>
            <td class="hm-val">{{due_date_display}}</td>
          </tr>
          <tr>
            <td class="hm-lbl">Invoice Frequency</td>
            <td class="hm-val">{{payment_method_label}}</td>
          </tr>
          {{#if reference}}
          <tr>
            <td class="hm-lbl">Reference</td>
            <td class="hm-val">{{reference}}</td>
          </tr>
          {{/if}}
        </tbody>
      </table>
    </div>

    {{!-- LEFT col, row 2 — PREPARED FOR + recipient --}}
    <div class="hdr-prepared">
      <div class="hdr-prepared-banner">PREPARED FOR</div>
      <div class="hdr-prepared-body">
        <p class="hdr-recipient-name">{{bill_to.name}}</p>
        {{#if bill_to.address}}<p class="hdr-recipient-line">{{bill_to.address}}</p>{{/if}}
        {{#if bill_to.abn}}<p class="hdr-recipient-line">ABN {{bill_to.abn}}</p>{{/if}}
        {{#if bill_to.email}}<p class="hdr-recipient-line">{{bill_to.email}}</p>{{/if}}
        {{#if bill_to.phone}}<p class="hdr-recipient-line">{{bill_to.phone}}</p>{{/if}}
      </div>
    </div>

    {{!-- Spacer keeps right col from collapsing in row 2 --}}
    <div class="hdr-right-spacer"></div>

  </div>{{!-- /doc-header --}}`
}

// defaultTemplateHeaderCSS returns only the CSS rules for the header block.
// Scoped to the classes used by defaultTemplateHeader() so there is no leakage
// into the section or footer styles.
func defaultTemplateHeaderCSS() string {
	return `
/* ══════════════════════════════════════════════
   HEADER  —  CSS Grid 2 col × 2 row
   col widths: 58% left | 42% right
   row 1: clinic info  |  title + meta (spans r1+r2)
   row 2: prepared-for |  (spacer)
══════════════════════════════════════════════ */
.doc-header {
  display: grid;
  grid-template-columns: 58fr 42fr;
  grid-template-rows: auto auto;
  gap: 0 36px;
  padding: 24px 36px 20px;
  border-bottom: 1px solid #d1d5db;
}

/* ── Left col, row 1: clinic identity ── */
.hdr-clinic {
  grid-column: 1;
  grid-row: 1;
  align-self: start;
}
.hdr-clinic-name {
  font-size: 16px;
  font-weight: 700;
  color: #111827;
  margin: 0 0 4px;
  line-height: 1.2;
  font-family: var(--invoice-font-body), sans-serif;
}
.hdr-clinic-line {
  font-size: 12px;
  color: #374151;
  margin: 2px 0;
}
.hdr-clinic-contact {
  font-size: 12px;
  color: #374151;
  margin: 4px 0 0;
  white-space: nowrap;
}

/* ── Right col, rows 1-2: document title + meta ── */
.hdr-right {
  grid-column: 2;
  grid-row: 1 / 3;
  text-align: right;
  align-self: start;
}
.hdr-doc-title {
  font-family: var(--invoice-font-header), Georgia, "Times New Roman", serif;
  font-size: 24px;
  font-weight: 800;
  color: #111827;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  margin: 0 0 14px;
  line-height: 1;
  white-space: nowrap;
}
.hdr-meta {
  border-collapse: collapse;
  margin-left: auto;
  font-size: 12px;
}
.hdr-meta td {
  padding: 3px 0;
  vertical-align: top;
}
.hm-lbl {
  font-weight: 700;
  color: #374151;
  text-align: right;
  padding-right: 16px;
  white-space: nowrap;
}
.hm-val {
  color: #111827;
  text-align: right;
  white-space: nowrap;
  min-width: 110px;
}

/* ── Left col, row 2: PREPARED FOR ── */
.hdr-prepared {
  grid-column: 1;
  grid-row: 2;
  margin-top: 16px;
}
.hdr-prepared-banner {
  display: block;
  background: var(--invoice-primary);
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  padding: 7px 14px;
}
.hdr-prepared-body {
  padding: 10px 0 0;
}
.hdr-recipient-name {
  font-size: 14px;
  font-weight: 700;
  color: #111827;
  margin: 0 0 4px;
}
.hdr-recipient-line {
  font-size: 12px;
  color: #374151;
  margin: 2px 0;
}

/* Spacer aligns grid row 2 right cell */
.hdr-right-spacer {
  grid-column: 2;
  grid-row: 2;
}`
}

func DefaultTemplates(clinicId uuid.UUID) []RqTemplate {
	return []RqTemplate{
		{
			ClinicId:  clinicId,
			Name:      "Default Template",
			IsDefault: true,
			IsActive:  true,
			Html: `
<div class="invoice{{#if watermark_enabled}} watermark-on{{/if}}"{{#if watermark_enabled}} data-watermark="{{watermark_text}}"{{/if}}>

  {{#if letterhead_url}}
    <div class="lh-banner"><img src="{{letterhead_url}}" alt="Letterhead" class="lh-banner-img" /></div>
  {{else}}
    {{#if letterhead_html}}<div class="lh-text-wrap"><div class="lh-text">{{letterhead_html}}</div></div>
    {{else}}<div class="lh-empty"></div>{{/if}}
  {{/if}}

  <div class="invoice-body">

` + defaultTemplateHeader() + `

` + defaultTemplateMainSections() + `

    {{#if has_attachments}}
    <div class="attachments">
      <h4>Attachments</h4>
      <ul class="attachment-list">{{#each attachments}}<li>{{file_name}}</li>{{/each}}</ul>
    </div>
    {{/if}}

  </div>

  {{#if footer_html}}
  <footer class="doc-footer-banner"><img src="{{footer_html}}" alt="Footer" class="doc-footer-banner-img" /></footer>
  {{else}}
  <footer class="doc-footer-placeholder"></footer>
  {{/if}}

</div>
`,
			Css: `:root {
  --invoice-primary:     {{primary_color}};
  --invoice-accent:      {{accent_color}};
  --invoice-font-body:   {{body_font_family}};
  --invoice-font-header: {{header_font_family}};
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: var(--invoice-font-body), system-ui, -apple-system, sans-serif;
  font-size: 13px; line-height: 1.5; color: #111827; background: #fff;
}
.invoice {
  position: relative; max-width: 820px; margin: 0 auto;
  background: #fff; display: flex; flex-direction: column; min-height: 100%;
}
.invoice.watermark-on::before {
  content: attr(data-watermark);
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
  font-size: 80px; font-weight: 700;
  color: var(--invoice-primary); opacity: 0.05;
  transform: rotate(-28deg); pointer-events: none; z-index: 0; white-space: nowrap;
}
.invoice > * { position: relative; z-index: 1; }
.lh-banner { width: 100%; line-height: 0; }
.lh-banner-img { width: 100%; height: 120px; max-height: 130px; object-fit: cover; display: block; }
.lh-text-wrap { padding: 16px 36px 0; }
.lh-text { font-size: 12px; color: #6b7280; white-space: pre-wrap; }
.lh-empty { min-height: 20px; }
.invoice-body { flex-grow: 1; padding: 0 0 32px; }
` + defaultTemplateHeaderCSS() + `
` + defaultTemplateMainCSS() + `
table.items {
  width: calc(100% - 72px); margin: 20px 36px 0;
  border-collapse: collapse; font-size: 12px;
}
table.items thead th {
  background: #f3f4f6; color: #4b5563;
  font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em;
  padding: 10px 8px; text-align: left; border-bottom: 1px solid #d1d5db;
}
table.items thead th.num { text-align: right; }
table.items tbody td { padding: 12px 8px; border-bottom: 1px solid #d1d5db; vertical-align: top; }
table.items tbody td.num { text-align: right; white-space: nowrap; }
table.items.striped tbody tr:nth-child(even) { background: #fafafa; }
table.items.bordered td, table.items.bordered th { border: 1px solid #d1d5db; }
.lower { display: grid; grid-template-columns: 1fr 300px; gap: 28px; margin: 24px 36px 0; }
.lower h4 { font-size: 12px; font-weight: 600; margin: 0 0 8px; color: #374151; }
.lower .text-block { font-size: 12px; color: #4b5563; white-space: pre-wrap; margin-bottom: 16px; }
.summary .row { display: flex; justify-content: space-between; padding: 5px 0; font-size: 13px; }
.summary .row span:first-child { color: #6b7280; }
.totals-caption { text-align: right; font-size: 11px; color: #6b7280; margin: 0 0 8px; }
.total-due-box {
  margin-top: 12px; background: var(--invoice-primary); color: #fff;
  padding: 14px 16px; display: flex; justify-content: space-between; align-items: center; border-radius: 2px;
}
.tdb-label { font-size: 13px; font-weight: 500; }
.tdb-amount { font-size: 22px; font-weight: 700; }
.amount-words { margin-top: 10px; font-size: 11px; color: #6b7280; font-style: italic; }
.attachments { margin: 20px 36px 0; font-size: 12px; }
.attachments h4 { font-size: 12px; font-weight: 600; margin: 0 0 8px; color: #374151; }
.attachment-list { margin: 0; padding-left: 18px; color: #4b5563; }
.attachment-list li { margin: 4px 0; }
.doc-footer-banner { width: 100%; margin: 0; padding: 0; line-height: 0; }
.doc-footer-banner-img { width: 100%; height: 100px; max-height: 120px; object-fit: cover; display: block; }
.doc-footer-placeholder { min-height: 28px; display: block; }
`,
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
		PrimaryColor:     "#1a6b5a",
		AccentColor:      "#2dd4a4",
		BodyFontFamily:   "Plus Jakarta Sans",
		HeaderFontFamily: "Fraunces",
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

// ─────────────────────────────────────────────────────────────────────────────
// DefaultCalcConsts — all constant labels, BAS codes, bullet items and the
// fee/GST rates for the 3-section Calculation Statement layout.
//
// NONE of these values come from the frontend or the database.
// They are fixed accounting document constants, defined here the same way
// DefaultSettings defines colours and fonts.
// The service calls DefaultCalcConsts() to build []CalcSection at render time,
// using only the invoice monetary totals as dynamic inputs.
// ─────────────────────────────────────────────────────────────────────────────

// CalcConsts holds every constant needed to render the 3 sections.
type CalcConsts struct {
	// ── Rates ──────────────────────────────────────────────────────
	FeeRatePct float64 // Clinic service/facility fee rate (e.g. 60.0 = 60%)
	GSTRatePct float64 // GST rate applied to the service fee (default 10%)

	// ── Section titles ─────────────────────────────────────────────
	Sec1Title string
	Sec2Title string
	Sec3Title string

	// ── Section 1 row labels: Patient Fees Collected ───────────────
	Sec1TotalCollected string // G1 row
	Sec1GSTCollected   string // 1A row
	Sec1GSTFreeSales   string // G3 row
	Sec1LessLabFees    string // lab fees row
	Sec1NetPatientFees string // net total row (bold)

	// ── Section 2 row labels: Service & Facility Fee ───────────────
	Sec2ServicesIntro   string // intro row with fee-rate pill
	Sec2ServiceFee      string // calculated fee row (bold)
	Sec2GSTOnServiceFee string // GST row
	Sec2TotalServiceFee string // total incl GST (bold)

	// ── Section 2 bullet items (dynamically rendered as a numbered list) ──
	// Add, remove or reorder items freely — the template loops over them.
	ServiceItems []string

	// ── Section 3 row labels: Net Settlement ──────────────────────
	Sec3TotalCollected  string // G1 echo row
	Sec3LessLabFees     string // negative lab row
	Sec3LessServiceFee  string // negative service fee row
	Sec3AmountDue       string // amount due row (bold)
	Sec3LessRetainers   string // retainers/drawings row (blue, negative)
	Sec3BalanceRemitted string // final balance (bold)

	// ── Footer note ───────────────────────────────────────────────
	FooterNote string

	// ── Dummy constant amounts used when real values are not available ─
	// These are display-only defaults. They make the template render a
	// realistic-looking document without any frontend / DB input.
	// Replace with real DB-sourced values once the data pipeline exists.
	DefaultLabFees   float64 // laboratory fees (net of GST) — default constant
	DefaultRetainers float64 // drawings / retainers previously paid — default constant
}

// DefaultCalcConsts returns the built-in constants for the Calculation Statement.
// Change a label, BAS code, rate or bullet item here and it reflects everywhere.
func DefaultCalcConsts() CalcConsts {
	return CalcConsts{
		FeeRatePct: 60.0,
		GSTRatePct: 10.0,

		Sec1Title: "PATIENT FEES COLLECTED ON YOUR BEHALF",
		Sec2Title: "SERVICE & FACILITY FEE  (see Tax Invoice \u2014 page 2)",
		Sec3Title: "NET SETTLEMENT  (see Remittance Advice \u2014 page 3)",

		// Section 1
		Sec1TotalCollected: "Total patient fees collected (incl. GST)",
		Sec1GSTCollected:   "GST collected on patient fees (taxable services)",
		Sec1GSTFreeSales:   "GST-free sales  [G1 \u2212 (1A \u00d7 11)]",
		Sec1LessLabFees:    "Less: laboratory fees (net of GST)",
		Sec1NetPatientFees: "Net patient fees  [G1 \u2212 1A \u2212 lab fees]",

		// Section 2
		Sec2ServicesIntro:   "Services rendered to you for the period, including:",
		Sec2ServiceFee:      "Service & Facility Fee  [net patient fees \u00d7 fee rate]",
		Sec2GSTOnServiceFee: "GST on Service & Facility Fee (10%)",
		Sec2TotalServiceFee: "Total Service & Facility Fee (incl. GST)",

		// Bullet items — fully dynamic array, add/remove freely
		ServiceItems: []string{
			"Rent of dental surgery/room",
			"Patient booking & reception",
			"Fee collection & banking",
			"Equipment & instrument hire",
			"General administration & support staff",
		},

		// Section 3
		Sec3TotalCollected:  "Total patient fees collected (incl. GST)  [G1]",
		Sec3LessLabFees:     "Less: laboratory fees (net of GST)",
		Sec3LessServiceFee:  "Less: Total Service & Facility Fee (incl. GST)",
		Sec3AmountDue:       "Amount due to dentist",
		Sec3LessRetainers:   "Less: retainers / drawings previously paid this period",
		Sec3BalanceRemitted: "BALANCE REMITTED TO DENTIST",

		FooterNote: "Notes: Total patient fees, GST collected (1A) and laboratory fees are sourced from the " +
			"practice management system for the billing period. Blue cells are inputs; all other figures are " +
			"calculated. BAS codes are shown for the clinic\u2019s activity statement.",

		// Constant dummy amounts — shown when real values are not yet wired in.
		// These match the sample figures in the target Calculation Statement image.
		DefaultLabFees:   3150.00, // laboratory fees (net of GST)
		DefaultRetainers: 5000.00, // retainers / drawings previously paid this period
	}
}

// defaultTemplateMainSections returns the Handlebars HTML for the 3-section
// calculation body. It is separate from the header so each part can be read,
// tested and maintained independently.
func defaultTemplateMainSections() string {
	return `
    {{!-- ══════════════════════════════════════════════════════════════
         CALCULATION STATEMENT — 3 numbered sections
         Data is built entirely by buildCalcSections() in service.go.
         No frontend or DB input reaches this block.
    ══════════════════════════════════════════════════════════════════ --}}
    {{#each calc_sections}}
    <div class="cs-section">

      {{!-- Section header bar: number + title + Amount col + optional BAS col --}}
      <div class="cs-sec-hdr">
        <span class="cs-sec-title">{{number}}.&nbsp;&nbsp;{{title}}</span>
        <span class="cs-col-hdr cs-col-amount">Amount</span>
        {{#if show_bas_column}}<span class="cs-col-hdr cs-col-bas">BAS Code</span>{{/if}}
      </div>

      {{!-- Data rows — each row is a CalcRow struct converted to map --}}
      {{#each rows}}
        {{#if fee_rate}}
        {{!-- Special intro row: shows "Fee rate  60.0%" instead of a money amount --}}
        <div class="cs-row{{#if is_bold}} cs-bold{{/if}}{{#if indent}} cs-indent{{/if}}">
          <span class="cs-lbl">{{label}}</span>
          <span class="cs-amt">
            <span class="cs-fee-lbl">Fee rate</span>
            <span class="cs-fee-val">{{fee_rate}}</span>
          </span>
          {{#if ../show_bas_column}}<span class="cs-bas">{{bas_code}}</span>{{/if}}
        </div>
        {{else}}
        {{!-- Normal money row --}}
        <div class="cs-row{{#if is_bold}} cs-bold{{/if}}{{#if indent}} cs-indent{{/if}}">
          <span class="cs-lbl">{{label}}</span>
          <span class="cs-amt{{#if is_blue}} cs-blue{{/if}}{{#if is_negative}} cs-neg{{/if}}">
            {{format_currency amount}}
          </span>
          {{#if ../show_bas_column}}<span class="cs-bas">{{bas_code}}</span>{{/if}}
        </div>
        {{/if}}
      {{/each}}

      {{!-- Optional numbered bullet list (Section 2 service items) --}}
      {{#if service_items}}
      <ul class="cs-svc-list">
        {{#each service_items}}<li>{{label}}</li>{{/each}}
      </ul>
      {{/if}}

    </div>{{!-- /cs-section --}}
    {{/each}}

    {{!-- Italic footer note below all sections --}}
    {{#if footer_note}}<p class="cs-footer-note">{{footer_note}}</p>{{/if}}

    {{!-- Optional notes block (terms text injected from settings) --}}
    {{#if notes}}<div class="cs-notes-block"><p class="cs-notes-text">{{notes}}</p></div>{{/if}}`
}

// defaultTemplateMainCSS returns only the CSS rules for the 3 calculation
// sections and the footer note. Scoped to cs-* classes.
func defaultTemplateMainCSS() string {
	return `
/* ══ Calculation sections ══════════════════════════════════════════
   Each .cs-section has:
     • .cs-sec-hdr  — dark primary header bar (number + title + col headers)
     • .cs-row      — data row (flex, baseline-aligned)
     • .cs-svc-list — numbered bullet list (Section 2 only)
   Column widths mirror the image: label flex-1, amount 120px, BAS 80px
═══════════════════════════════════════════════════════════════════ */
.cs-section {
  border: 1px solid #d1d5db;
  margin: 20px 36px 0;
}

/* Section header bar */
.cs-sec-hdr {
  display: flex;
  align-items: center;
  background: var(--invoice-primary);
  color: #ffffff;
  padding: 9px 14px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.cs-sec-title { flex: 1; }
.cs-col-hdr   { font-weight: 700; white-space: nowrap; }
.cs-col-amount { width: 120px; text-align: right; }
.cs-col-bas    { width: 80px;  text-align: right; }

/* Data rows */
.cs-row {
  display: flex;
  align-items: baseline;
  padding: 7px 14px;
  font-size: 12px;
  border-bottom: 1px solid #f3f4f6;
  background: #ffffff;
}
.cs-row:last-child          { border-bottom: none; }
.cs-row:nth-child(even)     { background: #fafafa; }
.cs-bold                    { font-weight: 700; background: #f3f4f6 !important; }
.cs-indent .cs-lbl          { padding-left: 28px; }

/* Row label */
.cs-lbl           { flex: 1; font-size: 12px; color: #374151; }
.cs-bold .cs-lbl  { color: #111827; }

/* Amount cell */
.cs-amt           { width: 120px; text-align: right; font-size: 12px; color: #111827; white-space: nowrap; }
.cs-bold .cs-amt  { font-weight: 700; }
.cs-blue          { color: #2563eb; }           /* input / user-entered values */
.cs-neg::before   { content: "("; }             /* negative parenthesis prefix */
.cs-neg::after    { content: ")"; }

/* BAS Code cell */
.cs-bas           { width: 80px; text-align: right; font-size: 12px; color: #6b7280; }
.cs-bold .cs-bas  { font-weight: 700; color: #111827; }

/* Fee-rate inline pill (Section 2 intro row) */
.cs-fee-lbl { font-weight: 700; color: #111827; margin-right: 8px; }
.cs-fee-val { color: #2563eb; font-weight: 700; }

/* Numbered service bullet list */
.cs-svc-list {
  list-style: decimal;
  margin: 0;
  padding: 6px 14px 10px 52px;
  background: #fafafa;
  font-size: 12px;
  color: #374151;
}
.cs-svc-list li { padding: 3px 0; }

/* Italic footer note */
.cs-footer-note {
  font-size: 11px;
  color: #6b7280;
  font-style: italic;
  line-height: 1.6;
  margin: 20px 36px 0;
  padding-top: 12px;
  border-top: 1px solid #d1d5db;
}

/* Notes / terms block */
.cs-notes-block {
  margin: 12px 36px 0;
  padding: 12px 14px;
  background: #f3f4f6;
  border-left: 3px solid var(--invoice-primary);
}
.cs-notes-text { font-size: 12px; color: #374151; white-space: pre-wrap; }`
}

