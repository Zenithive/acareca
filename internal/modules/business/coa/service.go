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

	ListCharts(ctx context.Context) ([]RsChartOfAccount, error)
	GetChartByID(ctx context.Context, id uuid.UUID) (*RsChartOfAccount, error)
	CreateChart(ctx context.Context, req *RqCreateChartOfAccount) (*RsChartOfAccount, error)
	UpdateChart(ctx context.Context, id uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error)
	DeleteChart(ctx context.Context, id uuid.UUID) error
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

func (s *service) ListCharts(ctx context.Context) ([]RsChartOfAccount, error) {
	list, err := s.repo.ListCharts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RsChartOfAccount, len(list))
	for i := range list {
		out[i] = list[i].ToRs()
	}
	return out, nil
}

func (s *service) GetChartByID(ctx context.Context, id uuid.UUID) (*RsChartOfAccount, error) {
	c, err := s.repo.GetChartByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := c.ToRs()
	return &rs, nil
}

func (s *service) CreateChart(ctx context.Context, req *RqCreateChartOfAccount) (*RsChartOfAccount, error) {
	createdBy, err := uuid.Parse(req.CreatedBy)
	if err != nil {
		return nil, err
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
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	chart := &ChartOfAccount{
		CreatedBy:     createdBy,
		AccountTypeID: req.AccountTypeID,
		AccountTaxID:  req.AccountTaxID,
		Code:          req.Code,
		Name:          req.Name,
		Description:   req.Description,
		IsSystem:      isSystem,
		IsActive:      isActive,
	}
	created, err := s.repo.CreateChart(ctx, chart)
	if err != nil {
		return nil, err
	}
	rs := created.ToRs()
	return &rs, nil
}

func (s *service) UpdateChart(ctx context.Context, id uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error) {
	existing, err := s.repo.GetChartByID(ctx, id)
	if err != nil {
		return nil, err
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
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	updated, err := s.repo.UpdateChart(ctx, existing)
	if err != nil {
		return nil, err
	}
	rs := updated.ToRs()
	return &rs, nil
}

func (s *service) DeleteChart(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteChart(ctx, id)
}
