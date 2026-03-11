package coa

import (
	"time"

	"github.com/google/uuid"
)

type AccountType struct {
	ID          int16     `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type AccountTax struct {
	ID          int16     `db:"id"`
	Name        string    `db:"name"`
	Rate        float64   `db:"rate"`
	BASField    *string   `db:"bas_field"`
	IsTaxable   bool      `db:"is_taxable"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (a *AccountTax) ToRs() AccountTax {
	return AccountTax{
		ID:          a.ID,
		Name:        a.Name,
		Rate:        a.Rate,
		BASField:    a.BASField,
		IsTaxable:   a.IsTaxable,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}
func (a *AccountType) ToRs() AccountType {
	return AccountType{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

// Chart of Accounts (tbl_chart_of_accounts)

type ChartOfAccount struct {
	ID              uuid.UUID  `db:"id"`
	CreatedBy       uuid.UUID  `db:"created_by"`
	AccountTypeID   int16      `db:"account_type_id"`
	AccountTaxID    int16      `db:"account_tax_id"`
	Code            string     `db:"code"`
	Name            string     `db:"name"`
	Description     *string    `db:"description"`
	IsSystem        bool       `db:"is_system"`
	SystemProvider  bool       `db:"system_provider"` // true = default (e.g. on practitioner create), false = user-created
	IsActive        bool       `db:"is_active"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

type RsChartOfAccount struct {
	ID             uuid.UUID `json:"id"`
	CreatedBy      uuid.UUID `json:"created_by"`
	AccountTypeID  int16     `json:"account_type_id"`
	AccountTaxID   int16     `json:"account_tax_id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	IsSystem       bool      `json:"is_system"`
	SystemProvider bool      `json:"system_provider"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (c *ChartOfAccount) ToRs() RsChartOfAccount {
	return RsChartOfAccount{
		ID:             c.ID,
		CreatedBy:      c.CreatedBy,
		AccountTypeID:  c.AccountTypeID,
		AccountTaxID:   c.AccountTaxID,
		Code:           c.Code,
		Name:           c.Name,
		Description:    c.Description,
		IsSystem:       c.IsSystem,
		SystemProvider: c.SystemProvider,
		IsActive:       c.IsActive,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

type RqCreateChartOfAccount struct {
	CreatedBy     string  `json:"created_by" validate:"omitempty,uuid"` // optional; use path createdById when present
	AccountTypeID int16   `json:"account_type_id" validate:"required,min=1"`
	AccountTaxID  int16   `json:"account_tax_id" validate:"required,min=1"`
	Code          string  `json:"code" validate:"required,max=10"`
	Name          string  `json:"name" validate:"required,max=255"`
	Description   *string `json:"description"`
	IsSystem      *bool   `json:"is_system"`
	IsActive      *bool   `json:"is_active"`
}

type RqUpdateChartOfAccount struct {
	AccountTypeID *int16  `json:"account_type_id" validate:"omitempty,min=1"`
	AccountTaxID  *int16  `json:"account_tax_id" validate:"omitempty,min=1"`
	Code          *string `json:"code" validate:"omitempty,max=10"`
	Name          *string `json:"name" validate:"omitempty,max=255"`
	Description   *string `json:"description"`
	IsActive      *bool   `json:"is_active"`
}
