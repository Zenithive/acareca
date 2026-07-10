package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/auth"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/render"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

type CompositeService struct {
	templateSvc ITemplate
	settingSvc  ISetting
	pdfSvc      IPDF
	syncSvc     ISync
}

// NewCompositeServiceWithDB creates a CompositeService with full invoice method resolution.
// Pass db so that DownloadPDF can fetch the billing method from the invoice record.
func NewCompositeServiceWithDB(db *sqlx.DB, cfg *config.Config, templateRepo repository.ITemplateRepository, settingRepo repository.ISettingRepository, authRepo auth.Repository) *CompositeService {
	encryptionSvc := NewEncryptionService(cfg.TemplateEncryptionKey)

	settingSvc := NewSetting(settingRepo, authRepo)
	templateSvc := NewTemplateService(templateRepo, encryptionSvc, settingSvc)
	renderer := render.NewChromeRenderer()

	invoiceReader := newDBInvoiceReader(db)
	pdfSvc := NewPDFServiceWithInvoiceReader(templateRepo, settingRepo, invoiceReader, encryptionSvc, renderer, cfg)

	syncSvc := NewSyncService(templateRepo, settingRepo, encryptionSvc)

	return &CompositeService{
		templateSvc: templateSvc,
		settingSvc:  settingSvc,
		pdfSvc:      pdfSvc,
		syncSvc:     syncSvc,
	}
}

func (cs *CompositeService) BulkCreate(ctx context.Context) (*[]template.RsTemplate, error) {
	commonRs, err := cs.templateSvc.BulkCreate(ctx)
	if err != nil {
		return nil, err
	}
	return convertCommonRsTemplateSlice(commonRs), nil
}

func (cs *CompositeService) Create(ctx context.Context, rq template.RqGlobalTemplate) (*template.RsTemplate, error) {
	commonRs, err := cs.templateSvc.Create(ctx, rq)
	if err != nil {
		return nil, err
	}
	return convertCommonRsTemplate(commonRs), nil
}

func (cs *CompositeService) Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*template.RsTemplate, error) {
	commonRs, err := cs.templateSvc.Update(ctx, id, rq)
	if err != nil {
		return nil, err
	}
	return convertCommonRsTemplate(commonRs), nil
}

func (cs *CompositeService) Delete(ctx context.Context, id uuid.UUID) error {
	return cs.templateSvc.Delete(ctx, id)
}

func (cs *CompositeService) Get(ctx context.Context, id uuid.UUID) (*template.RsTemplate, error) {
	commonRs, err := cs.templateSvc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertCommonRsTemplate(commonRs), nil
}

func (cs *CompositeService) List(ctx context.Context, method string) (*util.RsList, error) {
	return cs.templateSvc.List(ctx, method)
}

func (cs *CompositeService) GetInvoiceSetting(ctx context.Context, invoiceId uuid.UUID) (*common.RsSetting, error) {
	return cs.settingSvc.Get(ctx, invoiceId)
}

func (cs *CompositeService) UpdateSetting(ctx context.Context, rq template.RqUpdateSetting) (*common.RsSetting, error) {
	return cs.settingSvc.Update(ctx, rq)
}

func (cs *CompositeService) GeneratePDF(ctx context.Context, rq template.RqGeneratePDF) ([]byte, error) {
	return cs.pdfSvc.GeneratePDF(ctx, rq)
}

func (cs *CompositeService) GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data common.Invoice) ([]byte, error) {
	return cs.pdfSvc.GenerateMultiPDF(ctx, templateIds, data)
}

func (cs *CompositeService) DownloadPDF(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	return cs.pdfSvc.DownloadPDF(ctx, clinicId, invoiceId)
}

func (cs *CompositeService) BulkSyncDefaults(ctx context.Context) error {
	return cs.syncSvc.BulkSyncDefaults(ctx)
}

// convertCommonRsTemplate converts common.RsTemplate to template.RsTemplate
func convertCommonRsTemplate(src *common.RsTemplate) *template.RsTemplate {
	if src == nil {
		return nil
	}
	return &template.RsTemplate{
		Id:          src.Id,
		Description: src.Description,
		Name:        src.Name,
		Html:        src.Html,
		Css:         src.Css,
		IsDefault:   src.IsDefault,
		IsActive:    src.IsActive,
		CreatedAt:   src.CreatedAt,
		UpdatedAt:   src.UpdatedAt,
	}
}

// convertCommonRsTemplateSlice converts a slice of common.RsTemplate to template.RsTemplate
func convertCommonRsTemplateSlice(src *[]common.RsTemplate) *[]template.RsTemplate {
	if src == nil {
		return nil
	}
	result := make([]template.RsTemplate, len(*src))
	for i, item := range *src {
		result[i] = template.RsTemplate{
			Id:          item.Id,
			Description: item.Description,
			Name:        item.Name,
			Html:        item.Html,
			Css:         item.Css,
			IsDefault:   item.IsDefault,
			IsActive:    item.IsActive,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
	}
	return &result
}
