package coa

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// DefaultChartRow defines one default chart-of-account row (e.g. Xero-style defaults).
type DefaultChartRow struct {
	Code          string
	Name          string
	AccountTypeID int16 // 1=Asset, 2=Liability, 3=Equity, 4=Revenue, 5=Expense
	AccountTaxID  int16 // 1=GST on Income, 2=GST on Expenses, etc.
}

// DefaultChartOfAccounts returns the 4–5 default accounts created for each practitioner.
func DefaultChartOfAccounts() []DefaultChartRow {
	return []DefaultChartRow{
		{Code: "1-1000", Name: "Bank", AccountTypeID: 1, AccountTaxID: 1},           // Asset
		{Code: "2-2000", Name: "Accounts Receivable", AccountTypeID: 1, AccountTaxID: 1},
		{Code: "3-3000", Name: "Equity", AccountTypeID: 3, AccountTaxID: 3},      // Equity, GST Free
		{Code: "4-4000", Name: "Revenue", AccountTypeID: 4, AccountTaxID: 1},     // Revenue
		{Code: "5-5000", Name: "Expenses", AccountTypeID: 5, AccountTaxID: 2},    // Expense
	}
}

// SeedDefaultsForPractitioner creates default chart-of-account rows for a practitioner.
// created_by = practitionerID, system_provider = true, is_system = true.
// Reusable from onboarding or admin flows.
func SeedDefaultsForPractitioner(ctx context.Context, repo Repository, practitionerID uuid.UUID) error {
	for _, row := range DefaultChartOfAccounts() {
		chart := &ChartOfAccount{
			CreatedBy:      practitionerID,
			AccountTypeID:  row.AccountTypeID,
			AccountTaxID:   row.AccountTaxID,
			Code:           row.Code,
			Name:           row.Name,
			Description:    nil,
			IsSystem:       true,
			SystemProvider: true,
			IsActive:       true,
		}
		_, err := repo.CreateChart(ctx, chart)
		if err != nil {
			log.Printf("coa: seed default %q for practitioner %s: %v", row.Code, practitionerID, err)
			return err
		}
	}
	return nil
}
