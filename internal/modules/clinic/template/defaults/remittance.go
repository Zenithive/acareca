package defaults

import "fmt"

// RemittancePayeeBanner returns the address banner for Remittance Advice
func RemittancePayeeBanner() string {
	return `<div class="address-banner-box"><div class="banner-label">PAYEE</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`
}

// RemittancePaymentDetailsTable returns the payment details table for Remittance
func RemittancePaymentDetailsTable() string {
	return `<div class="payment-details-container">
    <div class="payment-details-header">PAYMENT DETAILS</div>
    <table class="payment-details-table{{#if table_style_bordered}} payment-details-table-bordered{{/if}}{{#if table_style_striped}} payment-details-table-striped{{/if}}">
      <tbody>
        <tr>
          <td style="font-weight: bold; width: 45%%;">Payment method</td>
          <td style="width: 55%%;">{{coalesce custom_payment_method payment_method_label "Electronic funds transfer (EFT)"}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Account name</td>
          <td>{{coalesce custom_payment_account_name bill_to.name}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">BSB / Account No.</td>
          <td>{{coalesce custom_payment_bsb "063-000"}} / {{coalesce custom_payment_account "12345678"}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment date</td>
          <td>{{coalesce payment_date_display issue_date_display}}</td>
        </tr>
        <tr>
          <td style="font-weight: bold;">Payment reference</td>
          <td>{{invoice_number}}</td>
        </tr>
      </tbody>
    </table>
  </div>`
}

// RemittanceHTML returns the HTML body for the default Remittance Advice.
// Method-B suppresses this document entirely (patient fees collected
// directly by the dentist, so there's nothing to remit) — driven by
// billing_method.show_remittance rather than {{#unless is_method_b}}.
func RemittanceHTML() string {
	remittanceTable := DataTable(TableConfig{
		Title:         "NET AMOUNT PAYABLE TO YOU",
		ItemsVariable: "remittance_items",
		Columns: []TableColumn{
			{Header: "NET AMOUNT PAYABLE TO YOU", Width: "65%", Align: "left", FieldType: "text"},
			{Header: "Amount", Width: "20%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})
	
	return fmt.Sprintf(`{{#if billing_method.show_remittance}}<div class="invoice-page"><div style="display: block; width: 100%%;">%s</div>

  %s

  %s

  <div class="footer-notes-box">
    <p style="text-transform: lowercase; font-style: italic;"><span style="text-transform: none; font-style: italic;">This remittance advice is issued</span> {{invoice_frequency}} <span style="text-transform: none; font-style: italic;">together with the Calculation Statement (page 1) and {{billing_method.page_two_label}}. Please retain for your records and provide to your accountant at year end.</span></p>
  </div>
</div>{{/if}}`, Header("REMITTANCE ADVICE", "Reference", RemittancePayeeBanner()), remittanceTable, RemittancePaymentDetailsTable())
}
