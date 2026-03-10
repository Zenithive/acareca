package coa

import "context"

type Service interface {
	ListAccountTypes(ctx context.Context) ([]AccountType, error)
	GetAccountTypeByID(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]AccountTax, error)
	GetAccountTaxByID(ctx context.Context, id int16) (*AccountTax, error)
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
