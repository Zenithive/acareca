package template

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/chromepdf"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IService interface {
	BulkCreate(ctx context.Context, clinicId uuid.UUID) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Update(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error
	Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context, clinicId uuid.UUID) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error)

	GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
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

func (s *Service) Create(ctx context.Context, rq RqTemplate) (*RsTemplate, error) {
	// Process and encrypt string text templates into database byte structures
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
		ClinicId:    rq.ClinicId,
		Description: rq.Description,
		Html:        html,
		Css:         css,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}

	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, err
	}

	st := DefaultSettings(t.Id)
	if err := s.repo.CreateSetting(ctx, &st); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Update(ctx context.Context, rq RqTemplate) (*RsTemplate, error) {
	html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt layout stream: %w", err)
	}
	css, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt styling stream: %w", err)
	}

	t := Template{
		Id:          rq.Id,
		Name:        rq.Name,
		ClinicId:    rq.ClinicId,
		Description: rq.Description,
		Html:        html,
		Css:         css,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}

	if err := s.repo.Update(ctx, &t); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*RsTemplate, error) {
	t, err := s.repo.Get(ctx, clinicId, id)
	if err != nil {
		return nil, err
	}

	rs := t.ToRs()

	// Convert pre-encrypted binary byte blocks into clean Base64
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error) {
	st, err := s.repo.GetSetting(ctx, templateId)
	if err != nil {
		return nil, err
	}

	if st == nil {
		return nil, fmt.Errorf("template setting not found for template id: %s", templateId)
	}

	// Fetch document details if IDs are present
	if st.LogoId != nil {
		logo, err := s.repo.GetDocumentByID(ctx, *st.LogoId)
		if err != nil {
			return nil, err
		}
		st.Logo = logo
	}

	if st.LetterHeadId != nil {
		letterhead, err := s.repo.GetDocumentByID(ctx, *st.LetterHeadId)
		if err != nil {
			return nil, err
		}
		st.LetterHead = letterhead
	}

	if st.FooterId != nil {
		footer, err := s.repo.GetDocumentByID(ctx, *st.FooterId)
		if err != nil {
			return nil, err
		}
		st.Footer = footer
	}

	rs := st.ToRs()
	return &rs, nil
}

func (s *Service) UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error) {
	st := rq.ToDB()
	if err := s.repo.UpdateSetting(ctx, &st); err != nil {
		return nil, err
	}
	rs := st.ToRs()
	return &rs, nil
}

func (s *Service) BulkCreate(ctx context.Context, clinicId uuid.UUID) (*[]RsTemplate, error) {
	rqs := DefaultTemplates(clinicId)
	templates := make([]Template, 0, len(rqs))

	for _, rq := range rqs {
		html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
		if err != nil {
			return nil, err
		}
		cssBlob, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
		if err != nil {
			return nil, err
		}

		templates = append(templates, Template{
			Name:        rq.Name,
			ClinicId:    rq.ClinicId,
			Description: rq.Description,
			Html:        html,
			Css:         cssBlob,
			IsDefault:   rq.IsDefault,
			IsActive:    rq.IsActive,
		})
	}

	if err := s.repo.BulkCreate(ctx, templates); err != nil {
		return nil, err
	}

	for _, t := range templates {
		st := DefaultSettings(t.Id)
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

// Pass-through routines remain untouched
func (s *Service) Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error {
	return s.repo.Delete(ctx, clinicId, id)
}

func (s *Service) List(ctx context.Context, clinicId uuid.UUID) (*util.RsList, error) {
	return s.repo.List(ctx, clinicId)
}
func (s *Service) GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error) {
	t, err := s.repo.Get(ctx, rq.ClinicId, rq.TemplateId)
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

	// Pull settings so CSS vars (primary_color, fonts etc.) resolve correctly
	st, err := s.repo.GetSetting(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}
	if st != nil {
		rq.Data.PrimaryColor = st.PrimaryColor
		rq.Data.AccentColor = st.AccentColor
		rq.Data.BodyFontFamily = st.BodyFontFamily
		rq.Data.HeaderFontFamily = st.HeaderFontFamily
		if st.IsTax {
			rq.Data.ShowTax = true
		}
		if st.TableStyle != nil {
			rq.Data.TableStyleClass = *st.TableStyle
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			rq.Data.WatermarkEnabled = true
			rq.Data.WatermarkText = *st.WaterMarkText
		}
		// Apply terms text from settings when caller didn't supply notes
		if st.TermText != nil && rq.Data.Notes == "" {
			rq.Data.Notes = *st.TermText
		}
	}

	// Build the 3 calc sections — all values come from DefaultCalcConsts() + invoice totals.
	// Nothing from frontend or DB.
	rq.Data.CalcSections = buildCalcSections(
		rq.Data.GrandTotal,
		rq.Data.TaxTotal,
		rq.Data.Subtotal,
	)
	rq.Data.FooterNote = DefaultCalcConsts().FooterNote

	data := invoiceDataToMap(rq.Data)

	fullHTML, err := chromepdf.Render(html, css, data)
	if err != nil {
		return nil, err
	}

	return chromepdf.Generate(ctx, fullHTML)
}

func invoiceDataToMap(d InvoiceData) map[string]any {
	return map[string]any{
		"clinic_name":          d.ClinicName,
		"invoice_number":       d.InvoiceNumber,
		"issue_date_display":   d.IssueDateDisplay,
		"due_date_display":     d.DueDateDisplay,
		"reference":            d.Reference,
		"payment_method_label": d.PaymentMethodLabel,
		"tax_method_label":     d.TaxMethodLabel,
		"show_logo":            d.ShowLogo,
		"show_logo_image":      d.ShowLogoImage,
		"logo_url":             d.LogoURL,
		"logo_initial":         d.LogoInitial,
		"watermark_enabled":    d.WatermarkEnabled,
		"watermark_text":       d.WatermarkText,
		"show_tax":             d.ShowTax,
		"letterhead_html":      d.LetterheadHTML,
		"footer_html":          d.FooterHTML,
		"notes":                d.Notes,
		"amount_in_words":      d.AmountInWords,
		"has_attachments":      d.HasAttachments,
		"bill_from": map[string]any{
			"name": d.BillFrom.Name, "address": d.BillFrom.Address,
			"abn": d.BillFrom.ABN, "email": d.BillFrom.Email, "phone": d.BillFrom.Phone,
		},
		"bill_to": map[string]any{
			"name": d.BillTo.Name, "address": d.BillTo.Address,
			"abn": d.BillTo.ABN, "email": d.BillTo.Email, "phone": d.BillTo.Phone,
		},
		"items":                  lineItemsToMap(d.Items),
		"subtotal":               d.Subtotal,
		"tax_total":              d.TaxTotal,
		"discount_total":         d.DiscountTotal,
		"grand_total":            d.GrandTotal,
		"totals_amounts_caption": d.TotalsAmountsCaption,
		"totals_subtotal_label":  d.TotalsSubtotalLabel,
		"totals_tax_label":       d.TotalsTaxLabel,
		"totals_discount_label":  d.TotalsDiscountLabel,
		"totals_grand_label":     d.TotalsGrandLabel,
		"table_style_class":      d.TableStyleClass,
		"attachments":            attachmentsToMap(d.Attachments),
		"primary_color":          d.PrimaryColor,
		"accent_color":           d.AccentColor,
		"body_font_family":       d.BodyFontFamily,
		"header_font_family":     d.HeaderFontFamily,
		// Calc sections — built from DefaultCalcConsts + invoice totals
		"calc_sections": calcSectionsToMap(d.CalcSections),
		"footer_note":   d.FooterNote,
	}
}

// calcSectionsToMap converts []CalcSection → []map[string]any for raymond.
func calcSectionsToMap(sections []CalcSection) []map[string]any {
	out := make([]map[string]any, len(sections))
	for i, s := range sections {
		rows := make([]map[string]any, len(s.Rows))
		for j, r := range s.Rows {
			rows[j] = map[string]any{
				"label":       r.Label,
				"amount":      r.Amount,
				"bas_code":    r.BASCode,
				"is_bold":     r.IsBold,
				"is_blue":     r.IsBlue,
				"is_negative": r.IsNegative,
				"indent":      r.Indent,
				"fee_rate":    r.FeeRate,
			}
		}
		svcItems := make([]map[string]any, len(s.ServiceItems))
		for k, si := range s.ServiceItems {
			svcItems[k] = map[string]any{"label": si.Label}
		}
		out[i] = map[string]any{
			"number":          s.Number,
			"title":           s.Title,
			"show_bas_column": s.ShowBASColumn,
			"rows":            rows,
			"service_items":   svcItems,
		}
	}
	return out
}

// buildCalcSections constructs the 3 CalcSection values from DefaultCalcConsts
// (all labels, rates, bullets, and default amounts) + the real invoice monetary
// totals (grandTotal, taxTotal, subtotal).
//
// Value sources — nothing comes from frontend or DB:
//   grandTotal            ← invoice total incl. GST  (from InvoiceData)
//   taxTotal              ← GST collected             (from InvoiceData)
//   subtotal              ← sum of unit prices × qty  (from InvoiceData)
//   c.DefaultLabFees      ← constant default (DefaultCalcConsts)
//   c.DefaultRetainers    ← constant default (DefaultCalcConsts)
//   c.FeeRatePct          ← constant (DefaultCalcConsts)
//   c.GSTRatePct          ← constant (DefaultCalcConsts)
func buildCalcSections(grandTotal, taxTotal, subtotal float64) []CalcSection {
	c := DefaultCalcConsts()
	feeRateStr := fmt.Sprintf("%.1f%%", c.FeeRatePct)

	labFees := c.DefaultLabFees
	retainers := c.DefaultRetainers

	// ── Section 1: Patient Fees Collected ─────────────────────────
	// G3 (GST-free sales) = subtotal − (1A × 11)
	gstFreeSales := subtotal - (taxTotal * 11)
	if gstFreeSales < 0 {
		gstFreeSales = 0
	}
	// Net patient fees = G1 − 1A − lab fees
	netPatientFees := grandTotal - taxTotal - labFees

	section1 := CalcSection{
		Number:        "1",
		Title:         c.Sec1Title,
		ShowBASColumn: true,
		Rows: []CalcRow{
			{Label: c.Sec1TotalCollected, Amount: grandTotal, BASCode: "G1", IsBold: true, IsBlue: true},
			{Label: c.Sec1GSTCollected, Amount: taxTotal, BASCode: "1A", IsBlue: true},
			{Label: c.Sec1GSTFreeSales, Amount: gstFreeSales, BASCode: "G3"},
			{Label: c.Sec1LessLabFees, Amount: labFees, IsBlue: true},
			{Label: c.Sec1NetPatientFees, Amount: netPatientFees, IsBold: true},
		},
	}

	// ── Section 2: Service & Facility Fee ─────────────────────────
	serviceFeeNet := netPatientFees * (c.FeeRatePct / 100)
	gstOnServiceFee := serviceFeeNet * (c.GSTRatePct / 100)
	totalServiceFee := serviceFeeNet + gstOnServiceFee

	svcItems := make([]ServiceItem, len(c.ServiceItems))
	for i, label := range c.ServiceItems {
		svcItems[i] = ServiceItem{Label: label}
	}

	section2 := CalcSection{
		Number:        "2",
		Title:         c.Sec2Title,
		ShowBASColumn: true,
		Rows: []CalcRow{
			{Label: c.Sec2ServicesIntro, FeeRate: feeRateStr},
			{Label: c.Sec2ServiceFee, Amount: serviceFeeNet, IsBold: true},
			{Label: c.Sec2GSTOnServiceFee, Amount: gstOnServiceFee, BASCode: "1B"},
			{Label: c.Sec2TotalServiceFee, Amount: totalServiceFee, BASCode: "G11", IsBold: true},
		},
		ServiceItems: svcItems,
	}

	// ── Section 3: Net Settlement ──────────────────────────────────
	// Amount due = G1 − lab fees − total service fee (incl. GST)
	amountDue := grandTotal - labFees - totalServiceFee
	// Balance = amount due − retainers/drawings previously paid
	balance := amountDue - retainers

	section3 := CalcSection{
		Number:        "3",
		Title:         c.Sec3Title,
		ShowBASColumn: false,
		Rows: []CalcRow{
			{Label: c.Sec3TotalCollected, Amount: grandTotal},
			{Label: c.Sec3LessLabFees, Amount: labFees, IsNegative: true},
			{Label: c.Sec3LessServiceFee, Amount: totalServiceFee, IsNegative: true},
			{Label: c.Sec3AmountDue, Amount: amountDue, IsBold: true},
			{Label: c.Sec3LessRetainers, Amount: retainers, IsBlue: true, IsNegative: true},
			{Label: c.Sec3BalanceRemitted, Amount: balance, IsBold: true},
		},
	}

	return []CalcSection{section1, section2, section3}
}

func lineItemsToMap(items []LineItem) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, it := range items {
		out[i] = map[string]any{
			"name": it.Name, "description": it.Description,
			"unit_price": it.UnitPrice, "qty": it.Qty,
			"discount_amount": it.DiscountAmount,
			"tax_percent":     it.TaxPercent, "tax_amount": it.TaxAmount,
			"line_total": it.LineTotal,
		}
	}
	return out
}

func attachmentsToMap(items []Attachment) []map[string]any {
	out := make([]map[string]any, len(items))
	for i, a := range items {
		out[i] = map[string]any{"file_name": a.FileName}
	}
	return out
}

func (s *Service) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	// Fetch invoice
	inv, err := s.repo.GetInvoice(ctx, clinicId, invoiceId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch invoice: %w", err)
	}

	// Fetch template
	t, err := s.repo.Get(ctx, clinicId, templateId)
	if err != nil {
		return nil, "", err
	}

	html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt html: %w", err)
	}
	css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt css: %w", err)
	}

	// Fetch settings
	st, err := s.repo.GetSetting(ctx, templateId)
	if err != nil {
		return nil, "", err
	}

	// Map invoice → InvoiceData
	data := invoiceToData(inv)

	// Overlay settings
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
		}
		if st.IsLogo {
			data.ShowLogo = true
			if st.Logo != nil {
				data.ShowLogoImage = true
				data.LogoURL = s.cfg.R2StoragePrefix + st.Logo.ToRsDocument().FileKey
			} else if len(inv.ClinicName) > 0 {
				data.LogoInitial = string([]rune(inv.ClinicName)[0])
			}
		}
	}

	// Build the 3 calc sections — all values come from DefaultCalcConsts() + invoice totals.
	// Nothing from frontend or DB.
	data.CalcSections = buildCalcSections(
		data.GrandTotal,
		data.TaxTotal,
		data.Subtotal,
	)
	data.FooterNote = DefaultCalcConsts().FooterNote

	fullHTML, err := chromepdf.Render(html, css, invoiceDataToMap(data))
	if err != nil {
		return nil, "", err
	}

	pdf, err := chromepdf.Generate(ctx, fullHTML)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("%s-%s", inv.InvoiceNumber, inv.ClinicName)
	return pdf, filename, nil
}
