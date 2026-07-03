package defaults

import "fmt"

// TaxInvoiceBillToBanner returns the address banner for Tax Invoice with RCTI support
func TaxInvoiceBillToBanner() string {
	return `<div class="address-banner-box"><div class="banner-label">{{billing_method.bill_to_label}}</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}{{#if billing_method.show_rcti_note}}<p class="recipient-line" style="margin-top: 4px; font-size: 10px; font-style: italic;">Recipient: {{bill_from.name}} ABN {{bill_from.abn}}<br>Issued by the recipient under an RCTI agreement between the parties.</p>{{/if}}</div>`
}

// PaymentDetailsSection returns the payment details table HTML
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

// ServiceFeeIntroRow returns the service fee introduction row HTML
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
