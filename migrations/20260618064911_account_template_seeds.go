package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAccountTemplateSeeds, downAccountTemplateSeeds)
}

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

type AccountTemplate struct {
	Code            int16
	Name            string
	AccountTypeName AccountType
	AccountTaxName  AccountTax
	IsSystem        bool
	IsCos           bool
	IsCapital       bool
}

func GenerateKeyFromName(name string) string {
	cleaned := regexp.MustCompile(`[^a-zA-Z0-9\s]`).ReplaceAllString(name, "")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(strings.ToLower(cleaned))
	return strings.ReplaceAll(cleaned, " ", "_")
}

func InsertTemplate(ctx context.Context, tx *sql.Tx, account []AccountTemplate) error {
	for _, acc := range account {

		var accountTypeID int16
		err := tx.QueryRowContext(
			ctx,
			`SELECT id FROM tbl_account_type WHERE name = $1`,
			acc.AccountTypeName,
		).Scan(&accountTypeID)
		if err != nil {
			return fmt.Errorf("account_type lookup failed for code=%d name=%q account_type=%q: %w",
				acc.Code, acc.Name, acc.AccountTypeName, err)
		}

		var accountTaxID int16
		err = tx.QueryRowContext(
			ctx,
			`SELECT id FROM tbl_account_tax WHERE name = $1`,
			acc.AccountTaxName,
		).Scan(&accountTaxID)
		if err != nil {
			return fmt.Errorf("account_tax lookup failed for code=%d name=%q account_tax=%q: %w",
				acc.Code, acc.Name, acc.AccountTaxName, err)
		}

		generatedKey := GenerateKeyFromName(acc.Name)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO tbl_chart_of_accounts_template (
				id,
				account_type_id,
				account_tax_id,
				code,
				key,
				name,
				is_system,
				is_cos,
				is_capital
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (code) DO NOTHING
		`,
			uuid.New(),
			accountTypeID,
			accountTaxID,
			acc.Code,
			generatedKey,
			acc.Name,
			acc.IsSystem,
			acc.IsCos,
			acc.IsCapital,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func upAccountTemplateSeeds(ctx context.Context, tx *sql.Tx) error {
	var COA = []AccountTemplate{
		{
			Code:            200,
			Name:            "Patient Fee Account (GST Free)",
			AccountTypeName: REVENUE,
			AccountTaxName:  GST_FREE_INCOME,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            201,
			Name:            "Patient Fee Account (GST)",
			AccountTypeName: REVENUE,
			AccountTaxName:  GST_ON_INCOME,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            202,
			Name:            "Commission Received",
			AccountTypeName: REVENUE,
			AccountTaxName:  GST_ON_INCOME,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            203,
			Name:            "Other Income",
			AccountTypeName: REVENUE,
			AccountTaxName:  GST_ON_INCOME,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            204,
			Name:            "Interest Income",
			AccountTypeName: REVENUE,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            310,
			Name:            "Laboratory Work (GST Free)",
			AccountTypeName: DIRECT_COST,
			AccountTaxName:  GST_FREE_EXPENSES,
			IsSystem:        false,
			IsCos:           true,
			IsCapital:       false,
		},
		{
			Code:            311,
			Name:            "Laboratory Work (GST)",
			AccountTypeName: DIRECT_COST,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           true,
			IsCapital:       false,
		},
		{
			Code:            312,
			Name:            "Management Fee (Gross Up)",
			AccountTypeName: DIRECT_COST,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           true,
			IsCapital:       false,
		},
		{
			Code:            313,
			Name:            "Materials/Dental Supplies",
			AccountTypeName: DIRECT_COST,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           true,
			IsCapital:       false,
		},
		{
			Code:            400,
			Name:            "Accounting Fees",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            401,
			Name:            "Audit Insurance",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            402,
			Name:            "Bank Fees",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_FREE_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            403,
			Name:            "Business Insurance",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            404,
			Name:            "Computer Expenses",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            405,
			Name:            "Conferences & Seminars (GST Free)",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_FREE_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            406,
			Name:            "Conferences & Seminars (GST)",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            407,
			Name:            "Depreciation",
			AccountTypeName: EXPENSE,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            408,
			Name:            "Home Office - Set Rate",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_FREE_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            409,
			Name:            "Home Office",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            410,
			Name:            "Income Tax Expense",
			AccountTypeName: EXPENSE,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            411,
			Name:            "Interest Expense",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_FREE_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            412,
			Name:            "Internet",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            413,
			Name:            "Legal Fees",
			AccountTypeName: EXPENSE,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            414,
			Name:            "Licence Fees (GST Free)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            415,
			Name:            "Licence Fees (GST)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            416,
			Name:            "Merchant Fees",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            417,
			Name:            "Motor Vehicle - Set Rate",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            418,
			Name:            "M/V Fuel",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            419,
			Name:            "M/V Insurance",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            420,
			Name:            "M/V Registration",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            421,
			Name:            "M/V Repairs/Maintenance",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            422,
			Name:            "Office Supplies",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            423,
			Name:            "Postage",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            424,
			Name:            "Professional Memberships (GST Free)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            425,
			Name:            "Professional Memberships (GST)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            426,
			Name:            "Protective Clothing",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            427,
			Name:            "Repairs and Maintenance",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            428,
			Name:            "Salary and Wages",
			AccountTypeName: "Expense",
			AccountTaxName:  "BAS Excluded",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            429,
			Name:            "Sundries",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            430,
			Name:            "Superannuation",
			AccountTypeName: "Expense",
			AccountTaxName:  "BAS Excluded",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            431,
			Name:            "Telephone",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            432,
			Name:            "Telephone and Internet (GST Free)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            433,
			Name:            "Tolls / Parking",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            434,
			Name:            "Travel/Accommodation (GST Free)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            435,
			Name:            "Travel/Accommodation (GST)",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            436,
			Name:            "Uniforms",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            437,
			Name:            "Waste Disposal",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST on Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            438,
			Name:            "Laundry",
			AccountTypeName: "Expense",
			AccountTaxName:  "GST Free Expenses",
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            490,
			Name:            "Personal Super Contributions",
			AccountTypeName: OTHER,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            491,
			Name:            "Income Protection Insurance",
			AccountTypeName: OTHER,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            492,
			Name:            "Donations",
			AccountTypeName: OTHER,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},

		{
			Code:            600,
			Name:            "Bank",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            610,
			Name:            "Accounts Receivable",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            620,
			Name:            "Prepayments",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            630,
			Name:            "Inventory",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            710,
			Name:            "Office Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       true,
		},
		{
			Code:            711,
			Name:            "Accumulated Depreciation - Office Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            720,
			Name:            "Computer Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       true,
		},
		{
			Code:            721,
			Name:            "Accumulated Depreciation - Computer Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       true,
		},
		{
			Code:            730,
			Name:            "Dental/Medical Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       true,
		},
		{
			Code:            731,
			Name:            "Accumulated Depreciation - Dental/Medical Equipment",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            740,
			Name:            "Motor Vehicle",
			AccountTypeName: ASSET,
			AccountTaxName:  GST_ON_EXPENSES,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       true,
		},
		{
			Code:            741,
			Name:            "Accumulated Depreciation - Motor Vehicle",
			AccountTypeName: ASSET,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            800,
			Name:            "Accounts Payable",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            801,
			Name:            "Unpaid Expense Claims",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            804,
			Name:            "Wages Payable - Payroll",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            820,
			Name:            "GST",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            825,
			Name:            "PAYG Withholdings Payable",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            826,
			Name:            "Superannuation Payable",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            830,
			Name:            "Income Tax Payable",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            840,
			Name:            "Historical Adjustment",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            850,
			Name:            "Suspense",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            860,
			Name:            "Rounding",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            877,
			Name:            "Tracking Transfers",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            890,
			Name:            "Loan",
			AccountTypeName: LIABILITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},

		{
			Code:            900,
			Name:            "Owner A Drawings",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            910,
			Name:            "Take Home Pay",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            920,
			Name:            "Owner A Funds Introduced",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            960,
			Name:            "Retained Earnings",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        true,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            970,
			Name:            "Owner A Share Capital",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        false,
			IsCos:           false,
			IsCapital:       false,
		},
		{
			Code:            980,
			Name:            "Current Year Earnings",
			AccountTypeName: EQUITY,
			AccountTaxName:  BAS_EXCLUDED,
			IsSystem:        true,
			IsCos:           false,
			IsCapital:       false,
		},
	}

	err := InsertTemplate(ctx, tx, COA)
	if err != nil {
		return err
	}

	return nil
}

func downAccountTemplateSeeds(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM tbl_chart_of_accounts_template
		WHERE code IN (
			200,201,202,203,204,
			310,311,312,313,
			400,401,402,403,404,405,406,407,408,409,410,411,
			412,413,414,415,416,417,418,419,420,421,422,423,
			424,425,426,427,428,429,430,431,432,433,434,435,
			436,437,438,
			490,491,492,
			600,610,620,630,
			710,711,720,721,730,731,740,741,
			800,801,804,820,825,826,830,840,850,860,877,890,
			900,910,920,960,970,980
		)
	`)
	return err
}
