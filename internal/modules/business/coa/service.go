package coa

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	ListAccountTypes(ctx context.Context) ([]AccountType, error)
	GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]AccountTax, error)
	GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error)

	ListChartsBypractice_id(ctx context.Context, practice_id uuid.UUID) ([]RsChartOfAccount, error)
	GetChartByIDAndpractice_id(ctx context.Context, id uuid.UUID, practice_id uuid.UUID) (*RsChartOfAccount, error)
	CreateChart(ctx context.Context, practice_id uuid.UUID, req *RqCreateChartOfAccount) (*RsChartOfAccount, error)
	UpdateChart(ctx context.Context, id uuid.UUID, practice_id uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error)
	DeleteChart(ctx context.Context, id uuid.UUID, practice_id uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListAccountTypes(ctx context.Context) ([]AccountType, error) {
	list, err := s.repo.ListAccountTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountType, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error) {
	a, err := s.repo.GetAccountTypeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := a.ToRs()
	return &rs, nil
}

func (s *service) ListAccountTaxes(ctx context.Context) ([]AccountTax, error) {
	list, err := s.repo.ListAccountTaxes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountTax, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error) {
	a, err := s.repo.GetAccountTaxByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := a.ToRs()
	return &rs, nil
}

func (s *service) ListChartsBypractice_id(ctx context.Context, practice_id uuid.UUID) ([]RsChartOfAccount, error) {
	list, err := s.repo.ListChartsBypractice_id(ctx, practice_id)
	if err != nil {
		return nil, err
	}
	out := make([]RsChartOfAccount, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) GetChartByIDAndpractice_id(ctx context.Context, id uuid.UUID, practice_id uuid.UUID) (*RsChartOfAccount, error) {
	c, err := s.repo.GetChartByIDAndpractice_id(ctx, id, practice_id)
	if err != nil {
		return nil, err
	}
	rs := c.ToRs()
	return &rs, nil
}

func (s *service) CreateChart(ctx context.Context, practice_id uuid.UUID, req *RqCreateChartOfAccount) (*RsChartOfAccount, error) {
	// (code, practice_id) must be unique per user
	existing, _ := s.repo.GetChartByCodeAndpractice_id(ctx, req.Code, practice_id, nil)
	if existing != nil {
		return nil, ErrCodeExists
	}
	if _, err := s.repo.GetAccountTypeByID(ctx, req.AccountTypeID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetAccountTaxByID(ctx, req.AccountTaxID); err != nil {
		return nil, err
	}
	isSystem := false
	if req.IsSystem != nil {
		isSystem = *req.IsSystem
	}
	chart := &ChartOfAccount{
		Practice_id:   practice_id,
		AccountTypeID: req.AccountTypeID,
		AccountTaxID:  req.AccountTaxID,
		Code:          req.Code,
		Name:          req.Name,
		IsSystem:      isSystem,
	}
	created, err := s.repo.CreateChart(ctx, chart)
	if err != nil {
		return nil, err
	}
	rs := created.ToRs()
	return &rs, nil
}

func (s *service) UpdateChart(ctx context.Context, id uuid.UUID, practice_id uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error) {
	existing, err := s.repo.GetChartByIDAndpractice_id(ctx, id, practice_id)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem {
		return nil, ErrSystemAccountProtected
	}
	if req.Code != nil && *req.Code != existing.Code {
		other, _ := s.repo.GetChartByCodeAndpractice_id(ctx, *req.Code, practice_id, &id)
		if other != nil {
			return nil, ErrCodeExists
		}
	}
	if req.AccountTypeID != nil {
		if _, err := s.repo.GetAccountTypeByID(ctx, *req.AccountTypeID); err != nil {
			return nil, err
		}
		existing.AccountTypeID = *req.AccountTypeID
	}
	if req.AccountTaxID != nil {
		if _, err := s.repo.GetAccountTaxByID(ctx, *req.AccountTaxID); err != nil {
			return nil, err
		}
		existing.AccountTaxID = *req.AccountTaxID
	}
	if req.Code != nil {
		existing.Code = *req.Code
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	updated, err := s.repo.UpdateChart(ctx, existing)
	if err != nil {
		return nil, err
	}
	rs := updated.ToRs()
	return &rs, nil
}

func (s *service) DeleteChart(ctx context.Context, id uuid.UUID, practice_id uuid.UUID) error {
	existing, err := s.repo.GetChartByIDAndpractice_id(ctx, id, practice_id)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemAccountProtected
	}
	return s.repo.DeleteChart(ctx, id, practice_id)
}
