package coa

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	ListAccountTypes(ctx context.Context) ([]AccountType, error)
	GetAccountType(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context) ([]AccountTax, error)
	GetAccountTax(ctx context.Context, id int16) (*AccountTax, error)

	ListChartOfAccount(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error)
	GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*RsChartOfAccount, error)
	CreateChartOfAccount(ctx context.Context, practitionerID uuid.UUID, req *RqCreateChartOfAccountOfAccount) (*RsChartOfAccount, error)
	UpdateCharOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID, req *RqUpdateCharOfAccountOfAccount) (*RsChartOfAccount, error)
	DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error
}

type service struct {
	repo Repository
	db   *sqlx.DB
}

func NewService(repo Repository, db *sqlx.DB) Service {
	return &service{repo: repo, db: db}
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

func (s *service) GetAccountType(ctx context.Context, id int16) (*AccountType, error) {
	a, err := s.repo.GetAccountType(ctx, id)
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

func (s *service) GetAccountTax(ctx context.Context, id int16) (*AccountTax, error) {
	a, err := s.repo.GetAccountTax(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := a.ToRs()
	return &rs, nil
}

func (s *service) ListChartOfAccount(ctx context.Context, practitionerID uuid.UUID, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.ListChartOfAccount(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountChartOfAccount(ctx, practitionerID, ft)
	if err != nil {
		return nil, err
	}

	data := make([]RsChartOfAccount, 0, len(list))
	for _, item := range list {
		data = append(data, item.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(data, total, ft.Offset, ft.Limit)

	return &rsList, nil
}

func (s *service) GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*RsChartOfAccount, error) {
	c, err := s.repo.GetChartOfAccount(ctx, id, practitionerID)
	if err != nil {
		return nil, err
	}
	rs := c.ToRs()
	return &rs, nil
}

func (s *service) CreateChartOfAccount(ctx context.Context, practitionerID uuid.UUID, req *RqCreateChartOfAccountOfAccount) (*RsChartOfAccount, error) {
	// (code, practitionerID) must be unique per user
	existing, _ := s.repo.GetChartByCodeAndPractitionerID(ctx, req.Code, practitionerID, nil)
	if existing != nil {
		return nil, ErrCodeExists
	}
	if _, err := s.repo.GetAccountType(ctx, req.AccountTypeID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetAccountTax(ctx, req.AccountTaxID); err != nil {
		return nil, err
	}
	isSystem := false
	if req.IsSystem != nil {
		isSystem = *req.IsSystem
	}
	chart := &ChartOfAccount{
		PractitionerID: practitionerID,
		AccountTypeID:  req.AccountTypeID,
		AccountTaxID:   req.AccountTaxID,
		Code:           req.Code,
		Name:           req.Name,
		IsSystem:       isSystem,
	}
	var err error
	var created *ChartOfAccount
	util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		created, err = s.repo.CreateChartOfAccount(ctx, chart, tx)
		if err != nil {
			return err
		}
		return nil
	})
	rs := created.ToRs()
	return &rs, nil
}

func (s *service) UpdateCharOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID, req *RqUpdateCharOfAccountOfAccount) (*RsChartOfAccount, error) {
	existing, err := s.repo.GetChartOfAccount(ctx, id, practitionerID)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem {
		return nil, ErrSystemAccountProtected
	}
	if req.Code != nil && *req.Code != existing.Code {
		other, _ := s.repo.GetChartByCodeAndPractitionerID(ctx, *req.Code, practitionerID, &id)
		if other != nil {
			return nil, ErrCodeExists
		}
	}
	if req.AccountTypeID != nil {
		if _, err := s.repo.GetAccountType(ctx, *req.AccountTypeID); err != nil {
			return nil, err
		}
		existing.AccountTypeID = *req.AccountTypeID
	}
	if req.AccountTaxID != nil {
		if _, err := s.repo.GetAccountTax(ctx, *req.AccountTaxID); err != nil {
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
	updated, err := s.repo.UpdateCharOfAccount(ctx, existing)
	if err != nil {
		return nil, err
	}
	rs := updated.ToRs()
	return &rs, nil
}

func (s *service) DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error {
	existing, err := s.repo.GetChartOfAccount(ctx, id, practitionerID)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return ErrSystemAccountProtected
	}
	return s.repo.DeleteChartOfAccount(ctx, id, practitionerID)
}
