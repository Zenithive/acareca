package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/rendering"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type CompositeService struct {
	templateSvc ITemplateService
	settingSvc  ISettingService
	pdfSvc      IPDFService
	syncSvc     ISyncService
}

func NewCompositeService(cfg *config.Config, templateRepo repository.ITemplateRepository, settingRepo repository.ISettingRepository) *CompositeService {
	encryptionSvc := NewEncryptionService(cfg.TemplateEncryptionKey)

	var templRepo repository.ITemplateRepository = templateRepo
	var setRepo repository.ISettingRepository = settingRepo

	settingSvc := NewSettingService(setRepo)

	templateSvc := NewTemplateService(templRepo, encryptionSvc, settingSvc)

	renderer := rendering.NewChromeRenderer()
	builder := rendering.NewTemplateBuilder()
	pdfSvc := NewPDFService(templRepo, setRepo, encryptionSvc, renderer, builder, cfg)

	syncSvc := NewSyncService(templRepo, setRepo, encryptionSvc)

	return &CompositeService{
		templateSvc: templateSvc,
		settingSvc:  settingSvc,
		pdfSvc:      pdfSvc,
		syncSvc:     syncSvc,
	}
}

func (cs *CompositeService) BulkCreate(ctx context.Context) (*[]common.RsTemplate, error) {
	return cs.templateSvc.BulkCreate(ctx)
}

func (cs *CompositeService) Create(ctx context.Context, rq template.RqGlobalTemplate) (*common.RsTemplate, error) {
	return cs.templateSvc.Create(ctx, rq)
}

func (cs *CompositeService) Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*common.RsTemplate, error) {
	return cs.templateSvc.Update(ctx, id, rq)
}

func (cs *CompositeService) Delete(ctx context.Context, id uuid.UUID) error {
	return cs.templateSvc.Delete(ctx, id)
}

func (cs *CompositeService) Get(ctx context.Context, id uuid.UUID) (*common.RsTemplate, error) {
	return cs.templateSvc.Get(ctx, id)
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

func (cs *CompositeService) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateIds []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	return cs.pdfSvc.DownloadPDF(ctx, clinicId, templateIds, invoiceId)
}

func (cs *CompositeService) BulkSyncDefaults(ctx context.Context) error {
	return cs.syncSvc.BulkSyncDefaults(ctx)
}
