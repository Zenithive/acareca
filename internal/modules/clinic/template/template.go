package template

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/templates"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// DefaultTemplates returns the full set of default global invoice document templates.
func DefaultTemplates() []RqGlobalTemplate {
	css := templates.CSS()

	return []RqGlobalTemplate{
		{
			Name:      "Calculation Statement",
			IsDefault: true,
			IsActive:  true,
			Html:      templates.CalculationHTML(),
			Css:       css,
		},
		{
			Name:      "Tax Invoice",
			IsDefault: false,
			IsActive:  true,
			Html:      templates.TaxInvoiceHTML(),
			Css:       css,
		},
		{
			Name:      "Recipient Created Tax Invoice",
			IsDefault: false,
			IsActive:  true,
			Html:      templates.RCTIHTML(),
			Css:       css,
		},
		{
			Name:      "Remittance Advice",
			IsDefault: false,
			IsActive:  true,
			Html:      templates.RemittanceHTML(),
			Css:       css,
		},
	}
}

func DefaultSettings(templateId uuid.UUID) common.Setting {
	termText := "This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you."
	paymentTerms := termText
	waterMarkText := "PAID"
	tableStyle := "simple"

	return common.Setting{
		InvoiceId:        nil,
		PrimaryColor:     "#1f4e5f",
		AccentColor:      "#5f96b4",
		BodyFontFamily:   "Arial",
		HeaderFontFamily: "Arial",
		IsLogo:           true,
		LogoId:           nil,
		TermText:         &termText,
		PaymentTerms:     &paymentTerms,
		IsWaterMark:      false,
		WaterMarkText:    &waterMarkText,
		TableStyle:       &tableStyle,
	}
}
