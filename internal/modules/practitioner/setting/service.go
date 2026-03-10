package setting

import (
	"context"
	"time"
)

type Service interface {
	CreateTentant(ctx context.Context, req *RqCreateTentant) (*RsTentant, error)
	GetTentant(ctx context.Context, id int) (*RsTentant, error)
	GetTentantByUserID(ctx context.Context, userID string) (*RsTentant, error)
	ListTentants(ctx context.Context) ([]*RsTentant, error)
	UpdateTentant(ctx context.Context, id int, req *RqUpdateTentant) (*RsTentant, error)
	DeleteTentant(ctx context.Context, id int) error

	GetSetting(ctx context.Context, tentantID int) (*RsTentantSetting, error)
	UpsertSetting(ctx context.Context, tentantID int, req *RqUpsertTentantSetting) (*RsTentantSetting, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateTentant(ctx context.Context, req *RqCreateTentant) (*RsTentant, error) {
	t := req.ToTentant()
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	return created.ToRs(), nil
}

func (s *service) GetTentant(ctx context.Context, id int) (*RsTentant, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) GetTentantByUserID(ctx context.Context, userID string) (*RsTentant, error) {
	t, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return t.ToRs(), nil
}

func (s *service) ListTentants(ctx context.Context) ([]*RsTentant, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*RsTentant, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) UpdateTentant(ctx context.Context, id int, req *RqUpdateTentant) (*RsTentant, error) {
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

func applyUpdate(t *Practitioner, req *RqUpdateTentant) {
	if req.ABN != nil {
		t.ABN = req.ABN
	}
	if req.Verifed != nil {
		t.Verifed = *req.Verifed
	}
	t.UpdatedAt = time.Now()
}

func (s *service) DeleteTentant(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) GetSetting(ctx context.Context, tentantID int) (*RsTentantSetting, error) {
	setting, err := s.repo.GetSettingByTentantID(ctx, tentantID)
	if err != nil {
		return nil, err
	}
	return setting.ToRs(), nil
}

func (s *service) UpsertSetting(ctx context.Context, tentantID int, req *RqUpsertTentantSetting) (*RsTentantSetting, error) {
	// Defaults
	timezone := "Australia/Sydney"
	color := "#000000"
	if req.Timezone != nil {
		timezone = *req.Timezone
	}
	if req.Color != nil {
		color = *req.Color
	}
	setting := &TentantSetting{
		TentantID: tentantID,
		Timezone:  timezone,
		Logo:      req.Logo,
		Color:     color,
		UpdatedAt: time.Now(),
	}
	updated, err := s.repo.UpsertSetting(ctx, setting)
	if err != nil {
		return nil, err
	}
	return updated.ToRs(), nil
}
