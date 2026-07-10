package templates

import "fmt"

// RemittanceHTML returns the HTML body for the default Remittance Advice.
func RemittanceHTML() string {
	remittanceTable := DataTable(TableConfig{
		ItemsVariable: "remittance_items",
		Columns: []TableColumn{
			{Header: "NET AMOUNT PAYABLE TO YOU", Width: "65%", Align: "left", FieldType: "text"},
			{Header: "Amount", Width: "20%", Align: "right", FieldType: "amount"},
			{Header: "BAS Code", Width: "15%", Align: "center", FieldType: "bas_code"},
		},
	})

	remittanceBannerHTML := `<div class="address-banner-box"><div class="banner-label">PAYEE</div><div class="recipient-name">{{bill_to.name}}</div>{{#if bill_to.address}}<p class="recipient-line">{{bill_to.address}}</p>{{else if address}}<p class="recipient-line">{{address}}</p>{{/if}}{{#if bill_to.abn}}<p class="recipient-line">ABN {{bill_to.abn}}</p>{{/if}}</div>`

	return fmt.Sprintf(`<div class="invoice-page">
  <div style="display: block; width: 100%%;">%s</div>

  %s

  %s

  <div class="footer-notes-box">
    <p style="text-transform: lowercase; font-style: italic;"><span style="text-transform: none; font-style: italic;">This remittance advice is issued</span> {{invoice_frequency}} <span style="text-transform: none; font-style: italic;">together with the Calculation Statement (page 1) and {{#if is_method_c}}RCTI (page 2){{else}}Tax Invoice (page 2){{/if}}. Please retain for your records and provide to your accountant at year end.</span></p>
  </div>
</div>`, 
		RemittanceHeader(remittanceBannerHTML), remittanceTable, RemittancePaymentDetailsTable(), 
	)
}
