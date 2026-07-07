package template

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// IService is the main service interface for template operations
type IService interface {
	BulkCreate(ctx context.Context) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqGlobalTemplate) (*RsTemplate, error)
	Update(ctx context.Context, id uuid.UUID, rq RqGlobalTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	GetInvoiceSetting(ctx context.Context, invoiceId uuid.UUID) (*common.RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*common.RsSetting, error)
	GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error)
	GenerateMultiPDF(ctx context.Context, templateIds []uuid.UUID, data common.Invoice) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateIds []uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
	BulkSyncDefaults(ctx context.Context) error
}
