package defaults

import "fmt"

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
