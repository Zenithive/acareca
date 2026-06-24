package template

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

func (s *Service) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	if len(templateId) == 0 {
		return nil, "", fmt.Errorf("at least one template ID is required")
	}

	if err := s.repo.ValidateTemplateAccess(ctx, templateId); err != nil {
		return nil, "", err
	}

	inv, err := s.repo.GetInvoice(ctx, clinicId, invoiceId)
	if err != nil {
		if errors.Is(err, ErrInvoiceNotFound) {
			return nil, "", fmt.Errorf("failed to fetch invoice: invoice record matching id %s not found for clinic %s", invoiceId, clinicId)
		}
		return nil, "", fmt.Errorf("failed to fetch invoice: %w", err)
	}

	var htmlBuilder, cssBuilder strings.Builder

	for _, tId := range templateId {
		t, err := s.repo.Get(ctx, tId)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, "", fmt.Errorf("template %s not found or inaccessible", tId)
			}
			return nil, "", fmt.Errorf("failed to fetch base template structural data: %w", err)
		}
		html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt html for template %s: %w", tId, err)
		}
		css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt css for template %s: %w", tId, err)
		}

		htmlBuilder.WriteString(html)
		htmlBuilder.WriteString("\n")
		cssBuilder.WriteString(css)
		cssBuilder.WriteString("\n")
	}

	html := htmlBuilder.String()
	css := cssBuilder.String()

	st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, templateId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice settings: %w", err)
	}

	// Fetch section metadata (section type, payment fields, document number)
	sections, err := s.repo.GetInvoiceSectionMeta(ctx, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice section metadata: %w", err)
	}

	data := InvoiceToData(inv)

	// Build per-section line item collections
	collections, invoiceNumber, paymentMeta := buildInvoiceCollections(inv.Items, sections, inv.InvoiceNumber)
	data.PatientFeeItems = collections.patientFeeItems
	data.ServiceFeeItems = collections.serviceFeeItems
	data.SettlementItems = collections.settlementItems
	data.TaxInvoiceItems = collections.taxInvoiceItems
	data.RemittanceItems = collections.remittanceItems
	data.Subtotal = collections.subtotal
	data.TaxTotal = collections.taxTotal
	data.GrandTotal = collections.grandTotal
	data.CustomFeeRate = collections.customFeeRate
	if invoiceNumber != "" {
		data.InvoiceNumber = invoiceNumber
	}
	data.CustomPaymentMethod = paymentMeta.paymentMethod
	data.PaymentMethodLabel = paymentMeta.paymentMethod
	data.CustomPaymentAccountName = paymentMeta.accountName
	data.CustomPaymentBsb = paymentMeta.bsb
	data.CustomPaymentAccount = paymentMeta.accountNumber
	data.PaymentDateDisplay = FormatDateString(paymentMeta.paymentDate)

	if st != nil {
		data.PrimaryColor = st.PrimaryColor
		data.AccentColor = st.AccentColor
		data.BodyFontFamily = st.BodyFontFamily
		data.HeaderFontFamily = st.HeaderFontFamily
		data.ShowTax = st.IsTax
		if st.TableStyle != nil {
			data.TableStyleClass = *st.TableStyle
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			data.WatermarkEnabled = true
			data.WatermarkText = *st.WaterMarkText
		}
		if st.TermText != nil {
			data.Notes = *st.TermText
			data.TermsText = *st.TermText
		}
		if st.IsLogo {
			data.ShowLogo = true
			if st.LogoId != nil {
				logo, err := s.repo.GetDocumentByID(ctx, *st.LogoId)
				if err == nil && logo != nil {
					data.ShowLogoImage = true
					data.LogoURL = s.cfg.R2StoragePrefix + logo.ToRsDocument().FileKey
				}
			} else if len(inv.ClinicName) > 0 {
				data.LogoInitial = string([]rune(inv.ClinicName)[0])
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
		data.TemplateSettings = map[string]interface{}{
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

	dataMap, err := invoiceDataToMap(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed mapping static properties schema payload: %w", err)
	}

	fullHTML, err := chromepdf.Render(html, css, dataMap)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render HTML: %w", err)
	}

	pdf, err := chromepdf.Generate(ctx, fullHTML)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	filename := fmt.Sprintf("INVOICE %s", inv.InvoiceNumber)
	return pdf, filename, nil
}

type invoiceCollections struct {
	patientFeeItems []map[string]interface{}
	serviceFeeItems []map[string]interface{}
	settlementItems []map[string]interface{}
	taxInvoiceItems []map[string]interface{}
	remittanceItems []map[string]interface{}
	subtotal        float64
	taxTotal        float64
	grandTotal      float64
	customFeeRate   string
}

type invoicePaymentMeta struct {
	paymentMethod string
	accountName   string
	bsb           string
	accountNumber string
	paymentDate   string
}

// buildInvoiceCollections categorises flat invoice items into per-template
// collections using the section type stored alongside each item.
func buildInvoiceCollections(items []InvoiceItem, sections []InvoiceSectionMeta, fallbackInvoiceNumber string) (invoiceCollections, string, invoicePaymentMeta) {
	var c invoiceCollections
	c.customFeeRate = "0"
	var meta invoicePaymentMeta
	invoiceNumber := fallbackInvoiceNumber

	// Build a lookup of section ID → section meta
	sectionByID := make(map[uuid.UUID]InvoiceSectionMeta, len(sections))
	for _, sec := range sections {
		sectionByID[sec.ID] = sec
	}

	// Collect per-section payment fields and document number from first section
	for i, sec := range sections {
		if i == 0 && sec.DocumentNumber != "" {
			invoiceNumber = sec.DocumentNumber
		}
		if sec.SectionType == "REMITTANCE_INVOICE" || sec.SectionType == "REMITTANCE_ADVICE" {
			if sec.PaymentMethod != nil {
				meta.paymentMethod = *sec.PaymentMethod
			}
			if sec.AccountName != nil {
				meta.accountName = *sec.AccountName
			}
			if sec.Bsb != nil {
				meta.bsb = *sec.Bsb
			}
			if sec.AccountNumber != nil {
				meta.accountNumber = *sec.AccountNumber
			}
			if sec.PaymentDate != nil {
				meta.paymentDate = *sec.PaymentDate
			}
			break
		} else {
			if sec.PaymentMethod != nil && meta.paymentMethod == "" {
				meta.paymentMethod = *sec.PaymentMethod
			}
			if sec.AccountName != nil && meta.accountName == "" {
				meta.accountName = *sec.AccountName
			}
			if sec.Bsb != nil && meta.bsb == "" {
				meta.bsb = *sec.Bsb
			}
			if sec.AccountNumber != nil && meta.accountNumber == "" {
				meta.accountNumber = *sec.AccountNumber
			}
			if sec.PaymentDate != nil && meta.paymentDate == "" {
				meta.paymentDate = *sec.PaymentDate
			}
		}
	}

	for _, it := range items {
		basStr := ""
		if it.BASCode != nil {
			basStr = *it.BASCode
		}
		fieldKey := ""
		if it.FieldKey != nil {
			fieldKey = *it.FieldKey
		}
		isCredit := strings.ToUpper(it.EntryType) == "CREDIT"

		itemMap := map[string]interface{}{
			"label":       it.Name,
			"description": it.Description,
			"amount":      it.Amount,
			"bas_code":    basStr,
			"entry_type":  it.EntryType,
			"row_class":   "",
			"value_class": "",
		}
		if it.IsFinal {
			itemMap["row_class"] = "row-final-balance"
		}

		sectionTypeUpper := strings.ToUpper(it.SectionType)

		switch {
		case sectionTypeUpper == "CALCULATION_STATEMENT":
			keyUpper := strings.ToUpper(fieldKey)
			if strings.Contains(keyUpper, "FACILITY") || strings.Contains(keyUpper, "SERVICE") {
				if strings.Contains(keyUpper, "RATE") && c.customFeeRate == "0" {
					c.customFeeRate = fmt.Sprintf("%.1f", it.Amount)
				}
				if strings.Contains(keyUpper, "RATE") {
					itemMap["is_fee_rate"] = true
					itemMap["amount"] = -it.Amount
				}
				c.serviceFeeItems = append(c.serviceFeeItems, itemMap)
			} else if strings.Contains(keyUpper, "SETTLE") || strings.Contains(keyUpper, "NET") || it.IsFinal {
				itemMap["is_bold"] = true
				if isCredit {
					itemMap["is_negative"] = true
				}
				c.settlementItems = append(c.settlementItems, itemMap)
			} else {
				c.patientFeeItems = append(c.patientFeeItems, itemMap)
			}

		case sectionTypeUpper == "SFA_INVOICE" || sectionTypeUpper == "TAX_INVOICE":
			itemGst := 0.0
			itemSubtotal := it.Amount
			if basStr == "G1" {
				itemSubtotal = it.Amount / 1.1
				itemGst = it.Amount - itemSubtotal
			}
			rowClass := itemMap["row_class"]
			if it.IsFinal {
				rowClass = "row-final-balance"
			}
			c.taxInvoiceItems = append(c.taxInvoiceItems, map[string]interface{}{
				"description": fmt.Sprintf("<strong>%s</strong><br/>%s", it.Name, it.Description),
				"amount":      itemSubtotal,
				"gst":         itemGst,
				"row_class":   rowClass,
			})
			c.subtotal += itemSubtotal
			c.taxTotal += itemGst
			c.grandTotal += it.Amount

		case sectionTypeUpper == "REMITTANCE_INVOICE" || sectionTypeUpper == "REMITTANCE_ADVICE":
			itemMap["is_bold"] = it.IsFinal
			if isCredit {
				itemMap["is_negative"] = true
			}
			if it.IsFinal {
				itemMap["row_class"] = "row-final-balance"
			}
			c.remittanceItems = append(c.remittanceItems, itemMap)

		default:
		}
	}

	// Fallback: if no typed sections produced tax items, treat everything as tax items
	if len(c.taxInvoiceItems) == 0 {
		for _, it := range items {
			basStr := ""
			if it.BASCode != nil {
				basStr = *it.BASCode
			}
			itemGst := 0.0
			itemSubtotal := it.Amount
			if basStr == "G1" {
				itemSubtotal = it.Amount / 1.1
				itemGst = it.Amount - itemSubtotal
			}
			c.taxInvoiceItems = append(c.taxInvoiceItems, map[string]interface{}{
				"description": fmt.Sprintf("<strong>%s</strong><br/>%s", it.Name, it.Description),
				"amount":      itemSubtotal,
				"gst":         itemGst,
			})
			c.subtotal += itemSubtotal
			c.taxTotal += itemGst
			c.grandTotal += it.Amount
		}
	}

	return c, invoiceNumber, meta
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

// invoiceDataToMap converts the InvoiceData struct into a generic map
// while fully preserving the json structural tags expected by the HTML template engine.
func invoiceDataToMap(data InvoiceData) (map[string]interface{}, error) {
	// Step 1: Marshal the struct into JSON bytes.
	// This automatically maps fields according to your json struct tags (e.g., InvoiceNumber -> "invoice_number")
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoice data to json: %w", err)
	}

	// Step 2: Unmarshal back into a generic map
	var dataMap map[string]interface{}
	if err := json.Unmarshal(bytes, &dataMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json bytes into map: %w", err)
	}

	// Step 3: Explicitly map the direct styling properties just in case your HTML templates
	// look for CamelCase variables instead of snake_case.
	dataMap["InvoiceNumber"] = data.InvoiceNumber
	dataMap["ClinicName"] = data.ClinicName
	dataMap["IssueDateDisplay"] = data.IssueDateDisplay
	dataMap["DueDateDisplay"] = data.DueDateDisplay
	dataMap["BillingPeriod"] = data.BillingPeriod
	dataMap["InvoiceFrequency"] = data.InvoiceFrequency
	dataMap["ShowLogo"] = data.ShowLogo
	dataMap["ShowLogoImage"] = data.ShowLogoImage
	dataMap["LogoURL"] = data.LogoURL
	dataMap["LogoInitial"] = data.LogoInitial
	dataMap["WatermarkEnabled"] = data.WatermarkEnabled
	dataMap["WatermarkText"] = data.WatermarkText
	dataMap["ShowTax"] = data.ShowTax
	dataMap["LetterheadHTML"] = data.LetterheadHTML
	dataMap["FooterHTML"] = data.FooterHTML
	dataMap["Notes"] = data.Notes
	dataMap["GrandTotal"] = data.GrandTotal
	dataMap["Subtotal"] = data.Subtotal
	dataMap["TaxTotal"] = data.TaxTotal
	dataMap["PrimaryColor"] = data.PrimaryColor
	dataMap["AccentColor"] = data.AccentColor
	dataMap["BodyFontFamily"] = data.BodyFontFamily
	dataMap["HeaderFontFamily"] = data.HeaderFontFamily
	dataMap["CustomFeeRate"] = data.CustomFeeRate
	dataMap["TermsText"] = data.TermsText
	dataMap["TableStyleClass"] = data.TableStyleClass

	// Handle remittance attributes specifically matching the PDF text placeholders
	dataMap["CustomPaymentMethod"] = data.CustomPaymentMethod
	dataMap["PaymentMethodLabel"] = data.PaymentMethodLabel
	dataMap["CustomPaymentAccountName"] = data.CustomPaymentAccountName
	dataMap["CustomPaymentBsb"] = data.CustomPaymentBsb
	dataMap["CustomPaymentAccount"] = data.CustomPaymentAccount
	dataMap["PaymentDateDisplay"] = data.PaymentDateDisplay

	return dataMap, nil
}
