package service

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template/repository"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// ITemplateService handles template CRUD operations
type ITemplate interface {
	Create(ctx context.Context, rq template.RqGlobalTemplate) (*common.RsTemplate, error)
	Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*common.RsTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*common.RsTemplate, error)
	List(ctx context.Context, method string) (*util.RsList, error)
	BulkCreate(ctx context.Context) (*[]common.RsTemplate, error)
	ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error
}

type Template struct {
	repo       repository.ITemplateRepository
	encryption IEncryptionService
	settingSvc ISetting
}

func NewTemplateService(repo repository.ITemplateRepository, encryption IEncryptionService, settingSvc ISetting) ITemplate {
	return &Template{
		repo:       repo,
		encryption: encryption,
		settingSvc: settingSvc,
	}
}

func (s *Template) Create(ctx context.Context, rq template.RqGlobalTemplate) (*common.RsTemplate, error) {
	// Encrypt content
	htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(rq.Html, rq.Css)
	if err != nil {
		return nil, err
	}

	// Build domain entity
	t := common.Template{
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

func (s *Template) Update(ctx context.Context, id uuid.UUID, rq template.RqGlobalTemplate) (*common.RsTemplate, error) {
	// Encrypt content
	htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(rq.Html, rq.Css)
	if err != nil {
		return nil, err
	}

	t := common.Template{
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

func (s *Template) Get(ctx context.Context, id uuid.UUID) (*common.RsTemplate, error) {
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

func (s *Template) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Template) List(ctx context.Context, method string) (*util.RsList, error) {
	return s.repo.List(ctx, method)
}

func (s *Template) BulkCreate(ctx context.Context) (*[]common.RsTemplate, error) {
	rqs := template.DefaultTemplates()
	templates := make([]common.Template, 0, len(rqs))

	for _, rq := range rqs {
		htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(
			string(freshRqHTMLFix(rq.Html)),
			string(freshRqHTMLFix(rq.Css)),
		)
		if err != nil {
			return nil, err
		}

		templates = append(templates, common.Template{
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

	rs := make([]common.RsTemplate, 0, len(templates))
	for _, t := range templates {
		rsItem := t.ToRs()
		rsItem.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsItem.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs = append(rs, rsItem)
	}
	return &rs, nil
}

func (s *Template) ValidateAccess(ctx context.Context, templateIds []uuid.UUID) error {
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
