package defaults

import "fmt"

// calculationPreparedFor is the address banner specific to the Calculation Statement.
const calculationPreparedFor = `<div class="address-banner-box"><div class="banner-label">PREPARED FOR</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`

// CalculationHTML returns the HTML body for the default Calculation Statement.
// All method-a/b/c branching is resolved upstream into billing_method.* fields
// (see BillingMethodView / resolveBillingMethod) — this template only reads
// flat variables, no nested conditionals.
func CalculationHTML() string {
	return fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">{{billing_method.patient_fees_label}}</th>
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
        <th style="width: 65%%; text-align: left;">{{billing_method.service_fee_section_label}}</th>
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
                  {{billing_method.rate_label}}&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
              </td>
              <td class="num" style="width: 20%%; padding: 0; text-align: right; vertical-align: middle;">{{service_fee_rate_intro.amount_display}}</td>
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
        <th style="width: 65%%; text-align: left;">3. NET SETTLEMENT</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each settlement_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;{{#if is_bold}} font-weight: bold;{{/if}}">{{#if is_negative}}({{format_currency amount}}){{else}}{{format_currency amount}}{{/if}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="footer-notes-box">
    <p style="font-style: italic; margin-bottom: 4px;{{#if footer_note}} font-style: normal;{{/if}}"><strong>Notes:</strong> {{footer_note}}</p>
  </div>
</div>`, Header("CALCULATION STATEMENT", "Statement No.", calculationPreparedFor))
}
