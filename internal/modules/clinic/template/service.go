package template

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/aymerick/raymond"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/chromepdf"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IService interface {
	BulkCreate(ctx context.Context) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqGlobalTemplate) (*RsTemplate, error)
	Update(ctx context.Context, id uuid.UUID, rq RqGlobalTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context, types []string) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error)
	GetInvoiceSetting(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID, templateId []uuid.UUID) (map[uuid.UUID]*RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error)
	GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error)
	GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data InvoiceData) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
	BulkUpdateDefaults(ctx context.Context) error
}

type Service struct {
	repo          IRepository
	cfg           *config.Config
	encryptionKey []byte
}

func NewService(repo IRepository, cfg *config.Config) IService {
	if len(cfg.TemplateEncryptionKey) != 32 {
		panic(fmt.Sprintf("template service configuration error: key must be exactly 32 chars, got %d", len(cfg.TemplateEncryptionKey)))
	}
	return &Service{
		repo:          repo,
		cfg:           cfg,
		encryptionKey: []byte(cfg.TemplateEncryptionKey),
	}
}

func (s *Service) Create(ctx context.Context, rq RqGlobalTemplate) (*RsTemplate, error) {
	html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt layout stream: %w", err)
	}
	css, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt styling stream: %w", err)
	}

	t := Template{
		Name:        rq.Name,
		Description: nil,
		Html:        html,
		Css:         css,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}

	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, err
	}

	st := DefaultSettings(t.Id)
	st.MappingId = nil
	if err := s.repo.CreateSetting(ctx, &st); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, rq RqGlobalTemplate) (*RsTemplate, error) {
	html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt layout stream: %w", err)
	}
	css, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt styling stream: %w", err)
	}

	t := Template{
		Id:        id,
		Name:      rq.Name,
		Html:      html,
		Css:       css,
		IsDefault: rq.IsDefault,
		IsActive:  rq.IsActive,
	}

	if err := s.repo.Update(ctx, &t); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RsTemplate, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

// Internal helper function to enrich media file structures from cloud IDs
func (s *Service) enrichSettingDocuments(ctx context.Context, st *Setting) error {
	if st.LogoId != nil {
		logo, err := s.repo.GetDocumentByID(ctx, *st.LogoId)
		if err == nil && logo != nil {
			st.Logo = logo
		}
	}
	if st.LetterHeadId != nil {
		lh, err := s.repo.GetDocumentByID(ctx, *st.LetterHeadId)
		if err == nil && lh != nil {
			st.LetterHead = lh
		}
	}
	if st.FooterId != nil {
		f, err := s.repo.GetDocumentByID(ctx, *st.FooterId)
		if err == nil && f != nil {
			st.Footer = f
		}
	}
	return nil
}

// GetSetting retrieves the default template settings
func (s *Service) GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error) {
	st, err := s.repo.GetSetting(ctx, templateId)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("global default setting configuration not provisioned for template: %s", templateId)
	}

	if err := s.enrichSettingDocuments(ctx, st); err != nil {
		return nil, err
	}

	rs := st.ToRs()
	return &rs, nil
}

// GetInvoiceSetting retrieves custom settings for an invoice template
func (s *Service) GetInvoiceSetting(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID, templateId []uuid.UUID) (map[uuid.UUID]*RsSetting, error) {
	result := make(map[uuid.UUID]*RsSetting, len(templateId))

	for _, tId := range templateId {
		st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, []uuid.UUID{tId})
		if err != nil {
			return nil, fmt.Errorf("failed looking up configuration hierarchy for template %s: %w", tId, err)
		}
		if st == nil {
			// no setting found for this template — skip rather than fail the whole batch
			result[tId] = nil
			continue
		}

		if err := s.enrichSettingDocuments(ctx, st); err != nil {
			return nil, fmt.Errorf("failed enriching setting documents for template %s: %w", tId, err)
		}

		rs := st.ToRs()
		result[tId] = &rs
	}

	return result, nil
}

func (s *Service) UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error) {
	st := rq.ToDB()
	if err := s.repo.UpdateSetting(ctx, &st, rq.TemplateId); err != nil {
		return nil, err
	}
	rs := st.ToRs()
	return &rs, nil
}

func (s *Service) BulkCreate(ctx context.Context) (*[]RsTemplate, error) {
	rqs := DefaultTemplates()
	templates := make([]Template, 0, len(rqs))

	for _, rq := range rqs {
		html, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(rq.Html)), s.encryptionKey)
		if err != nil {
			return nil, err
		}
		cssBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(rq.Css)), s.encryptionKey)
		if err != nil {
			return nil, err
		}

		templates = append(templates, Template{
			Name:      rq.Name,
			Html:      html,
			Css:       cssBlob,
			IsDefault: rq.IsDefault,
			IsActive:  rq.IsActive,
		})
	}

	if err := s.repo.BulkCreate(ctx, templates); err != nil {
		return nil, err
	}

	for _, t := range templates {
		st := DefaultSettings(t.Id)
		st.MappingId = nil
		if err := s.repo.CreateSetting(ctx, &st); err != nil {
			return nil, err
		}
	}

	rs := make([]RsTemplate, 0, len(templates))
	for _, t := range templates {
		rsItem := t.ToRs()
		rsItem.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsItem.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs = append(rs, rsItem)
	}
	return &rs, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, types []string) (*util.RsList, error) {
	return s.repo.List(ctx, types)
}

func (s *Service) GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data InvoiceData) ([]byte, error) {
	if len(templateIds) == 0 {
		return nil, fmt.Errorf("at least one template ID is required")
	}
	if err := s.repo.ValidateTemplateAccess(ctx, templateIds); err != nil {
		return nil, err
	}
	return s.renderTemplatesPDF(ctx, templateIds, data)
}

func (s *Service) renderTemplatesPDF(ctx context.Context, templateIds []uuid.UUID, data InvoiceData) ([]byte, error) {
	var htmlBuilder, cssBuilder strings.Builder
	for _, tId := range templateIds {
		t, err := s.repo.Get(ctx, tId)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch template %s: %w", tId, err)
		}
		html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt html for template: %w", err)
		}
		css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt css for template: %w", err)
		}
		htmlBuilder.WriteString(html)
		htmlBuilder.WriteString("\n")
		cssBuilder.WriteString(css)
		cssBuilder.WriteString("\n")
	}

	dataMap, err := invoiceDataToMap(data)
	if err != nil {
		return nil, fmt.Errorf("failed mapping template data: %w", err)
	}

	fullHTML, err := chromepdf.Render(htmlBuilder.String(), cssBuilder.String(), dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML context: %w", err)
	}

	pdf, err := chromepdf.Generate(ctx, fullHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to generate output PDF: %w", err)
	}
	return pdf, nil
}

func (s *Service) GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error) {
	t, err := s.repo.Get(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}

	html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt html: %w", err)
	}
	css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt css: %w", err)
	}

	st, err := s.repo.GetSetting(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}

	if st != nil {
		if err := s.enrichSettingDocuments(ctx, st); err != nil {
			return nil, fmt.Errorf("failed enriching setting documents: %w", err)
		}

		rq.Data.PrimaryColor = st.PrimaryColor
		rq.Data.AccentColor = st.AccentColor
		rq.Data.BodyFontFamily = st.BodyFontFamily
		rq.Data.HeaderFontFamily = st.HeaderFontFamily
		rq.Data.ShowTax = st.IsTax
		if st.TableStyle != nil {
			rq.Data.TableStyleClass = *st.TableStyle
		}
		if st.TermText != nil {
			rq.Data.Notes = *st.TermText
			rq.Data.TermsText = *st.TermText
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			rq.Data.WatermarkEnabled = true
			rq.Data.WatermarkText = *st.WaterMarkText
		}
		if st.IsLogo {
			rq.Data.ShowLogo = true
			if st.LogoId != nil && st.Logo != nil {
				rq.Data.ShowLogoImage = true
				rq.Data.LogoURL = s.cfg.R2StoragePrefix + st.Logo.ToRsDocument().FileKey
			}
		}

		// Build template_settings map so CSS/HTML {{template_settings.*}} variables resolve
		watermarkText := "PAID"
		if st.WaterMarkText != nil {
			watermarkText = *st.WaterMarkText
		}
		termsText := ""
		if st.TermText != nil {
			termsText = *st.TermText
		}
		rq.Data.TemplateSettings = map[string]interface{}{
			"primary_color":      st.PrimaryColor,
			"accent_color":       st.AccentColor,
			"body_font_family":   st.BodyFontFamily,
			"header_font_family": st.HeaderFontFamily,
			"is_logo":            st.IsLogo,
			"is_watermark":       st.IsWaterMark,
			"watermark_text":     watermarkText,
			"is_tax":             st.IsTax,
			"terms_text":         termsText,
		}
	}

	dataMap, err := invoiceDataToMap(rq.Data)
	if err != nil {
		return nil, fmt.Errorf("failed mapping dynamic payload context: %w", err)
	}

	fullHTML, err := chromepdf.Render(html, css, dataMap)
	if err != nil {
		return nil, err
	}
	return chromepdf.Generate(ctx, fullHTML)
}

func (s *Service) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateIds []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {

	if len(templateIds) == 0 {
		return nil, "", fmt.Errorf("at least one template ID is required")
	}

	if err := s.repo.ValidateTemplateAccess(ctx, templateIds); err != nil {
		return nil, "", err
	}

	// Load invoice & Related Sections
	inv, err := s.repo.GetInvoice(ctx, clinicId, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice: %w", err)
	}

	sections, err := s.repo.GetInvoiceSectionMeta(ctx, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice sections: %w", err)
	}

	st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, templateIds)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice settings: %w", err)
	}

	if st != nil {
		if err := s.enrichSettingDocuments(ctx, st); err != nil {
			return nil, "", fmt.Errorf("failed enriching setting documents: %w", err)
		}
	}

	// Build Invoice Data Structures
	data := InvoiceToData(inv)

	ApplyPDFCollections(&data, inv.Items, sections, inv.InvoiceNumber)

	if data.PaymentDateDisplay == "" {
		data.PaymentDateDisplay = data.IssueDateDisplay
	}

	// Apply Template Settings (Colors, Fonts, Watermarks)
	s.applyInvoiceSettings(&data, st)

	// Build Context Parameter Map
	dataMap, err := invoiceDataToMap(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed mapping invoice data: %w", err)
	}

	if freq, ok := dataMap["invoice_frequency"].(string); ok && freq != "" {
		freqLower := strings.ToLower(freq)
		dataMap["invoice_frequency"] = strings.ToUpper(freqLower[:1]) + freqLower[1:]
	}

	// Fixed Page Sequence Array
	pageOrder := map[string]int{
		"Calculation Statement": 1,
		"Tax Invoice":           2,
		"Remittance Advice":     3,
	}

	type templateAsset struct {
		Order int
		HTML  string
		CSS   string
	}

	var assets []templateAsset

	for _, id := range templateIds {

		t, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, "", fmt.Errorf("failed fetching template %s: %w", id, err)
		}

		html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed decrypting html for template %s: %w", t.Name, err)
		}

		css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed decrypting css for template %s: %w", t.Name, err)
		}

		order, ok := pageOrder[t.Name]
		if !ok {
			continue
		}

		assets = append(assets, templateAsset{
			Order: order,
			HTML:  html,
			CSS:   css,
		})
	}

	if len(assets) == 0 {
		return nil, "", fmt.Errorf("no valid templates found")
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Order < assets[j].Order
	})

	// Render Pages with Isolated Header Reference Numbers
	var htmlBuilder strings.Builder
	var cssBuilder strings.Builder

	for _, a := range assets {
		if ts, ok := dataMap["template_settings"].(map[string]interface{}); ok {
			switch a.Order {
			case 1:
				dataMap["invoice_number"] = ts["calculation_invoice_number"]
			case 2:
				dataMap["invoice_number"] = ts["tax_invoice_number"]
			case 3:
				dataMap["invoice_number"] = ts["remittance_invoice_number"]
			}

			// Translate table border style into template-safe boolean flags
			if styleStr, exists := ts["table_style"].(string); exists {
				dataMap["table_style_bordered"] = (styleStr == "bordered")
				dataMap["table_style_striped"] = (styleStr == "striped")
			}
		}

		// Direct struct property fallback verification
		if dataMap["table_style_bordered"] == nil && data.TableStyleClass != "" {
			dataMap["table_style_bordered"] = (data.TableStyleClass == "bordered")
			dataMap["table_style_striped"] = (data.TableStyleClass == "striped")
		}

		renderedHTML, err := raymond.Render(a.HTML, dataMap)
		if err != nil {
			return nil, "", fmt.Errorf("failed rendering template page html: %w", err)
		}
		htmlBuilder.WriteString(renderedHTML)
		htmlBuilder.WriteString("\n")

		renderedCSS, err := raymond.Render(a.CSS, dataMap)
		if err != nil {
			return nil, "", fmt.Errorf("failed rendering template page css: %w", err)
		}
		cssBuilder.WriteString(renderedCSS)
		cssBuilder.WriteString("\n")
	}

	document := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
%s
</style>
</head>
<body>
%s
</body>
</html>
`,
		cssBuilder.String(),
		htmlBuilder.String(),
	)

	pdf, err := chromepdf.Generate(ctx, document)
	if err != nil {
		return nil, "", fmt.Errorf("failed generating pdf: %w", err)
	}

	return pdf, fmt.Sprintf("INVOICE_%s", data.InvoiceNumber), nil
}

func (s *Service) applyInvoiceSettings(data *InvoiceData, st *Setting) {

	if st == nil {
		data.TemplateSettings = map[string]interface{}{
			"primary_color":          "#1f4e5f",
			"accent_color":           "#1f4e5f",
			"body_font_family":       "Arial",
			"body_font_family_css":   "Arial",
			"header_font_family":     "Arial",
			"header_font_family_css": "Arial",
			"is_logo":                false,
			"is_watermark":           false,
			"watermark_text":         "PAID",
			"is_tax":                 true,
			"terms_text":             "",
			"table_style":            "simple",
		}
		return
	}

	data.PrimaryColor = st.PrimaryColor
	data.AccentColor = st.AccentColor
	data.BodyFontFamily = st.BodyFontFamily
	data.HeaderFontFamily = st.HeaderFontFamily
	data.ShowTax = st.IsTax

	if st.TableStyle != nil {
		data.TableStyleClass = *st.TableStyle
	}

	if st.TermText != nil {
		data.Notes = *st.TermText
		data.TermsText = *st.TermText
	}

	if st.IsWaterMark && st.WaterMarkText != nil {
		data.WatermarkEnabled = true
		data.WatermarkText = *st.WaterMarkText
	}

	if st.IsLogo {
		data.ShowLogo = true
		if st.Logo != nil && st.Logo.ToRsDocument().FileKey != "" {
			data.LogoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + st.Logo.ToRsDocument().FileKey
		}
	}

	watermark := "PAID"
	if st.WaterMarkText != nil {
		watermark = *st.WaterMarkText
	}

	terms := ""
	if st.TermText != nil {
		terms = *st.TermText
	}

	if data.TemplateSettings == nil {
		data.TemplateSettings = map[string]interface{}{}
	}

	bodyFontImport := st.BodyFontFamily
	bodyFontCSS := strings.ReplaceAll(st.BodyFontFamily, "+", " ")
	headerFontImport := st.HeaderFontFamily
	headerFontCSS := strings.ReplaceAll(st.HeaderFontFamily, "+", " ")

	data.TemplateSettings["primary_color"] = st.PrimaryColor
	data.TemplateSettings["accent_color"] = st.AccentColor
	data.TemplateSettings["body_font_family"] = bodyFontImport
	data.TemplateSettings["body_font_family_css"] = bodyFontCSS
	data.TemplateSettings["header_font_family"] = headerFontImport
	data.TemplateSettings["header_font_family_css"] = headerFontCSS
	data.TemplateSettings["is_logo"] = st.IsLogo
	data.TemplateSettings["is_watermark"] = st.IsWaterMark
	data.TemplateSettings["watermark_text"] = watermark
	data.TemplateSettings["is_tax"] = st.IsTax
	data.TemplateSettings["terms_text"] = terms

	if st.TableStyle != nil {
		data.TemplateSettings["table_style"] = *st.TableStyle
	} else {
		data.TemplateSettings["table_style"] = "simple"
	}
}

func (s *Service) BulkUpdateDefaults(ctx context.Context) error {
	freshTemplates := DefaultTemplates()
	existingList, err := s.repo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed listing global blueprints: %w", err)
	}

	existingMap := make(map[string]RsTemplate)
	if existingList != nil && existingList.Items != nil {
		switch items := existingList.Items.(type) {
		case []RsTemplate:
			for _, item := range items {
				existingMap[item.Name] = item
			}
		case []interface{}:
			for _, v := range items {
				if item, ok := v.(RsTemplate); ok {
					existingMap[item.Name] = item
				}
			}
		default:
			return fmt.Errorf("unexpected Items type in list response: %T", existingList.Items)
		}
	}

	for _, freshRq := range freshTemplates {
		htmlBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(freshRq.Html)), s.encryptionKey)
		if err != nil {
			return err
		}
		cssBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(freshRq.Css)), s.encryptionKey)
		if err != nil {
			return err
		}

		if existingMatched, exists := existingMap[freshRq.Name]; exists {
			t := Template{
				Id:        existingMatched.Id,
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.repo.Update(ctx, &t); err != nil {
				return fmt.Errorf("failed overwriting central design template text context '%s': %w", freshRq.Name, err)
			}

		} else {
			t := Template{
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.repo.Create(ctx, &t); err != nil {
				return fmt.Errorf("failed tracking fresh template layout baseline: %w", err)
			}

			mappingID := uuid.New()
			settingID := uuid.New()

			st := DefaultSettings(t.Id)
			st.Id = settingID
			st.MappingId = &mappingID

			if err := s.repo.CreateSetting(ctx, &st); err != nil {
				return fmt.Errorf("failed tracking default options values profiles: %w", err)
			}

			m := Mapping{
				ID:         mappingID,
				InvoiceID:  nil, // Explicitly NULL because it represents a default blueprint configuration
				TemplateID: t.Id,
				SettingID:  settingID,
				ClinicID:   nil, // System-wide global default
			}

			if err := s.repo.CreateMapping(ctx, &m); err != nil {
				return fmt.Errorf("failed creating global default mapping linkage: %w", err)
			}
		}
	}
	return nil
}

func freshRqHTMLFix(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if bytes, ok := v.([]byte); ok {
		return string(bytes)
	}
	return ""
}
