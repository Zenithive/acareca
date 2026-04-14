package coa

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// GenerateKeyFromName converts a name to a key format (e.g., "Commission Fee" -> "commission_fee")
func GenerateKeyFromName(name string) string {
	// Remove special characters except spaces
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	cleaned := reg.ReplaceAllString(name, "")

	// Replace multiple spaces with single space
	spaceReg := regexp.MustCompile(`\s+`)
	cleaned = spaceReg.ReplaceAllString(cleaned, " ")

	// Trim and convert to lowercase
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.ToLower(cleaned)

	// Replace spaces with underscores
	key := strings.ReplaceAll(cleaned, " ", "_")

	return key
}

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
	Key            string     `db:"key"`
	IsSystem       bool       `db:"is_system"`
	IsTaxable      bool       `db:"is_taxable"`
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

	IsSystem  bool      `json:"is_system"`
	IsTaxable bool      `json:"is_taxable"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
		IsTaxable:      c.IsTaxable,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
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

type RsCodeUnique struct {
	IsUnique bool `json:"is_unique"`
}

type RqCheckCodeUnique struct {
	PractitionerID uuid.UUID  `json:"practitioner_id" validate:"required_if=Role Accountant"`
	Code           int16      `json:"code" validate:"required,gte=100,lte=9999"`
	ExcludeID      *uuid.UUID `json:"exclude_id"`
}

type RqUpdateCharOfAccountOfAccount struct {
	PractitionerID *uuid.UUID `json:"practitioner_id" validate:"required_if=Role Accountant"`
	AccountTypeID  *int16     `json:"account_type_id" validate:"omitempty,min=1"`
	AccountTaxID   *int16     `json:"account_tax_id" validate:"omitempty,min=1"`
	Code           *int16     `json:"code" validate:"omitempty,gte=100,lte=9999"`
	Name           *string    `json:"name" validate:"omitempty,max=255"`
}

// RsChartOfAccountList is the paginated list response for chart of accounts.
type RsChartOfAccountList struct {
	Data  []RsChartOfAccount `json:"data"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

type Filter struct {
	PractitionerID *uuid.UUID `form:"-"`
	Name           *string    `form:"name"`
	Id             *string    `form:"id"`
	Code           *int       `form:"code"`
	AccountType    *string    `form:"account_type"`
	AccountTypeID  *int16     `form:"-"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	if filter.Id != nil {
		id, err := uuid.Parse(*filter.Id)
		if err != nil {
			fmt.Println("invalid clinic_id: %w", err)
		}
		filters["id"] = uuid.UUID(id)
	}
	if filter.Name != nil {
		filters["name"] = *filter.Name
	}
	if filter.Code != nil {
		filters["code"] = *filter.Code
	}

	if filter.AccountTypeID != nil {
		filters["account_type_id"] = *filter.AccountTypeID
	}

	f := common.NewFilter(filter.Search, filters, nil, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)

	return f
}
