package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// ISettingService handles settings management
type ISetting interface {
	Get(ctx context.Context, invoiceId uuid.UUID) (*common.RsSetting, error)
	Update(ctx context.Context, rq template.RqUpdateSetting) (*common.RsSetting, error)
	CreateDefaultForTemplate(ctx context.Context, templateId uuid.UUID) error
	// EnrichWithDocuments(ctx context.Context, st *common.Setting) error
}

type Setting struct {
	repo repository.ISettingRepository
}

func NewSetting(repo repository.ISettingRepository) ISetting {
	return &Setting{
		repo: repo,
	}
}

func (s *Setting) Get(ctx context.Context, invoiceId uuid.UUID) (*common.RsSetting, error) {
	st, err := s.repo.Get(ctx, invoiceId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invoice settings: %w", err)
	}

	if st == nil {
		fallbackSetting := template.DefaultSettings(uuid.New())
		rs := fallbackSetting.ToRs()
		return &rs, nil
	}

	// if err := s.EnrichWithDocuments(ctx, st); err != nil {
	// 	return nil, fmt.Errorf("failed enriching setting documents: %w", err)
	// }

	rs := st.ToRs()
	return &rs, nil
}

func (s *Setting) Update(ctx context.Context, rq template.RqUpdateSetting) (*common.RsSetting, error) {
	st := rq.ToDB()
	if err := s.repo.Update(ctx, &st, *rq.InvoiceId); err != nil {
		return nil, err
	}
	rs := st.ToRs()
	return &rs, nil
}

func (s *Setting) CreateDefaultForTemplate(ctx context.Context, templateId uuid.UUID) error {
	st := template.DefaultSettings(templateId)
	return s.repo.Create(ctx, &st)
}

// func (s *Setting) EnrichWithDocuments(ctx context.Context, st *common.Setting) error {
// 	if st.LogoId != nil {
// 		doc, err := s.docRepo.GetDocumentByID(ctx, *st.LogoId)
// 		if err == nil && doc != nil {
// 			st.Logo = doc
// 		}
// 	}
// 	return nil
// }
