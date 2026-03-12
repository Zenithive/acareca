package coa

import (
	"time"

	"github.com/google/uuid"
)

type AccountType struct {
	ID        int16     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type AccountTax struct {
	ID        int16     `db:"id"`
	Name      string    `db:"name"`
	Rate      float64   `db:"rate"`
	IsTaxable bool      `db:"is_taxable"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (a *AccountTax) ToRs() AccountTax {
	return AccountTax{
		ID:        a.ID,
		Name:      a.Name,
		Rate:      a.Rate,
		IsTaxable: a.IsTaxable,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}
func (a *AccountType) ToRs() AccountType {
	return AccountType{
		ID:        a.ID,
		Name:      a.Name,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

type ChartOfAccount struct {
	ID             uuid.UUID  `db:"id"`
	PractitionerID uuid.UUID  `db:"practitioner_id"`
	AccountTypeID  int16      `db:"account_type_id"`
	AccountTaxID   int16      `db:"account_tax_id"`
	Code           int16      `db:"code"`
	Name           string     `db:"name"`
	IsSystem       bool       `db:"is_system"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

type RsChartOfAccount struct {
	ID             uuid.UUID `json:"id"`
	PractitionerID uuid.UUID `json:"practitioner_id"`
	AccountTypeID  int16     `json:"account_type_id"`
	AccountTaxID   int16     `json:"account_tax_id"`
	Code           int16     `json:"code"`
	Name           string    `json:"name"`
	IsSystem       bool      `json:"is_system"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (c *ChartOfAccount) ToRs() RsChartOfAccount {
	return RsChartOfAccount{
		ID:             c.ID,
		PractitionerID: c.PractitionerID,
		AccountTypeID:  c.AccountTypeID,
		AccountTaxID:   c.AccountTaxID,
		Code:           c.Code,
		Name:           c.Name,
		IsSystem:       c.IsSystem,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

type RqCreateChartOfAccountOfAccount struct {
	AccountTypeID int16  `json:"account_type_id" validate:"required,min=1"`
	AccountTaxID  int16  `json:"account_tax_id" validate:"required,min=1"`
	Code          int16  `json:"code" validate:"required,gte=100,lte=9999"`
	Name          string `json:"name" validate:"required,max=255"`
	IsSystem      *bool  `json:"is_system"`
}

type RqUpdateCharOfAccountOfAccount struct {
	AccountTypeID *int16  `json:"account_type_id" validate:"omitempty,min=1"`
	AccountTaxID  *int16  `json:"account_tax_id" validate:"omitempty,min=1"`
	Code          *int16  `json:"code" validate:"omitempty,gte=100,lte=9999"`
	Name          *string `json:"name" validate:"omitempty,max=255"`
}
