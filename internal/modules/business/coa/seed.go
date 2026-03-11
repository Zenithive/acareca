package coa

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// DefaultChartRow defines one default chart-of-account row.
type DefaultChartRow struct {
	Code          int16  // 3–4 digit code (100–9999)
	Name          string
	AccountTypeID int16  // 1=Asset, 2=Liability, 3=Equity, 4=Revenue, 5=Expense
	AccountTaxID  int16  // 1=GST on Income, 2=GST on Expenses, etc.
}

// DefaultChartOfAccounts returns the default chart of accounts created for each practitioner.
func DefaultChartOfAccounts() []DefaultChartRow {
	return []DefaultChartRow{
		// Revenue accounts
		{Code: 200, Name: "Patient Fee Account", AccountTypeID: 4, AccountTaxID: 5}, // Revenue, GST Free Income
		{Code: 201, Name: "Commission Received", AccountTypeID: 4, AccountTaxID: 1}, // Revenue, GST on Income
		{Code: 202, Name: "Other Income", AccountTypeID: 4, AccountTaxID: 1},        // Revenue, GST on Income

		// Expense accounts
		{Code: 400, Name: "Home Office (GST)", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 401, Name: "Home Office (GST Free)", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 402, Name: "Laboratory Work (GST Free)", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 403, Name: "Laboratory Work (GST)", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 404, Name: "Subscription/Membership (GST Free)", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 405, Name: "Subscription/Membership (GST)", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 406, Name: "Bank Fees", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 407, Name: "Merchant Fees", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 408, Name: "Motor Vehicle - Set Rate", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 409, Name: "M/V Insurance", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 410, Name: "M/V Registration", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 411, Name: "M/V Fuel", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 412, Name: "M/V Repairs/Maintenance", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 413, Name: "Management Fee (Gross Up)", AccountTypeID: 5, AccountTaxID: 4},
		{Code: 414, Name: "Materials/Dental Supplies", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 415, Name: "Office Supplies", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 416, Name: "Postage", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 417, Name: "Protective Clothing", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 418, Name: "Internet", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 419, Name: "Telephone", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 420, Name: "Telephone and Internet (GST Free)", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 421, Name: "Travel/Accommodation (GST Free)", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 422, Name: "Travel/Accommodation (GST)", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 423, Name: "Tolls / Parking", AccountTypeID: 5, AccountTaxID: 3},
		{Code: 424, Name: "Waste Disposal", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 425, Name: "Repairs and Maintenance", AccountTypeID: 5, AccountTaxID: 2},
		{Code: 426, Name: "Sundries", AccountTypeID: 5, AccountTaxID: 2},

		// Asset accounts
		{Code: 610, Name: "Accounts Receivable", AccountTypeID: 1, AccountTaxID: 4},
		{Code: 620, Name: "Prepayments", AccountTypeID: 1, AccountTaxID: 4},
		{Code: 630, Name: "Inventory", AccountTypeID: 1, AccountTaxID: 4},
		{Code: 710, Name: "Office Equipment", AccountTypeID: 1, AccountTaxID: 2},
		{Code: 711, Name: "Accumulated Depreciation – Office Equipment", AccountTypeID: 1, AccountTaxID: 4},
		{Code: 720, Name: "Computer Equipment", AccountTypeID: 1, AccountTaxID: 2},
		{Code: 721, Name: "Accumulated Depreciation – Computer Equipment", AccountTypeID: 1, AccountTaxID: 4},

		// Liability accounts
		{Code: 800, Name: "Accounts Payable", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 801, Name: "Unpaid Expense Claims", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 804, Name: "Wages Payable – Payroll", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 820, Name: "GST", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 825, Name: "PAYG Withholdings Payable", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 826, Name: "Superannuation Payable", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 830, Name: "Income Tax Payable", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 840, Name: "Historical Adjustment", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 850, Name: "Suspense", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 860, Name: "Rounding", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 877, Name: "Tracking Transfers", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 880, Name: "Owner A Drawings", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 881, Name: "Owner A Funds Introduced", AccountTypeID: 2, AccountTaxID: 4},
		{Code: 900, Name: "Loan", AccountTypeID: 2, AccountTaxID: 4},

		// Equity accounts
		{Code: 960, Name: "Retained Earnings", AccountTypeID: 3, AccountTaxID: 4},
		{Code: 970, Name: "Owner A Share Capital", AccountTypeID: 3, AccountTaxID: 4},
	}
}

// SeedDefaultsForPractitioner creates default chart-of-account rows for a practitioner.
// practice_id = practitionerID, is_system = true.
func SeedDefaultsForPractitioner(ctx context.Context, repo Repository, practitionerID uuid.UUID) error {
	for _, row := range DefaultChartOfAccounts() {
		chart := &ChartOfAccount{
			practice_id:     practitionerID,
			AccountTypeID: row.AccountTypeID,
			AccountTaxID:  row.AccountTaxID,
			Code:          row.Code,
			Name:          row.Name,
			IsSystem:      true,
		}
		_, err := repo.CreateChart(ctx, chart)
		if err != nil {
			log.Printf("coa: seed default %d for practitioner %s: %v", row.Code, practitionerID, err)
			return err
		}
	}
	return nil
}
