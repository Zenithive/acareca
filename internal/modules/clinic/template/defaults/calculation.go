package defaults

import "fmt"

// ServiceFeeIntroRowCalculation returns the service fee introduction row for Calculation Statement
func ServiceFeeIntroRowCalculation() string {
	return `<tr>
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
      </tr>`
}

// CalculationHTML returns the HTML body for the default Calculation Statement.
func CalculationHTML() string {
	// Build patient fees table
	patientFeesTable := DataTable(TableConfig{
		Title:         "{{billing_method.patient_fees_label}}",
		ItemsVariable: "patient_fee_items",
		Columns: []TableColumn{
			{Header: "{{billing_method.patient_fees_label}}", Width: "65%", Align: "left", FieldType: "text"},
			{Header: "Amount", Width: "20%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})
	
	// Build settlement table
	settlementTable := DataTable(TableConfig{
		Title:         "3. NET SETTLEMENT",
		ItemsVariable: "settlement_items",
		Columns: []TableColumn{
			{Header: "3. NET SETTLEMENT", Width: "65%", Align: "left", FieldType: "text"},
			{Header: "Amount", Width: "20%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})
	
	return fmt.Sprintf(`<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  %s

  %s

  <table class="data-table">
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
        <td class="num{{#if value_class}} {{value_class}}{{/if}}" style="width: 20%%;{{#if is_bold}} font-weight: bold;{{/if}}">{{format_table_amount this}}</td>
        <td class="center" style="width: 15%%;">{{bas_code}}</td>
      </tr>
      {{/each}}
    </tbody>
  </table>

  %s

  %s
</div>`, CalculationStatementHeader(DefaultPreparedForBanner()), patientFeesTable, ServiceFeeIntroRowCalculation(), settlementTable, FooterNotesSection("{{footer_note}}"))
}
