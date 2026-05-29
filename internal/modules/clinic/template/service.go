package template

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type IService interface {
	BulkCreate(ctx context.Context, clincId uuid.UUID) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Update(ctx context.Context, rq RqTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error
	Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error)
}

type Service struct {
	repo IRepository
}

func NewService(repo IRepository) IService {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, rq RqTemplate) (*RsTemplate, error) {
	t := rq.ToDB()
	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, err
	}
	rs := t.ToRs()
	st := DefaultSettings(t.Id)
	if err := s.repo.CreateSetting(ctx, &st); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (s *Service) Update(ctx context.Context, rq RqTemplate) (*RsTemplate, error) {
	t := rq.ToDB()
	t.Id = rq.Id
	if err := s.repo.Update(ctx, &t); err != nil {
		return nil, err
	}
	rs := t.ToRs()
	return &rs, nil
}

func (s *Service) Delete(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) error {
	return s.repo.Delete(ctx, clinicId, id)
}

func (s *Service) Get(ctx context.Context, clinicId uuid.UUID, id uuid.UUID) (*RsTemplate, error) {
	t, err := s.repo.Get(ctx, clinicId, id)
	if err != nil {
		return nil, err
	}
	rs := t.ToRs()
	return &rs, nil
}

func (s *Service) List(ctx context.Context) (*util.RsList, error) {
	return s.repo.List(ctx)
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
		templates = append(templates, rq.ToDB())
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
		rs = append(rs, t.ToRs())
	}

	return &rs, nil
}
