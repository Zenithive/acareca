package setting

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	ListPractitioners(ctx context.Context) ([]*RsPractitioner, error)
	UpdatePractitioner(ctx context.Context, id uuid.UUID, req *RqUpdatePractitioner) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error

	GetSetting(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerSetting, error)
	UpsertSetting(ctx context.Context, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner) (*RsPractitioner, error) {
	t := req.ToPractitioner()
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	return created.ToRs(), nil
}

func (s *service) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	t, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) ListPractitioners(ctx context.Context) ([]*RsPractitioner, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*RsPractitioner, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) UpdatePractitioner(ctx context.Context, id uuid.UUID, req *RqUpdatePractitioner) (*RsPractitioner, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	applyUpdate(existing, req)
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}

func applyUpdate(t *Practitioner, req *RqUpdatePractitioner) {
	if req.ABN != nil {
		t.ABN = req.ABN
	}
	if req.Verified != nil {
		t.Verified = *req.Verified
	}
	t.UpdatedAt = time.Now()
}

func (s *service) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) GetSetting(ctx context.Context, practitionerID uuid.UUID) (*RsPractitionerSetting, error) {
	setting, err := s.repo.GetSettingByPractitionerID(ctx, practitionerID)
	if err != nil {
		return nil, err
	}
	return setting.ToRs(), nil
}

func (s *service) UpsertSetting(ctx context.Context, practitionerID uuid.UUID, req *RqUpsertPractitionerSetting) (*RsPractitionerSetting, error) {
	// Defaults
	timezone := "Australia/Sydney"
	color := "#000000"
	if req.Timezone != nil {
		timezone = *req.Timezone
	}
	if req.Color != nil {
		color = *req.Color
	}
	setting := &PractitionerSetting{
		PractitionerID: practitionerID,
		Timezone:       timezone,
		Logo:           req.Logo,
		Color:          color,
		UpdatedAt:      time.Now(),
	}
	updated, err := s.repo.UpsertSetting(ctx, setting)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}
