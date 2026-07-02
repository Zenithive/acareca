package template

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/defaults"
)

// DefaultTemplates returns the full set of default global invoice document templates.
func DefaultTemplates() []RqGlobalTemplate {
	css := defaults.CSS()

	return []RqGlobalTemplate{
		{
			Name:      "Calculation Statement",
			IsDefault: true,
			IsActive:  true,
			Html:      defaults.CalculationHTML(),
			Css:       css,
		},
		{
			Name:      "Tax Invoice",
			IsDefault: false,
			IsActive:  true,
			Html:      defaults.TaxInvoiceHTML(),
			Css:       css,
		},
		{
			Name:      "Remittance Advice",
			IsDefault: false,
			IsActive:  true,
			Html:      defaults.RemittanceHTML(),
			Css:       css,
		},
	}
}

func DefaultSettings(templateId uuid.UUID) Setting {
	termText := "This invoice is settled by offset against patient fees collected on your behalf. No payment is required—refer to the attached Remittance Advice for the net amount payable to you."
	paymentTerms := termText
	waterMarkText := "PAID"
	tableStyle := "simple"

	return Setting{
		TemplateId:       templateId,
		MappingId:        nil,
		PrimaryColor:     "#1f4e5f",
		AccentColor:      "#5f96b4",
		BodyFontFamily:   "Arial",
		HeaderFontFamily: "Arial",
		IsLogo:           true,
		LogoId:           nil,
		LetterHeadId:     nil,
		FooterId:         nil,
		TermText:         &termText,
		PaymentTerms:     &paymentTerms,
		IsWaterMark:      false,
		WaterMarkText:    &waterMarkText,
		IsTax:            true,
		TableStyle:       &tableStyle,
	}
}
