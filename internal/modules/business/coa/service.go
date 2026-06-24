package coa

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type Service interface {
	ListAccountTypes(ctx context.Context, f *Filter) (*util.RsList, error)
	GetAccountType(ctx context.Context, id int16) (*AccountType, error)
	ListAccountTaxes(ctx context.Context, f *Filter) (*util.RsList, error)
	GetAccountTax(ctx context.Context, id int16) (*AccountTax, error)
	ListChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f *Filter) (*util.RsList, error)
	GetChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*RsChartOfAccount, error)
	GetChartOfAccountByKey(ctx context.Context, key string, actorID uuid.UUID, role string) (*RsChartOfAccount, error)
	CheckCodeUnique(ctx context.Context, practitionerID uuid.UUID, code int16, excludeID *uuid.UUID) (*RsCodeUnique, error)
	CreateChartOfAccount(ctx context.Context, practitionerID uuid.UUID, req *RqCreateChartOfAccount) (*RsChartOfAccount, error)
	UpdateCharOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error)
	DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error
	GetByIDInternal(ctx context.Context, id uuid.UUID) (*RsChartOfAccount, error)
}

type service struct {
	repo     Repository
	db       *sqlx.DB
	auditSvc audit.Service
}

func NewService(repo Repository, db *sqlx.DB, auditSvc audit.Service) Service {
	return &service{repo: repo, db: db, auditSvc: auditSvc}
}

func (s *service) ListAccountTypes(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.ListAccountTypes(ctx, ft)
	if err != nil {
		return nil, err
	}
	data := make([]AccountType, len(list))
	for i := range list {
		data[i] = list[i].ToRs()
	}

	var rsList util.RsList
	rsList.MapToList(data, len(data), *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) GetAccountType(ctx context.Context, id int16) (*AccountType, error) {
	a, err := s.repo.GetAccountType(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := a.ToRs()
	return &rs, nil
}

func (s *service) ListAccountTaxes(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()
	list, err := s.repo.ListAccountTaxes(ctx, ft)
	if err != nil {
		return nil, err
	}
	data := make([]AccountTax, len(list))
	for i := range list {
		data[i] = list[i].ToRs()
	}

	var rsList util.RsList
	rsList.MapToList(data, len(data), *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *service) GetAccountTax(ctx context.Context, id int16) (*AccountTax, error) {
	a, err := s.repo.GetAccountTax(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := a.ToRs()
	return &rs, nil
}

func (s *service) ListChartOfAccount(ctx context.Context, actorID *uuid.UUID, role string, f *Filter) (*util.RsList, error) {
	if f.AccountType != nil && *f.AccountType != "" {
		id, err := s.repo.GetAccountTypeByName(ctx, *f.AccountType)
		if err != nil {
			return nil, err
		}
		typeID := int16(id)
		f.AccountTypeID = &typeID
	}

	for _, rawExclude := range f.ExcludeType {
		if rawExclude == "" {
			continue
		}
		for _, part := range strings.Split(rawExclude, ",") {
			trimmedPart := strings.TrimSpace(part)
			if trimmedPart == "" {
				continue
			}
			excludeID, err := s.repo.GetAccountTypeByName(ctx, trimmedPart)
			if err != nil {
				return nil, err
			}
			f.ExcludeTypeIDs = append(f.ExcludeTypeIDs, int16(excludeID))
		}
	}

	ft := f.MapToFilter()

	if f.AccountTypeID != nil {
		ft.Where = append(ft.Where, common.Condition{
			Field:    "account_type_id",
			Operator: common.OpEq,
			Value:    *f.AccountTypeID,
		})
	}

	for _, targetExcludeID := range f.ExcludeTypeIDs {
		ft.Where = append(ft.Where, common.Condition{
			Field:    "account_type_id",
			Operator: common.OpNotEq,
			Value:    targetExcludeID,
		})
	}

	switch role {
	case util.RolePractitioner:
		ft.Where = append(ft.Where, common.Condition{
			Field:    "practitioner_id",
			Operator: common.OpEq,
			Value:    actorID,
		})
	case util.RoleAccountant:
		if len(f.PractitionerID) > 0 {
			ft.Where = append(ft.Where, common.Condition{
				Field:    "practitioner_id",
				Operator: common.OpIn,
				Value:    f.PractitionerID,
			})
		}
	}

	list, err := s.repo.ListChartOfAccount(ctx, actorID, role, ft)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountChartOfAccount(ctx, actorID, role, ft)
	if err != nil {
		return nil, err
	}

	data := make([]RsChartOfAccount, 0, len(list))
	for _, item := range list {
		data = append(data, item.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(data, total, *ft.Offset, *ft.Limit)
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

func (s *service) GetChartOfAccountByKey(ctx context.Context, key string, actorID uuid.UUID, role string) (*RsChartOfAccount, error) {
	targetID := actorID
	if role == util.RoleAccountant {
		targetID = uuid.Nil
	}

	c, err := s.repo.GetChartOfAccountByKey(ctx, key, targetID)
	if err != nil {
		return nil, err
	}
	rs := c.ToRs()
	return &rs, nil
}

func (s *service) CreateChartOfAccount(ctx context.Context, practitionerID uuid.UUID, req *RqCreateChartOfAccount) (*RsChartOfAccount, error) {
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
	var isCos, isCapital bool
	if req.IsCos != nil {
		isCos = *req.IsCos
	}
	if req.IsCapital != nil {
		isCapital = *req.IsCapital
	}

	accountKey := req.Key
	if accountKey == "" {
		accountKey = GenerateKeyFromName(req.Name)
	}

	chart := &ChartOfAccount{
		PractitionerID: practitionerID,
		TemplateID:     nil,
		IsCustom:       true,
		Key:            accountKey,
		AccountTypeID:  &req.AccountTypeID,
		AccountTaxID:   &req.AccountTaxID,
		Code:           &req.Code,
		Name:           &req.Name,
		IsSystem:       &isSystem,
		IsCos:          &isCos,
		IsCapital:      &isCapital,
	}

	var created *ChartOfAccount
	err := util.RunInTransaction(ctx, s.db, func(txCtx context.Context, tx *sqlx.Tx) error {
		var txErr error
		created, txErr = s.repo.CreateChartOfAccount(txCtx, chart, tx)
		return txErr
	})
	if err != nil {
		return nil, err
	}

	rs := created.ToRs()
	idStr := created.ID.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionCOACreated,
		Module:     auditctx.ModuleBusiness,
		EntityType: lo.ToPtr(auditctx.EntityCOA),
		EntityID:   &idStr,
		AfterState: rs,
	})

	return &rs, nil
}

func (s *service) UpdateCharOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID, req *RqUpdateChartOfAccount) (*RsChartOfAccount, error) {
	existing, err := s.repo.GetChartOfAccount(ctx, id, practitionerID)
	if err != nil {
		return nil, err
	}

	resolvedRes := existing.ToRs()
	if resolvedRes.IsSystem {
		return nil, ErrSystemAccountProtected
	}

	if req.Code != nil && *req.Code != resolvedRes.Code {
		other, _ := s.repo.GetChartByCodeAndPractitionerID(ctx, *req.Code, practitionerID, &id)
		if other != nil {
			return nil, ErrCodeExists
		}
	}

	if !existing.IsCustom {
		existing.IsCustom = true
	}

	if req.AccountTypeID != nil {
		if _, err := s.repo.GetAccountType(ctx, *req.AccountTypeID); err != nil {
			return nil, err
		}
		existing.AccountTypeID = req.AccountTypeID
	}

	if req.AccountTaxID != nil {
		if _, err := s.repo.GetAccountTax(ctx, *req.AccountTaxID); err != nil {
			return nil, err
		}
		existing.AccountTaxID = req.AccountTaxID
	}

	if req.Code != nil {
		existing.Code = req.Code
	}

	if req.Name != nil {
		existing.Name = req.Name
		existing.Key = GenerateKeyFromName(*req.Name)
	}

	if req.Key != nil && *req.Key != "" {
		existing.Key = *req.Key
	}

	updated, err := s.repo.UpdateCharOfAccount(ctx, existing)
	if err != nil {
		return nil, err
	}

	bytes := updated.ToRs()
	idStr := id.String()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionCOAUpdated,
		Module:     auditctx.ModuleBusiness,
		EntityType: lo.ToPtr(auditctx.EntityCOA),
		EntityID:   &idStr,
		AfterState: bytes,
	})

	return &bytes, nil
}

func (s *service) DeleteChartOfAccount(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) error {
	existing, err := s.repo.GetChartOfAccount(ctx, id, practitionerID)
	if err != nil {
		return err
	}
	resolvedRes := existing.ToRs()
	if resolvedRes.IsSystem {
		return ErrSystemAccountProtected
	}
	if err := s.repo.DeleteChartOfAccount(ctx, id, practitionerID); err != nil {
		return err
	}

	idStr := id.String()
	rs := existing.ToRs()

	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionCOADeleted,
		Module:      auditctx.ModuleBusiness,
		EntityType:  lo.ToPtr(auditctx.EntityCOA),
		EntityID:    &idStr,
		BeforeState: rs,
	})

	return nil
}

func (s *service) CheckCodeUnique(ctx context.Context, practitionerID uuid.UUID, code int16, excludeID *uuid.UUID) (*RsCodeUnique, error) {
	existing, _ := s.repo.GetChartByCodeAndPractitionerID(ctx, code, practitionerID, excludeID)
	return &RsCodeUnique{IsUnique: existing == nil}, nil
}

func (s *service) GetByIDInternal(ctx context.Context, id uuid.UUID) (*RsChartOfAccount, error) {
	c, err := s.repo.GetByIDInternal(ctx, id)
	if err != nil {
		return nil, err
	}
	rs := c.ToRs()
	return &rs, nil
}
