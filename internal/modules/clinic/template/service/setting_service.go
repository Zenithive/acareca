package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/file"
)

// ISettingRepository defines setting data access interface
type ISettingRepository interface {
	Get(ctx context.Context, invoiceId uuid.UUID) (*template.Setting, error)
	Create(ctx context.Context, st *template.Setting) error
	Update(ctx context.Context, st *template.Setting, invoiceId uuid.UUID) error
}

// IDocumentRepository defines document retrieval interface
type IDocumentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*file.Document, error)
}

// ISettingService handles settings management
type ISettingService interface {
	Get(ctx context.Context, invoiceId uuid.UUID) (*template.RsSetting, error)
	Update(ctx context.Context, rq template.RqUpdateSetting) (*template.RsSetting, error)
	CreateDefaultForTemplate(ctx context.Context, templateId uuid.UUID) error
	EnrichWithDocuments(ctx context.Context, st *template.Setting) error
}

type SettingService struct {
	repo    ISettingRepository
	docRepo IDocumentRepository
}

func NewSettingService(
	repo ISettingRepository,
	docRepo IDocumentRepository,
) ISettingService {
	return &SettingService{
		repo:    repo,
		docRepo: docRepo,
	}
}

func (s *SettingService) Get(ctx context.Context, invoiceId uuid.UUID) (*template.RsSetting, error) {
	st, err := s.repo.Get(ctx, invoiceId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invoice settings: %w", err)
	}

	// If no configuration exists, return default settings
	if st == nil {
		fallbackSetting := template.DefaultSettings(uuid.New())
		rs := fallbackSetting.ToRs()
		return &rs, nil
	}

	// Enrich with documents (logo)
	if err := s.EnrichWithDocuments(ctx, st); err != nil {
		return nil, fmt.Errorf("failed enriching setting documents: %w", err)
	}

	rs := st.ToRs()
	return &rs, nil
}

func (s *SettingService) Update(ctx context.Context, rq template.RqUpdateSetting) (*template.RsSetting, error) {
	st := rq.ToDB()
	if err := s.repo.Update(ctx, &st, *rq.InvoiceId); err != nil {
		return nil, err
	}
	rs := st.ToRs()
	return &rs, nil
}

func (s *SettingService) CreateDefaultForTemplate(ctx context.Context, templateId uuid.UUID) error {
	st := template.DefaultSettings(templateId)
	return s.repo.Create(ctx, &st)
}

func (s *SettingService) EnrichWithDocuments(ctx context.Context, st *template.Setting) error {
	if st.LogoId != nil {
		doc, err := s.docRepo.GetByID(ctx, *st.LogoId)
		if err == nil && doc != nil {
			st.Logo = doc
		}
	}
	return nil
}
