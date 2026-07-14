package templates

import "fmt"

// ServiceFeeIntroRowCalculation returns the service fee introduction row for Calculation Statement
func ServiceFeeIntroRowCalculation() string {
	return `<tr>
        <td colspan="3" style="border-bottom: none; padding-top: 5px; padding-bottom: 4px;">
          <table class="layout-table" style="width: 100%%; border-collapse: collapse;">
            <tr>
              <td style="padding: 0; color: black; width: 65%%; vertical-align: middle; line-height: 1.4;">
                {{service_fee_rate_intro.label}}
                <span style="display: inline-block; font-weight: bold; white-space: nowrap; margin-left: 6px;">
                  {{#if is_method_c}}Commission rate{{else}}{{#if billing_method.rate_label}}{{billing_method.rate_label}}{{else}}Fee rate{{/if}}{{/if}}&nbsp;
                  <span class="txt-blue-val">{{service_fee_rate_intro.fee_rate_display}}</span>
                </span>
              </td>
              <td class="num" style="width: 20%%; padding: 0; text-align: right; vertical-align: middle;">{{service_fee_rate_intro.amount_display}}</td>
              <td style="width: 15%%; padding: 0;"></td>
            </tr>
          </table>
        </td>
      </tr>`
}

// CalculationHTML returns the HTML body for the default Calculation Statement.
func CalculationHTML() string {
	treatmentCostsTable := DataTable(TableConfig{
		ItemsVariable: "treatment_cost_items",
		Columns: []TableColumn{
			{Header: "2. TREATMENT COSTS", Width: "50%", Align: "left", FieldType: "text"},
			{Header: "Paid By", Width: "20%", Align: "center", FieldType: "paid_by"},
			{Header: "Amount", Width: "15%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})

	netPatientFeesTable := DataTable(TableConfig{
		ItemsVariable: "net_patient_fee_items",
		Columns: []TableColumn{
			{Header: "3. NET PATIENT FEES", Width: "65%", Align: "left", FieldType: "text"},
			{Header: "Amount", Width: "20%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})

	return fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">1.  PATIENT FEES</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      {{#each patient_fee_items}}
      {{#if is_total}}
        {{#if ../patient_fee_adjustment}}
        <tr>
          <td colspan="3">
           <p style="font-weight: bold; background-color: rgb(from var(--accent-color) r g b / 0.25) !important; color: var(--primary-color); padding-left: 6px !important; padding-top: 1px !important; padding-bottom: 1px !important; border-left: 10px solid #ffffff !important; background-clip: padding-box !important;">Patient Adjustment (fee mis-allocation)</p>
          </td>
        </tr>
        {{#each ../patient_adjustment_items}}
        <tr>
          <td style="padding-left: 24px; font-size: 11px;">{{label}}</td>
          <td class="num" style="font-size: 11px;">{{format_table_amount this}}</td>
          <td class="center" style="font-size: 11px;">{{bas_code}}</td>
        </tr>
        {{/each}}
        {{/if}}
      {{/if}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;padding-left: 12px;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;{{#if is_bold}} font-weight: bold;{{/if}}">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  %s

  %s

  <table class="data-table">
    <thead>
      <tr>
        <th style="width: 65%%; text-align: left;">{{#if billing_method.service_fee_section_label}}{{billing_method.service_fee_section_label}}{{else}}4. SERVICE &amp; FACILITY FEE{{/if}}</th>
        <th style="width: 20%%; text-align: right;">Amount</th>
        <th style="width: 15%%; text-align: center;">BAS Code</th>
      </tr>
    </thead>
    <tbody>
      %s
      {{#each service_fee_items}}
      <tr{{#if row_class}} class="{{row_class}}"{{/if}}>
        <td style="width: 65%%;">{{label}}</td>
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;{{#if is_bold}} font-weight: bold;{{/if}}">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  <div class="footer-notes-box">
    {{#if notes}}
    <p style="font-style: italic; margin-top: 4px;">Notes: {{notes}}</p>
    {{else}}
      {{#if template_settings.terms_text}}
      <p style="font-style: italic; margin-top: 4px;">Notes: {{template_settings.terms_text}}</p>
      {{else}}
        {{#if footer_note}}
        <p style="font-style: italic; margin-top: 4px;">Notes: {{footer_note}}</p>
        {{/if}}
      {{/if}}
    {{/if}}
  </div>
</div>`, CalculationStatementHeader(DefaultPreparedForBanner()), treatmentCostsTable, netPatientFeesTable, ServiceFeeIntroRowCalculation())
}
