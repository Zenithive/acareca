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

// ChartOfAccount mirrors the precise schema of tbl_chart_of_accounts
type ChartOfAccount struct {
	ID              uuid.UUID  `db:"id"`
	PractitionerID  uuid.UUID  `db:"practitioner_id"`
	TemplateID      *uuid.UUID `db:"template_id"`
	IsCustom        bool       `db:"is_custom"`
	Key             string     `db:"key"`
	AccountTypeID   *int16     `db:"account_type_id"`
	AccountTypeName string     `db:"account_type_name"`
	IsTaxable       bool       `db:"is_taxable"`
	AccountTaxID    *int16     `db:"account_tax_id"`
	Code            *int16     `db:"code"`
	Name            *string    `db:"name"`
	IsSystem        *bool      `db:"is_system"`
	IsCos           *bool      `db:"is_cos"`
	IsCapital       *bool      `db:"is_capital"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       *time.Time `db:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at"`

	TemplateUUID          *uuid.UUID `db:"template_uuid"`
	TemplateAccountTypeID *int16     `db:"template_account_type_id"`
	TemplateAccountTaxID  *int16     `db:"template_account_tax_id"`
	TemplateCode          *int16     `db:"template_code"`
	TemplateName          *string    `db:"template_name"`
	TemplateIsSystem      *bool      `db:"template_is_system"`
	TemplateIsCos         *bool      `db:"template_is_cos"`
	TemplateIsCapital     *bool      `db:"template_is_capital"`
}

// ToRs safely extracts values by using local overrides if custom, or falling back to the template if not.
func (c *ChartOfAccount) ToRs() RsChartOfAccount {
	res := RsChartOfAccount{
		ID:             c.ID,
		PractitionerID: c.PractitionerID,
		IsCustom:       c.IsCustom,
		Key:            c.Key,
		IsTaxable:      c.IsTaxable,
		CreatedAt:      c.CreatedAt,
	}

	if c.UpdatedAt != nil {
		res.UpdatedAt = *c.UpdatedAt
	}

	// Dynamic Resolution Engine Strategy
	if c.IsCustom || c.TemplateUUID == nil {
		if c.AccountTypeID != nil {
			res.AccountTypeID = *c.AccountTypeID
		}
		res.AccountTypeName = c.AccountTypeName
		if c.AccountTaxID != nil {
			res.AccountTaxID = *c.AccountTaxID
		}
		if c.Code != nil {
			res.Code = *c.Code
		}
		if c.Name != nil {
			res.Name = *c.Name
		}
		if c.IsSystem != nil {
			res.IsSystem = *c.IsSystem
		}
		if c.IsCos != nil {
			res.IsCos = *c.IsCos
		}
		if c.IsCapital != nil {
			res.IsCapital = *c.IsCapital
		}
	} else {
		// Fallback: Pull directly from the flattened template fields mapped during SQL collection
		if c.TemplateAccountTypeID != nil {
			res.AccountTypeID = *c.TemplateAccountTypeID
		}
		res.AccountTypeName = c.AccountTypeName
		if c.TemplateAccountTaxID != nil {
			res.AccountTaxID = *c.TemplateAccountTaxID
		}
		if c.TemplateCode != nil {
			res.Code = *c.TemplateCode
		}
		if c.TemplateName != nil {
			res.Name = *c.TemplateName
		}
		if c.TemplateIsSystem != nil {
			res.IsSystem = *c.TemplateIsSystem
		}
		if c.TemplateIsCos != nil {
			res.IsCos = *c.TemplateIsCos
		}
		if c.TemplateIsCapital != nil {
			res.IsCapital = *c.TemplateIsCapital
		}
	}

	return res
}

type RqCreateChartOfAccount struct {
	PractitionerID uuid.UUID `json:"practitioner_id" validate:"required_if=Role Accountant"`
	AccountTypeID  int16     `json:"account_type_id" validate:"required,min=1"`
	AccountTaxID   int16     `json:"account_tax_id" validate:"required,min=1"`
	Code           int16     `json:"code" validate:"required,gte=100,lte=9999"`
	Name           string    `json:"name" validate:"required,max=255"`
	Key            string    `json:"key"`
	IsSystem       *bool     `json:"is_system"`
	IsCos          *bool     `json:"is_cos"`
	IsCapital      *bool     `json:"is_capital"`
}

type RqUpdateChartOfAccount struct {
	PractitionerID *uuid.UUID `json:"practitioner_id" validate:"required_if=Role Accountant"`
	AccountTypeID  *int16     `json:"account_type_id" validate:"omitempty,min=1"`
	AccountTaxID   *int16     `json:"account_tax_id" validate:"omitempty,min=1"`
	Code           *int16     `json:"code" validate:"omitempty,gte=100,lte=9999"`
	Name           *string    `json:"name" validate:"omitempty,max=255"`
	Key            *string    `json:"key"`
}

type RqCheckCodeUnique struct {
	PractitionerID uuid.UUID  `json:"practitioner_id" validate:"required_if=Role Accountant"`
	Code           int16      `json:"code" validate:"required,gte=100,lte=9999"`
	ExcludeID      *uuid.UUID `json:"exclude_id"`
}

type RsChartOfAccount struct {
	ID              uuid.UUID `json:"id"`
	PractitionerID  uuid.UUID `json:"practitioner_id"`
	IsCustom        bool      `json:"is_custom"`
	Key             string    `json:"key"`
	IsTaxable       bool      `json:"is_taxable"`
	AccountTypeID   int16     `json:"account_type_id"`
	AccountTypeName string    `json:"account_type_name"`
	AccountTaxID    int16     `json:"account_tax_id"`
	Code            int16     `json:"code"`
	Name            string    `json:"name"`
	IsSystem        bool      `json:"is_system"`
	IsCos           bool      `json:"is_cos"`
	IsCapital       bool      `json:"is_capital"`
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
	Key            *string     `form:"key"`
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
	if filter.Key != nil {
		filters["key"] = filter.Key
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

// COATemplate mirrors the schema of tbl_chart_of_accounts_template
type COATemplate struct {
	ID            *uuid.UUID `db:"id"`
	AccountTypeID *int16     `db:"account_type_id"`
	AccountTaxID  *int16     `db:"account_tax_id"`
	Code          *int16     `db:"code"`
	Name          *string    `db:"name"`
	Key           *string    `db:"key"`
	IsSystem      *bool      `db:"is_system"`
	IsCos         *bool      `db:"is_cos"`
	IsCapital     *bool      `db:"is_capital"`
	CreatedBy     *uuid.UUID `db:"created_by"`
	UpdatedBy     *uuid.UUID `db:"updated_by"`
	CreatedAt     *time.Time `db:"created_at"`
	UpdatedAt     *time.Time `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}
