package service

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// ITemplateRepository defines template data access interface
type ITemplateRepository interface {
	Create(ctx context.Context, t *template.Template) error
	Update(ctx context.Context, t *template.Template) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*template.Template, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	BulkCreate(ctx context.Context, templates []template.Template) error
	ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error
}

// ITemplateService handles template CRUD operations
type ITemplateService interface {
	Create(ctx context.Context, rq template.RqGlobalTemplate) (*template.RsTemplate, error)
	Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*template.RsTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*template.RsTemplate, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	BulkCreate(ctx context.Context) (*[]template.RsTemplate, error)
	ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error
}

type TemplateService struct {
	repo       ITemplateRepository
	encryption IEncryptionService
	settingSvc ISettingService
}

func NewTemplateService(
	repo ITemplateRepository,
	encryption IEncryptionService,
	settingSvc ISettingService,
) ITemplateService {
	return &TemplateService{
		repo:       repo,
		encryption: encryption,
		settingSvc: settingSvc,
	}
}

func (s *TemplateService) Create(ctx context.Context, rq template.RqGlobalTemplate) (*template.RsTemplate, error) {
	// Encrypt content
	htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(rq.Html, rq.Css)
	if err != nil {
		return nil, err
	}

	// Build domain entity
	t := template.Template{
		Name:        rq.Name,
		Description: nil,
		Html:        htmlBlob,
		Css:         cssBlob,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}

	// Persist
	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, err
	}

	// Create default settings
	if err := s.settingSvc.CreateDefaultForTemplate(ctx, t.Id); err != nil {
		return nil, err
	}

	// Build response
	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(htmlBlob)
	rs.Css = base64.StdEncoding.EncodeToString(cssBlob)

	return &rs, nil
}

func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*template.RsTemplate, error) {
	// Encrypt content
	htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(rq.Html, rq.Css)
	if err != nil {
		return nil, err
	}

	t := template.Template{
		Id:        id,
		Name:      rq.Name,
		Html:      htmlBlob,
		Css:       cssBlob,
		IsDefault: rq.IsDefault,
		IsActive:  rq.IsActive,
	}

	if err := s.repo.Update(ctx, &t); err != nil {
		return nil, err
	}

	// Decrypt for response
	html, css, err := s.encryption.DecryptTemplate(htmlBlob, cssBlob)
	if err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString([]byte(html))
	rs.Css = base64.StdEncoding.EncodeToString([]byte(css))

	return &rs, nil
}

func (s *TemplateService) Get(ctx context.Context, id uuid.UUID) (*template.RsTemplate, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Decrypt content
	html, css, err := s.encryption.DecryptTemplate(t.Html, t.Css)
	if err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString([]byte(html))
	rs.Css = base64.StdEncoding.EncodeToString([]byte(css))

	return &rs, nil
}

func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *TemplateService) List(ctx context.Context, method string) (*util.RsList, error) {
	return s.repo.List(ctx, method)
}

func (s *TemplateService) BulkCreate(ctx context.Context) (*[]template.RsTemplate, error) {
	rqs := template.DefaultTemplates()
	templates := make([]template.Template, 0, len(rqs))

	for _, rq := range rqs {
		htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(
			string(freshRqHTMLFix(rq.Html)),
			string(freshRqHTMLFix(rq.Css)),
		)
		if err != nil {
			return nil, err
		}

		templates = append(templates, template.Template{
			Name:      rq.Name,
			Html:      htmlBlob,
			Css:       cssBlob,
			IsDefault: rq.IsDefault,
			IsActive:  rq.IsActive,
		})
	}

	if err := s.repo.BulkCreate(ctx, templates); err != nil {
		return nil, err
	}

	// Create settings for each template
	for _, t := range templates {
		if err := s.settingSvc.CreateDefaultForTemplate(ctx, t.Id); err != nil {
			return nil, err
		}
	}

	rs := make([]template.RsTemplate, 0, len(templates))
	for _, t := range templates {
		rsItem := t.ToRs()
		rsItem.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsItem.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs = append(rs, rsItem)
	}
	return &rs, nil
}

func (s *TemplateService) ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error {
	if len(templateIds) == 0 {
		return template.ErrTemplateRequired
	}

	const maxTemplates = 10
	if len(templateIds) > maxTemplates {
		return template.ErrTooManyTemplates
	}

	return s.repo.ValidateAccess(ctx, templateIds)
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
