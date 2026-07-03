package defaults

import "fmt"

// TaxInvoiceHTML returns the HTML body for the default Tax Invoice.
func TaxInvoiceHTML() string {
	return fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table" style="margin-top: 4px;">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">{{billing_method.service_fee_section_label}}</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      %s

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
      <td style="width: 50%%;">
        %s
      </td>
      <td style="width: 50%%; padding: 0; vertical-align: top;">
        %s
      </td>
    </tr>
  </table>

  <div class="footer-notes-box" style="margin-top: 24px;">
    <p style="font-style: italic;">{{#if payment_terms_resolved}}Payment terms: {{/if}}{{payment_terms_resolved}}</p>
  </div>
</div>`, Header("{{billing_method.tax_invoice_title}}", "{{billing_method.invoice_number_label}}", TaxInvoiceBillToBanner()), ServiceFeeIntroRow(), PaymentDetailsSection(), TaxSummarySection())
}

