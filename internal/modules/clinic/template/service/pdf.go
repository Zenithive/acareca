package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/render"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/templates"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/chromepdf"
	"github.com/iamarpitzala/acareca/pkg/config"
)

// IInvoiceReader is a narrow interface to avoid a circular import with the invoice package.
// Only the fields required by the PDF service are exposed.
type IInvoiceReader interface {
	GetInvoiceMethod(ctx context.Context, invoiceId uuid.UUID) (util.InvoiceType, error)
	// GetInvoiceRenderData returns a map[string]interface{} shaped exactly as the
	// Handlebars templates expect — bill_from.name, bill_to.address (string), etc.
	GetInvoiceRenderData(ctx context.Context, invoiceId uuid.UUID) (map[string]interface{}, error)
}

type IPDF interface {
	GeneratePDF(ctx context.Context, rq template.RqGeneratePDF) ([]byte, error)
	GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data common.Invoice) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
}

type PDFService struct {
	templateRepo  repository.ITemplateRepository
	settingSvc    ISetting
	invoiceReader IInvoiceReader
	encryption    IEncryptionService
	renderer      render.IPDFRenderer
	dataMapper    *render.DataMapper
	cfg           *config.Config
}

func NewPDFService(templateRepo repository.ITemplateRepository, settingSvc ISetting, encryption IEncryptionService, renderer render.IPDFRenderer, cfg *config.Config) IPDF {
	return &PDFService{
		templateRepo: templateRepo,
		settingSvc:   settingSvc,
		encryption:   encryption,
		renderer:     renderer,
		dataMapper:   render.NewDataMapper(),
		cfg:          cfg,
	}
}

// NewPDFServiceWithInvoiceReader creates a PDFService with an invoice reader for billing method resolution.
func NewPDFServiceWithInvoiceReader(templateRepo repository.ITemplateRepository, settingSvc ISetting, invoiceReader IInvoiceReader, encryption IEncryptionService, renderer render.IPDFRenderer, cfg *config.Config) IPDF {
	return &PDFService{
		templateRepo:  templateRepo,
		settingSvc:    settingSvc,
		invoiceReader: invoiceReader,
		encryption:    encryption,
		renderer:      renderer,
		dataMapper:    render.NewDataMapper(),
		cfg:           cfg,
	}
}

func (s *PDFService) GeneratePDF(ctx context.Context, rq template.RqGeneratePDF) ([]byte, error) {
	t, err := s.templateRepo.Get(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}

	html, css, err := s.encryption.DecryptTemplate(t.Html, t.Css)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt template: %w", err)
	}

	st, err := s.settingSvc.Get(ctx, uuid.Nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	if st != nil {
		s.applySettings(&rq.Data, st)
	}

	dataMap, err := s.dataMapper.ToMap(rq.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to map data: %w", err)
	}

	fullHTML, err := chromepdf.Render(html, css, dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}

	return s.renderer.RenderToPDF(ctx, fullHTML)
}

func (s *PDFService) GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data common.Invoice) ([]byte, error) {
	if len(templateIds) == 0 {
		return nil, template.ErrTemplateRequired
	}

	if err := s.templateRepo.ValidateAccess(ctx, templateIds); err != nil {
		return nil, err
	}

	var htmlBuilder, cssBuilder strings.Builder

	for _, tId := range templateIds {
		t, err := s.templateRepo.Get(ctx, tId)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch template %s: %w", tId, err)
		}

		html, css, err := s.encryption.DecryptTemplate(t.Html, t.Css)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt template: %w", err)
		}

		// Check size limits
		if err := render.ValidateTemplateSize(html, css); err != nil {
			return nil, err
		}

		htmlBuilder.WriteString(html)
		htmlBuilder.WriteString("\n")
		cssBuilder.WriteString(css)
		cssBuilder.WriteString("\n")
	}

	// Convert data to map
	dataMap, err := s.dataMapper.ToMap(data)
	if err != nil {
		return nil, fmt.Errorf("failed to map data: %w", err)
	}

	// Render HTML
	fullHTML, err := chromepdf.Render(htmlBuilder.String(), cssBuilder.String(), dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}

	// Generate PDF
	return s.renderer.RenderToPDF(ctx, fullHTML)
}

func (s *PDFService) DownloadPDF(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	// 1. Resolve billing method from the actual invoice record
	var billingMethodStr string
	if s.invoiceReader != nil {
		method, err := s.invoiceReader.GetInvoiceMethod(ctx, invoiceId)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve invoice method: %w", err)
		}
		billingMethodStr = string(method)
	}

	// 2. Determine template sequence from billing method type
	templateNames := common.GetTemplateNames(billingMethodStr)
	if len(templateNames) == 0 {
		return nil, "", fmt.Errorf("no template sequence defined for billing method %q", billingMethodStr)
	}

	// 3. Fetch fully-shaped render data and apply per-invoice style overrides
	var baseMap map[string]interface{}
	if s.invoiceReader != nil {
		fetched, err := s.invoiceReader.GetInvoiceRenderData(ctx, invoiceId)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch invoice render data: %w", err)
		}
		baseMap = fetched
	} else {
		baseMap = map[string]interface{}{"invoice_id": invoiceId.String()}
	}

	// Apply styling overrides from settings on top of the base map
	st, err := s.settingSvc.Get(ctx, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice settings: %w", err)
	}
	if st != nil {
		applySettingsToMap(baseMap, st, s.cfg)
	}

	// 4. Extract terms/payment settings for TemplateDataBuilder
	var templateTerms, paymentTermsSetting string
	if st != nil {
		if st.TermText != nil {
			templateTerms = *st.TermText
		}
		if st.PaymentTerms != nil {
			paymentTermsSetting = *st.PaymentTerms
		}
	}

	invoiceNotes, _ := baseMap["notes"].(string)
	invoicePaymentTerms, _ := baseMap["payment_terms"].(string)

	dataBuilder := templates.TemplateDataBuilder{
		Method:                       billingMethodStr,
		Notes:                        invoiceNotes,
		TemplateTermsText:            templateTerms,
		PaymentTerms:                 invoicePaymentTerms,
		TemplateSettingsPaymentTerms: paymentTermsSetting,
		BaseData:                     baseMap,
	}
	finalRenderMap := dataBuilder.Build()

	if st != nil {
		applySettingsToMap(finalRenderMap, st, s.cfg)
	}

	// 5. Fetch, decrypt, and render each template page individually with its specific context
	var renderedPagesBuilder strings.Builder

	for _, name := range templateNames {
		t, err := s.templateRepo.GetByName(ctx, name)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch template %q: %w", name, err)
		}

		html, css, err := s.encryption.DecryptTemplate(t.Html, t.Css)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt template %q: %w", name, err)
		}

		if err := render.ValidateTemplateSize(html, css); err != nil {
			return nil, "", fmt.Errorf("template %q exceeds size limit: %w", name, err)
		}

		// Create a localized clone of the context map specifically for this page
		pageRenderMap := make(map[string]interface{})
		for k, v := range finalRenderMap {
			pageRenderMap[k] = v
		}

		// Assign the specific document number to 'invoice_number' context based on template category
		nameUpper := strings.ToUpper(name)
		if ts, ok := finalRenderMap["template_settings"].(map[string]interface{}); ok {
			if strings.Contains(nameUpper, "CALCULATION") {
				pageRenderMap["invoice_number"] = ts["calculation_invoice_number"]
			} else if strings.Contains(nameUpper, "TAX") || strings.Contains(nameUpper, "SFA") || strings.Contains(nameUpper, "COMMISSION") || strings.Contains(nameUpper, "RCTI") {
				pageRenderMap["invoice_number"] = ts["tax_invoice_number"]
			} else if strings.Contains(nameUpper, "REMITTANCE") || strings.Contains(nameUpper, "SETTLEMENT") {
				pageRenderMap["invoice_number"] = ts["remittance_invoice_number"]
			}
		}

		// Compile this template page explicitly with its specific localized invoice number
		compiledPageHTML, err := chromepdf.Render(html, css, pageRenderMap)
		if err != nil {
			return nil, "", fmt.Errorf("failed to compile page template %q: %w", name, err)
		}

		renderedPagesBuilder.WriteString(compiledPageHTML)
		renderedPagesBuilder.WriteString("\n")
	}

	// 6. Render the concatenated multi-page HTML stream to PDF via headless Chromium
	pdfBytes, err := s.renderer.RenderToPDF(ctx, renderedPagesBuilder.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate final PDF document: %w", err)
	}

	filename := fmt.Sprintf("INVOICE_%s", invoiceId.String()[:8])
	return pdfBytes, filename, nil
}

func (s *PDFService) applySettings(data *common.Invoice, st *common.RsSetting) {
	if st == nil {
		return
	}

	data.PrimaryColor = st.PrimaryColor
	data.AccentColor = st.AccentColor
	data.BodyFontFamily = strings.ReplaceAll(st.BodyFontFamily, "+", " ")
	data.HeaderFontFamily = strings.ReplaceAll(st.HeaderFontFamily, "+", " ")

	data.TableStyleClass = st.TableStyle

	if st.TermText != nil {
		data.Notes = *st.TermText
		data.TermsText = *st.TermText
	}

	if st.IsWaterMark && st.WaterMarkText != nil {
		data.WatermarkEnabled = true
		data.WatermarkText = *st.WaterMarkText
	}

	if st.IsLogo && st.Logo != nil {
		data.ShowLogo = true
		data.ShowLogoImage = true
		data.LogoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + st.Logo.FileKey
	}
}

// applySettingsToMap injects template settings into the render map that the
// Handlebars templates read from template_settings.* keys.
func applySettingsToMap(m map[string]interface{}, st *common.RsSetting, cfg *config.Config) {
	if st == nil {
		return
	}

	existing, _ := m["template_settings"].(map[string]interface{})
	if existing == nil {
		existing = map[string]interface{}{}
	}

	existing["primary_color"] = st.PrimaryColor
	existing["accent_color"] = st.AccentColor
	existing["body_font_family"] = st.BodyFontFamily
	existing["header_font_family"] = st.HeaderFontFamily
	existing["is_watermark"] = st.IsWaterMark
	existing["is_logo"] = st.IsLogo

	bodyFontClean := strings.ReplaceAll(st.BodyFontFamily, "+", " ")
	headerFontClean := strings.ReplaceAll(st.HeaderFontFamily, "+", " ")

	existing["body_font_family"] = bodyFontClean
	existing["header_font_family"] = headerFontClean

	if bodyFontClean != "" {
		existing["body_font_family_css"] = strings.ReplaceAll(bodyFontClean, " ", "+")
	}
	if headerFontClean != "" {
		existing["header_font_family_css"] = strings.ReplaceAll(headerFontClean, " ", "+")
	}

	style := strings.ToLower(strings.TrimSpace(st.TableStyle))

	existing["table_style"] = style
	m["table_style_class"] = style

	// Backward-compatible template flags
	m["table_style_bordered"] = false
	m["table_style_striped"] = false

	switch style {
	case "bordered":
		m["table_style_bordered"] = true
	case "striped":
		m["table_style_striped"] = true
	case "bordered-striped", "striped-bordered":
		m["table_style_bordered"] = true
		m["table_style_striped"] = true
	}

	if st.TermText != nil {
		existing["terms_text"] = *st.TermText
		m["notes"] = *st.TermText
	}

	if st.IsWaterMark && st.WaterMarkText != nil {
		existing["watermark_text"] = *st.WaterMarkText
		m["watermark_enabled"] = true
	}

	if st.IsLogo && st.Logo != nil {
		logoURL := strings.TrimRight(cfg.R2StoragePrefix, "/") + "/" + st.Logo.FileKey
		existing["logo_url"] = logoURL
		m["logo_url"] = logoURL
		m["show_logo"] = true
	}

	m["template_settings"] = existing
}
