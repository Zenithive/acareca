package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/render"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/rendering"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/pkg/chromepdf"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IPDF interface {
	GeneratePDF(ctx context.Context, rq template.RqGeneratePDF) ([]byte, error)
	GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data common.Invoice) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateIds []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
}

type PDFService struct {
	templateRepo repository.ITemplateRepository
	settingRepo  repository.ISettingRepository
	encryption   IEncryptionService
	renderer     render.IPDFRenderer
	dataMapper   *render.DataMapper
	cfg          *config.Config
}

func NewPDFService(templateRepo repository.ITemplateRepository, settingRepo repository.ISettingRepository, encryption IEncryptionService, renderer render.IPDFRenderer, cfg *config.Config) IPDF {
	return &PDFService{
		templateRepo: templateRepo,
		settingRepo:  settingRepo,
		encryption:   encryption,
		renderer:     renderer,
		dataMapper:   render.NewDataMapper(),
		cfg:          cfg,
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

	st, err := s.settingRepo.Get(ctx, uuid.Nil)
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
		if err := rendering.ValidateTemplateSize(html, css); err != nil {
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

func (s *PDFService) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateIds []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	if len(templateIds) == 0 {
		return nil, "", template.ErrTemplateRequired
	}

	// if len(templateIds) > template.MaxTemplateCount {
	// 	return nil, "", template.ErrTooManyTemplates
	// }

	if len(templateIds) > 0 {
		return nil, "", template.ErrTooManyTemplates
	}
	// Validate access
	if err := s.templateRepo.ValidateAccess(ctx, templateIds); err != nil {
		return nil, "", err
	}

	// This would load invoice and sections from repository
	// For now, return a placeholder implementation
	// The full implementation would be similar to the old DownloadPDF method

	filename := fmt.Sprintf("INVOICE_%s", invoiceId.String()[:8])

	// Placeholder: In full implementation, this would:
	// 1. Load invoice from repository
	// 2. Load sections
	// 3. Load settings
	// 4. Build invoice data
	// 5. Apply settings
	// 6. Render templates in order
	// 7. Generate PDF

	return nil, filename, fmt.Errorf("full implementation pending")
}

func (s *PDFService) applySettings(data *common.Invoice, st *common.Setting) {
	if st == nil {
		return
	}

	data.PrimaryColor = st.PrimaryColor
	data.AccentColor = st.AccentColor
	data.BodyFontFamily = st.BodyFontFamily
	data.HeaderFontFamily = st.HeaderFontFamily

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

	if st.IsLogo && st.Logo != nil {
		data.ShowLogo = true
		data.ShowLogoImage = true
		data.LogoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + st.Logo.ToRsDocument().FileKey
	}
}
