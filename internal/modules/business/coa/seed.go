package coa

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type AccountClassification string

const (
	ClassificationCurrentAsset        AccountClassification = "Current Asset"
	ClassificationNonCurrentAsset     AccountClassification = "Non-Current Asset"
	ClassificationContraAsset         AccountClassification = "Contra-Asset"
	ClassificationCurrentLiability    AccountClassification = "Current Liability"
	ClassificationNonCurrentLiability AccountClassification = "Non-Current Liability"
	ClassificationEquity              AccountClassification = "Equity"
	ClassificationContraEquity        AccountClassification = "Contra-Equity"
	ClassificationOperatingRevenue    AccountClassification = "Operating Revenue"
	ClassificationOtherRevenue        AccountClassification = "Other Revenue"
	ClassificationDirectCosts         AccountClassification = "Direct Costs"
	ClassificationOperatingExpense    AccountClassification = "Operating Expense"
)

// DefaultChartRow defines one default chart-of-account row.
type DefaultChartRow struct {
	Code           int16 // 3–4 digit code (100–9999)
	Name           string
	AccountTypeID  int16                 // 1=Asset, 2=Liability, 3=Equity, 4=Revenue, 5=Expense
	AccountTaxID   int16                 // 1=GST on Income, 2=GST on Expenses, etc.
	Classification AccountClassification // Maps to account classification type
	IsSystem       bool                  // true only for owner fund side
}

// DefaultChartOfAccounts returns the default chart of accounts with classifications.
func DefaultChartOfAccounts() []DefaultChartRow {
	return []DefaultChartRow{
		// Revenue accounts
		{Code: 200, Name: "Patient Fee Account (GST Free)", AccountTypeID: 4, AccountTaxID: 5, Classification: ClassificationOperatingRevenue, IsSystem: false},
		{Code: 201, Name: "Patient Fee Account (GST)", AccountTypeID: 4, AccountTaxID: 1, Classification: ClassificationOperatingRevenue, IsSystem: false},
		{Code: 202, Name: "Commission Received", AccountTypeID: 4, AccountTaxID: 1, Classification: ClassificationOperatingRevenue, IsSystem: false},
		{Code: 203, Name: "Other Income", AccountTypeID: 4, AccountTaxID: 1, Classification: ClassificationOtherRevenue, IsSystem: false},

		// Expense accounts
		{Code: 400, Name: "Home Office (GST)", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 401, Name: "Home Office (GST Free)", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 402, Name: "Laboratory Work (GST Free)", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationDirectCosts, IsSystem: false},
		{Code: 403, Name: "Laboratory Work (GST)", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationDirectCosts, IsSystem: false},
		{Code: 404, Name: "Subscription/Membership (GST Free)", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 405, Name: "Subscription/Membership (GST)", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 406, Name: "Bank Fees", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 407, Name: "Merchant Fees", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 408, Name: "Motor Vehicle - Set Rate", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 409, Name: "M/V Insurance", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 410, Name: "M/V Registration", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 411, Name: "M/V Fuel", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 412, Name: "M/V Repairs/Maintenance", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 413, Name: "Management Fee (Gross Up)", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 414, Name: "Materials/Dental Supplies", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationDirectCosts, IsSystem: false},
		{Code: 415, Name: "Office Supplies", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 416, Name: "Postage", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 417, Name: "Protective Clothing", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 418, Name: "Internet", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 419, Name: "Telephone", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 420, Name: "Telephone and Internet (GST Free)", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 421, Name: "Travel/Accommodation (GST Free)", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 422, Name: "Travel/Accommodation (GST)", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 423, Name: "Tolls / Parking", AccountTypeID: 5, AccountTaxID: 3, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 424, Name: "Waste Disposal", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 425, Name: "Repairs and Maintenance", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},
		{Code: 426, Name: "Sundries", AccountTypeID: 5, AccountTaxID: 2, Classification: ClassificationOperatingExpense, IsSystem: false},

		// Asset accounts
		{Code: 600, Name: "Business Bank Account", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationCurrentAsset, IsSystem: true},
		{Code: 610, Name: "Accounts Receivable", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationCurrentAsset, IsSystem: false},
		{Code: 620, Name: "Prepayments", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationCurrentAsset, IsSystem: false},
		{Code: 630, Name: "Inventory", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationCurrentAsset, IsSystem: false},
		{Code: 710, Name: "Office Equipment", AccountTypeID: 1, AccountTaxID: 2, Classification: ClassificationNonCurrentAsset, IsSystem: false},
		{Code: 711, Name: "Accumulated Depreciation - Office Equipment", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationContraAsset, IsSystem: false},
		{Code: 720, Name: "Computer Equipment", AccountTypeID: 1, AccountTaxID: 2, Classification: ClassificationNonCurrentAsset, IsSystem: false},
		{Code: 721, Name: "Accumulated Depreciation - Computer Equipment", AccountTypeID: 1, AccountTaxID: 4, Classification: ClassificationContraAsset, IsSystem: false},

		// Liability accounts
		{Code: 800, Name: "Accounts Payable", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 801, Name: "Unpaid Expense Claims", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 804, Name: "Wages Payable - Payroll", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 820, Name: "GST", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 825, Name: "PAYG Withholdings Payable", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 826, Name: "Superannuation Payable", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 830, Name: "Income Tax Payable", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 840, Name: "Historical Adjustment", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 850, Name: "Suspense", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 860, Name: "Rounding", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 877, Name: "Tracking Transfers", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationCurrentLiability, IsSystem: false},
		{Code: 900, Name: "Loan", AccountTypeID: 2, AccountTaxID: 4, Classification: ClassificationNonCurrentLiability, IsSystem: false},

		// Equity accounts (owner fund side)
		{Code: 880, Name: "Owner A Drawings", AccountTypeID: 3, AccountTaxID: 4, Classification: ClassificationContraEquity, IsSystem: true},
		{Code: 881, Name: "Owner A Funds Introduced", AccountTypeID: 3, AccountTaxID: 4, Classification: ClassificationEquity, IsSystem: true},
		{Code: 960, Name: "Retained Earnings", AccountTypeID: 3, AccountTaxID: 4, Classification: ClassificationEquity, IsSystem: true},
		{Code: 970, Name: "Owner A Share Capital", AccountTypeID: 3, AccountTaxID: 4, Classification: ClassificationEquity, IsSystem: true},
	}
}

// SeedDefaultsForPractitioner creates default chart-of-account rows for a practitioner in a single bulk insert.
func SeedDefaultsForPractitioner(ctx context.Context, repo Repository, practitionerID uuid.UUID, tx *sqlx.Tx) error {
	defaults := DefaultChartOfAccounts()
	rows := make([]*ChartOfAccount, len(defaults))
	for i, row := range defaults {
		rows[i] = &ChartOfAccount{
			PractitionerID: practitionerID,
			AccountTypeID:  row.AccountTypeID,
			AccountTaxID:   row.AccountTaxID,
			Code:           row.Code,
			Name:           row.Name,
			Key:            GenerateKeyFromName(row.Name),
			Classification: row.Classification,
			IsSystem:       row.IsSystem,
		}
	}
	return repo.BulkCreateChartOfAccounts(ctx, rows, tx)
}
