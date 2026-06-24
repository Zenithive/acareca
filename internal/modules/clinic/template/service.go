package template

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
		return nil, "", fmt.Errorf("failed to fetch invoice: %w", err)
	}

	sections, err := s.repo.GetInvoiceSectionMeta(ctx, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice section metadata: %w", err)
	}

	st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, templateId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice settings: %w", err)
	}

	if st != nil {
		if err := s.enrichSettingDocuments(ctx, st); err != nil {
			return nil, "", fmt.Errorf("failed enriching setting documents: %w", err)
		}
	}

	var htmlBuilder, cssBuilder strings.Builder
	for _, tId := range templateId {
		t, err := s.repo.Get(ctx, tId)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch template %s: %w", tId, err)
		}
		html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt html for template: %w", err)
		}
		css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt css for template: %w", err)
		}
		htmlBuilder.WriteString(html)
		htmlBuilder.WriteString("\n")
		cssBuilder.WriteString(css)
		cssBuilder.WriteString("\n")
	}

	collections, invoiceNumber, paymentMeta := buildInvoiceCollections(inv.Items, sections, inv.InvoiceNumber)

	data := InvoiceToData(inv)
	if invoiceNumber != "" {
		data.InvoiceNumber = invoiceNumber
	}

	// Dynamic Template Array Bindings
	data.PatientFeeItems = collections.patientFeeItems
	data.ServiceFeeItems = collections.serviceFeeItems
	data.SettlementItems = collections.settlementItems
	data.RemittanceItems = collections.remittanceItems

	// Dynamic Template Totals Mappings
	data.Subtotal = collections.subtotal
	data.TaxTotal = collections.taxTotal
	data.GrandTotal = collections.grandTotal
	data.CustomFeeRate = collections.customFeeRate
	data.CustomFeeRateDisplay = collections.customFeeRate + "%"

	// Remittance Metadata Assignment
	data.CustomPaymentMethod = paymentMeta.paymentMethod
	data.PaymentMethodLabel = paymentMeta.paymentMethod
	data.CustomPaymentAccountName = paymentMeta.accountName
	data.CustomPaymentBsb = paymentMeta.bsb
	data.CustomPaymentAccount = paymentMeta.accountNumber

	if paymentMeta.paymentDate != "" {
		if parsedTime, err := time.Parse("2006-01-02 15:04:05.999999-07", paymentMeta.paymentDate); err == nil {
			data.PaymentDateDisplay = parsedTime.Format("02 June 2026")
		} else if parsedTime, err := time.Parse("2006-01-02", paymentMeta.paymentDate); err == nil {
			data.PaymentDateDisplay = parsedTime.Format("02 June 2026")
		} else {
			data.PaymentDateDisplay = paymentMeta.paymentDate
		}
	} else {
		data.PaymentDateDisplay = data.IssueDateDisplay
	}

	primaryColor := "#1f4e5f"
	accentColor := "#1f4e5f"
	bodyFontFamily := "Arial"
	headerFontFamily := "Arial"
	watermarkText := "PAID"
	watermarkEnabled := false
	showTax := true
	termsText := ""
	isLogo := false

	if st != nil {
		primaryColor = st.PrimaryColor
		accentColor = st.AccentColor
		bodyFontFamily = st.BodyFontFamily
		headerFontFamily = st.HeaderFontFamily
		showTax = st.IsTax

		if st.TableStyle != nil {
			data.TableStyleClass = *st.TableStyle
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			watermarkEnabled = true
			watermarkText = *st.WaterMarkText
		}
		if st.TermText != nil {
			termsText = *st.TermText
			data.Notes = termsText
			data.TermsText = termsText
		}
		if st.IsLogo {
			isLogo = true
			data.ShowLogo = true
			if st.Logo != nil && st.Logo.ToRsDocument().FileKey != "" {
				data.ShowLogoImage = true
				data.LogoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + st.Logo.ToRsDocument().FileKey
			}
		}
	}

	data.TemplateSettings = map[string]interface{}{
		"primary_color":      primaryColor,
		"accent_color":       accentColor,
		"body_font_family":   bodyFontFamily,
		"header_font_family": headerFontFamily,
		"is_logo":            isLogo,
		"is_watermark":       watermarkEnabled,
		"watermark_text":     watermarkText,
		"is_tax":             showTax,
		"terms_text":         termsText,
	}

	dataMap, err := invoiceDataToMap(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed mapping template data: %w", err)
	}

	fullHTML, err := chromepdf.Render(htmlBuilder.String(), cssBuilder.String(), dataMap)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render HTML context: %w", err)
	}

	pdf, err := chromepdf.Generate(ctx, fullHTML)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate output PDF: %w", err)
	}

	return pdf, fmt.Sprintf("INVOICE_%s", data.InvoiceNumber), nil
}

// buildInvoiceCollections assigns items purely according to DB SectionType metadata matching the template rows
func buildInvoiceCollections(items []InvoiceItem, sections []InvoiceSectionMeta, fallbackInvoiceNumber string) (invoiceCollections, string, invoicePaymentMeta) {
	var c invoiceCollections
	c.customFeeRate = "0.0"
	var meta invoicePaymentMeta
	invoiceNumber := fallbackInvoiceNumber

	for _, sec := range sections {
		secTypeUpper := strings.TrimSpace(strings.ToUpper(sec.SectionType))
		if secTypeUpper == "SFA_INVOICE" && sec.DocumentNumber != "" {
			invoiceNumber = sec.DocumentNumber
		}
		if secTypeUpper == "REMITTANCE_INVOICE" || secTypeUpper == "REMITTANCE_ADVICE" {
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
		}
	}

	// Extract global configurations first
	for _, it := range items {
		var fk string
		if it.FieldKey != nil {
			fk = *it.FieldKey
		}
		if strings.TrimSpace(strings.ToUpper(fk)) == "FEE_RATE" {
			if it.Amount > 0.0 && it.Amount <= 1.0 {
				c.customFeeRate = fmt.Sprintf("%.1f", it.Amount*100)
			} else {
				c.customFeeRate = fmt.Sprintf("%.1f", it.Amount)
			}
			break
		}
	}

	// Section-by-section population logic mirroring the layout requirements
	for _, it := range items {
		basStr := ""
		if it.BASCode != nil {
			basStr = *it.BASCode
		}

		var fk string
		if it.FieldKey != nil {
			fk = *it.FieldKey
		}

		keyUpper := strings.TrimSpace(strings.ToUpper(fk))
		secTypeUpper := strings.TrimSpace(strings.ToUpper(it.SectionType))
		isDebit := strings.TrimSpace(strings.ToUpper(it.EntryType)) == "DEBIT"

		if keyUpper == "FEE_RATE" {

			feeRateDisplay := fmt.Sprintf("%.1f", it.Amount)

			if it.Amount <= 1 {
				feeRateDisplay = fmt.Sprintf("%.1f", it.Amount*100)
			}

			c.customFeeRate = feeRateDisplay

			c.serviceFeeRateIntro = map[string]interface{}{
				"label":            it.Name,
				"fee_rate_display": feeRateDisplay,
				"amount_display":   it.Amount,
			}

			continue
		}

		if keyUpper == "SERVICE_DESCRIPTION" {

			if strings.TrimSpace(it.Description) != "" {
				c.serviceDescriptionItems =
					append(c.serviceDescriptionItems, it.Description)
			}

			continue
		}

		row := map[string]interface{}{
			"label":       it.Name,
			"description": it.Description,
			"amount":      it.Amount,
			"bas_code":    basStr,
			"entry_type":  it.EntryType,
			"row_class":   "",
			"value_class": "",
			"is_bold":     it.IsFinal,
			"is_negative": false,
		}

		if it.IsFinal {
			row["row_class"] = "row-total"
			row["is_bold"] = true
		}

		switch secTypeUpper {
		case "CALCULATION_STATEMENT":
			// Assign rows purely to the targeted sections
			if keyUpper == "G1" || basStr == "G1" || strings.Contains(strings.ToUpper(it.Name), "INCOME") {
				c.patientFeeItems = append(c.patientFeeItems, row)
			} else if keyUpper == "1A" || basStr == "1A" || strings.Contains(strings.ToUpper(it.Name), "EXPENSE") || keyUpper == "G11" {
				c.serviceFeeItems = append(c.serviceFeeItems, row)
			} else {
				row["row_class"] = "row-final-balance"
				c.settlementItems = append(c.settlementItems, row)
			}

		case "SFA_INVOICE", "TAX_INVOICE":
			if it.IsFinal || keyUpper == "TOTAL" {
				c.grandTotal = it.Amount
				c.subtotal = it.Amount / 1.1
				c.taxTotal = it.Amount - c.subtotal
			}
			c.serviceFeeItems = append(c.serviceFeeItems, row)

		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			if isDebit || strings.Contains(strings.ToUpper(it.Name), "EXPENSE") || strings.Contains(strings.ToUpper(it.Name), "FEE") {
				row["is_negative"] = true
			}
			if it.IsFinal {
				row["row_class"] = "row-final-balance"
			}
			c.remittanceItems = append(c.remittanceItems, row)
		}
	}

	return c, invoiceNumber, meta
}

func invoiceDataToMap(data InvoiceData) (map[string]interface{}, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(bytes, &dataMap); err != nil {
		return nil, err
	}

	dataMap["invoice_number"] = data.InvoiceNumber
	dataMap["issue_date_display"] = data.IssueDateDisplay
	dataMap["billing_period"] = data.BillingPeriod
	dataMap["invoice_frequency"] = data.InvoiceFrequency
	dataMap["custom_fee_rate"] = data.CustomFeeRate
	dataMap["custom_fee_rate_display"] = data.CustomFeeRateDisplay
	dataMap["grand_total"] = data.GrandTotal
	dataMap["subtotal"] = data.Subtotal
	dataMap["tax_total"] = data.TaxTotal
	dataMap["notes"] = data.Notes
	dataMap["terms_text"] = data.TermsText

	dataMap["patient_fee_items"] = data.PatientFeeItems
	dataMap["service_fee_items"] = data.ServiceFeeItems
	dataMap["settlement_items"] = data.SettlementItems
	dataMap["remittance_items"] = data.RemittanceItems

	dataMap["custom_payment_method"] = data.CustomPaymentMethod
	dataMap["payment_method_label"] = data.PaymentMethodLabel
	dataMap["custom_payment_account_name"] = data.CustomPaymentAccountName
	dataMap["custom_payment_bsb"] = data.CustomPaymentBsb
	dataMap["custom_payment_account"] = data.CustomPaymentAccount
	dataMap["payment_date_display"] = data.PaymentDateDisplay

	return dataMap, nil
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
