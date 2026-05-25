package coa

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

func GenerateKeyFromName(name string) string {
	cleaned := regexp.MustCompile(`[^a-zA-Z0-9\s]`).ReplaceAllString(name, "")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(strings.ToLower(cleaned))
	return strings.ReplaceAll(cleaned, " ", "_")
}

type AccountType struct {
	ID        int16     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (a *AccountType) ToRs() AccountType {
	return *a
}

type AccountTax struct {
	ID        int16     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Rate      float64   `db:"rate" json:"rate"`
	IsTaxable bool      `db:"is_taxable" json:"is_taxable"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (a *AccountTax) ToRs() AccountTax {
	return *a
}

type ChartOfAccount struct {
	ID              uuid.UUID  `db:"id"`
	PractitionerID  uuid.UUID  `db:"practitioner_id"`
	AccountTypeID   int16      `db:"account_type_id"`
	AccountTypeName string     `db:"account_type_name"`
	AccountTaxID    int16      `db:"account_tax_id"`
	Code            int16      `db:"code"`
	Name            string     `db:"name"`
	Key             string     `db:"key"`
	IsSystem        bool       `db:"is_system"`
	IsTaxable       bool       `db:"is_taxable"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`
}

func (c *ChartOfAccount) ToRs() RsChartOfAccount {
	return RsChartOfAccount{
		ID:              c.ID,
		PractitionerID:  c.PractitionerID,
		AccountTypeID:   c.AccountTypeID,
		AccountTypeName: c.AccountTypeName,
		AccountTaxID:    c.AccountTaxID,
		Code:            c.Code,
		Name:            c.Name,
		IsSystem:        c.IsSystem,
		IsTaxable:       c.IsTaxable,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

type RqCreateChartOfAccountOfAccount struct {
	PractitionerID uuid.UUID `json:"practitioner_id" validate:"required_if=Role Accountant"`
	AccountTypeID  int16     `json:"account_type_id" validate:"required,min=1"`
	AccountTaxID   int16     `json:"account_tax_id" validate:"required,min=1"`
	Code           int16     `json:"code" validate:"required,gte=100,lte=9999"`
	Name           string    `json:"name" validate:"required,max=255"`
	IsSystem       *bool     `json:"is_system"`
}

type RqUpdateCharOfAccountOfAccount struct {
	PractitionerID *uuid.UUID `json:"practitioner_id" validate:"required_if=Role Accountant"`
	AccountTypeID  *int16     `json:"account_type_id" validate:"omitempty,min=1"`
	AccountTaxID   *int16     `json:"account_tax_id" validate:"omitempty,min=1"`
	Code           *int16     `json:"code" validate:"omitempty,gte=100,lte=9999"`
	Name           *string    `json:"name" validate:"omitempty,max=255"`
}

type RqCheckCodeUnique struct {
	PractitionerID uuid.UUID  `json:"practitioner_id" validate:"required_if=Role Accountant"`
	Code           int16      `json:"code" validate:"required,gte=100,lte=9999"`
	ExcludeID      *uuid.UUID `json:"exclude_id"`
}

type RsChartOfAccount struct {
	ID              uuid.UUID `json:"id"`
	PractitionerID  uuid.UUID `json:"practitioner_id"`
	AccountTypeID   int16     `json:"account_type_id"`
	AccountTypeName string    `json:"account_type_name"`
	AccountTaxID    int16     `json:"account_tax_id"`
	Code            int16     `json:"code"`
	Name            string    `json:"name"`
	IsSystem        bool      `json:"is_system"`
	IsTaxable       bool      `json:"is_taxable"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RsCodeUnique struct {
	IsUnique bool `json:"is_unique"`
}

type RsChartOfAccountList struct {
	Data  []RsChartOfAccount `json:"data"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

type Filter struct {
	PractitionerID []uuid.UUID `form:"-"`
	Name           *string     `form:"name"`
	Id             *string     `form:"id"`
	Code           *int        `form:"code"`
	AccountType    *string     `form:"account_type"`
	ExcludeType    []string    `form:"exclude_type"`
	AccountTypeID  *int16      `form:"-"`
	ExcludeTypeIDs []int16     `form:"-"`
	AccountTaxID   *int16      `form:"account_tax_id"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.Id != nil {
		if id, err := uuid.Parse(*filter.Id); err == nil {
			filters["id"] = id
		}
	}
	if filter.Name != nil {
		filters["name"] = filter.Name
	}
	if filter.Code != nil {
		filters["code"] = filter.Code
	}
	if filter.AccountTypeID != nil {
		filters["account_type_id"] = filter.AccountTypeID
	}
	if filter.AccountTaxID != nil {
		filters["account_tax_id"] = filter.AccountTaxID
	}

	return common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}
