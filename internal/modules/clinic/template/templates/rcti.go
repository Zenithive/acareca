package templates

import "fmt"

// RCTIHTML returns the HTML body for the default Recipient Created Tax Invoice.
func RCTIHTML() string {
	rctiBannerHTML := `<div class="address-banner-box"><div class="banner-label">SUPPLIER (DENTIST)</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}<p class="recipient-line" style="margin-top: 4px; font-size: 10px; font-style: italic; color: gray !important">Recipient: {{bill_from.name}} • ABN {{bill_from.abn}}<br>Issued by the recipient under an RCTI agreement between the parties.</p></div>`

	return fmt.Sprintf(`{{#if is_method_c}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

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

  <table class="layout-table" style="margin-top: 4px; width: 100%%;">
    <tr>
      <td style="width: 50%%; vertical-align: top; padding: 0;"></td>
      <td style="width: 50%%; padding: 0; vertical-align: top;">
        %s
      </td>
    </tr>
  </table>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS - PAY TO DENTIST</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}" style="width: 100%%; border-collapse: collapse;">
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
</div>{{else}}{{#if billing_method.show_rcti_note}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

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
                {{coalesce billing_method.tax_invoice_intro "Professional dental services for the period {{billing_period}}, remunerated at the agreed commission rate on net patient fees."}}
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

  <table class="layout-table" style="margin-top: 4px; width: 100%%;">
    <tr>
      <td style="width: 50%%; vertical-align: top; padding: 0;"></td>
      <td style="width: 50%%; padding: 0; vertical-align: top;">
        %s
      </td>
    </tr>
  </table>

  <div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS - PAY TO DENTIST</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}" style="width: 100%%; border-collapse: collapse;">
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

  %s
</div>{{/if}}{{/if}}`, Header("RECIPIENT CREATED TAX INVOICE", "RCTI No.", rctiBannerHTML), TaxSummarySection(), Header("RECIPIENT CREATED TAX INVOICE", "RCTI No.", rctiBannerHTML), TaxSummarySection(), FooterNotesSection("{{footer_note}}"))
}