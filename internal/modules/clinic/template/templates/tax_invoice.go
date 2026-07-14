package templates

import "fmt"

// TaxInvoiceHTML returns the HTML body for the default Tax Invoice.
func TaxInvoiceHTML() string {
	return fmt.Sprintf(`{{#unless is_method_c}}{{#unless billing_method.show_rcti_note}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table" style="margin-top: 4px;">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">{{#if billing_method.service_fee_section_label}} {{billing_method.service_fee_section_label}} {{else}} SERVICE &amp; FACILITY FEE {{/if}}</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td style="width: 65%%; vertical-align: top; line-height: 1.5; color: #000000; padding-bottom: 8px;">
          Service and facility fee for the period {{billing_period}},<br>
          calculated at the agreed rate on net patient fees, comprising: <strong> Fee Rate </strong> <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
          <ol style="margin: 4px 0 0 0; list-style-type: decimal; padding-left: 20px;">
            <li style="padding-bottom: 8px;">Rent of dental surgery/room</li>
            <li style="padding-bottom: 8px;">Patient booking &amp; reception</li>
            <li style="padding-bottom: 8px;">Equipment &amp; instrument hire</li>
            <li>General administration &amp; support staff</li>
          </ol>
        </td>
        <td class="num" style="width: 20%%; vertical-align: top; text-align: right;"></td>
        <td class="center" style="width: 15%%; vertical-align: top;"></td>
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

  <table class="layout-table" style="margin-top: 4px; width: 100%%;">
    <tr>
      <td style="width: 50%%; vertical-align: top; padding: 0;"></td>
      <td style="width: 50%%; padding: 0; vertical-align: top;">
        %s
      </td>
    </tr>
  </table>

  <div style="width: 100%%; margin-top: 12px; display: block;">
    {{#if is_method_b}}
    <div class="payment-details-container" style="margin-top: 0px; width: 100%%;">
      <div class="payment-details-header">PAYMENT DETAILS - PAY TO CLINIC</div>
      <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}" style="width: 100%%; border-collapse: collapse;">
        <tbody>
          <tr>
            <td style="font-weight: bold; width: 30%%;">Account</td>
            <td style="width: 70%%;">{{bill_from.name}}</td>
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
    {{else if billing_method.show_payment_details}}
    <div class="payment-details-container" style="margin-top: 0px; width: 100%%;">
      <div class="payment-details-header">PAYMENT DETAILS - PAY TO CLINIC</div>
      <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}" style="width: 100%%; border-collapse: collapse;">
        <tbody>
          <tr>
            <td style="font-weight: bold; width: 30%%;">Account</td>
            <td style="width: 70%%;">{{bill_from.name}}</td>
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
    {{/if}}
  </div>

 {{#if payment_terms}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">Payment terms: {{payment_terms}}</p>
  </div>
  {{else if template_settings.payment_terms}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">Payment terms: {{template_settings.payment_terms}}</p>
  </div>
  {{else if payment_terms_resolved}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">Payment terms: {{payment_terms_resolved}}</p>
  </div>
  {{else}}
  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic; margin-bottom: 4px;">
      {{#if is_method_b}}
      Patient fees for the period were collected directly by the dentist. This tax invoice is the clinic's service &amp; facility fee (plus any costs paid by the clinic) and is payable by the dentist to the clinic at the account above.
      {{/if}}
    </p>
  </div>
  {{/if}}
</div>{{/unless}}{{/unless}}`, Header("TAX INVOICE", "Invoice No.", TaxInvoiceBillToBanner()), TaxSummarySection())
}
