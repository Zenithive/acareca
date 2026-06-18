package coa

import (
	"time"

	"github.com/google/uuid"
)

type AccountTax string

const (
	GST_FREE_EXPENSES AccountTax = "GST Free Expenses"
	GST_ON_EXPENSES   AccountTax = "GST on Expenses"
	BAS_EXCLUDED      AccountTax = "BAS Excluded"
	GST_FREE_INCOME   AccountTax = "GST Free Income"
	GST_ON_INCOME     AccountTax = "GST on Income"
)

type AccountType string

const (
	EXPENSE     AccountType = "Expense"
	REVENUE     AccountType = "Revenue"
	DIRECT_COST AccountType = "Direct Cost"
	BANK        AccountType = "Bank"
	ASSET       AccountType = "Asset"
	LIABILITY   AccountType = "Liability"
	EQUITY      AccountType = "Equity"
	OTHER       AccountType = "Other - ITR Reporting Item"
)

type RqAccountTemplate struct {
	Code          int16  `json:"code"`
	Name          string `json:"name"`
	AccountTypeId int16  `json:"account_type_id"`
	AccountTaxId  int16  `json:"account_tax_id"`
	IsSystem      bool   `json:"is_system"`
	IsCos         bool   `json:"is_cos"`
	IsCapital     bool   `json:"is_capital"`

	CreatedBy uuid.UUID `json:"-"`
}

func (r *RqAccountTemplate) ToDB() AccountTemplate {
	return AccountTemplate{
		Code:          r.Code,
		Name:          r.Name,
		AccountTypeId: r.AccountTypeId,
		AccountTaxId:  r.AccountTaxId,
		IsSystem:      r.IsSystem,
		IsCos:         r.IsCos,
		IsCapital:     r.IsCapital,
		CreatedBy:     r.CreatedBy,
	}
}

type AccountTemplate struct {
	ID            uuid.UUID `db:"id"`
	Code          int16     `db:"code"`
	Name          string    `db:"name"`
	AccountTypeId int16     `db:"account_type_id"`
	AccountTaxId  int16     `db:"account_tax_id"`
	IsSystem      bool      `db:"is_system"`
	IsCos         bool      `db:"is_cos"`
	IsCapital     bool      `db:"is_capital"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`

	CreatedBy uuid.UUID  `db:"created_by"`
	UpdatedBy *uuid.UUID `db:"updated_by"`
}

func (a *AccountTemplate) ToResponse() RsAccountTemplate {
	return RsAccountTemplate{
		ID:            a.ID,
		Code:          a.Code,
		Name:          a.Name,
		AccountTypeId: a.AccountTypeId,
		AccountTaxId:  a.AccountTaxId,
		IsSystem:      a.IsSystem,
		IsCos:         a.IsCos,
		IsCapital:     a.IsCapital,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
		CreatedBy:     a.CreatedBy,
		UpdatedBy:     a.UpdatedBy,
	}
}

type RsAccountTemplate struct {
	ID            uuid.UUID `json:"id"`
	Code          int16     `json:"code"`
	Name          string    `json:"name"`
	AccountTypeId int16     `json:"account_type_id"`
	AccountTaxId  int16     `json:"account_tax_id"`
	IsSystem      bool      `json:"is_system"`
	IsCos         bool      `json:"is_cos"`
	IsCapital     bool      `json:"is_capital"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`

	CreatedBy uuid.UUID  `json:"created_by"`
	UpdatedBy *uuid.UUID `json:"updated_by"`
}

type RqUpdateAccountTemplate struct {
	ID            uuid.UUID `json:"id"`
	Name          *string   `json:"name,omitempty"`
	AccountTypeId *int16    `json:"account_type_id,omitempty"`
	AccountTaxId  *int16    `json:"account_tax_id,omitempty"`
	IsSystem      *bool     `json:"is_system,omitempty"`
	IsCos         *bool     `json:"is_cos,omitempty"`
	IsCapital     *bool     `json:"is_capital,omitempty"`

	UpdatedBy *uuid.UUID `json:"-"`
}

func (r *RqUpdateAccountTemplate) ApplyTo(account *AccountTemplate) {
	if r.Name != nil {
		account.Name = *r.Name
	}

	if r.AccountTypeId != nil {
		account.AccountTypeId = *r.AccountTypeId
	}

	if r.AccountTaxId != nil {
		account.AccountTaxId = *r.AccountTaxId
	}

	if r.IsSystem != nil {
		account.IsSystem = *r.IsSystem
	}

	if r.IsCos != nil {
		account.IsCos = *r.IsCos
	}

	if r.IsCapital != nil {
		account.IsCapital = *r.IsCapital
	}

	account.UpdatedBy = r.UpdatedBy
}

func ToResponses(accounts []AccountTemplate) []RsAccountTemplate {
	rs := make([]RsAccountTemplate, len(accounts))

	for i := range accounts {
		rs[i] = accounts[i].ToResponse()
	}

	return rs
}
