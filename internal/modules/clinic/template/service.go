package template

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IService interface {
	BulkCreate(ctx context.Context, clinicId uuid.UUID) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Update(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error
	Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error)
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

func (s *Service) List(ctx context.Context) (*util.RsList, error) {
	return s.repo.List(ctx)
}
