package bs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, practitionerID uuid.UUID, f *BSFilter) (*RsBalanceSheet, error)
}

type service struct {
	repo      Repository
	equitySvc equity.Service
}

func NewService(repo Repository, equitySvc equity.Service) Service {
	return &service{
		repo:      repo,
		equitySvc: equitySvc,
	}
}

func (s *service) GetBalanceSheet(ctx context.Context, practitionerID uuid.UUID, f *BSFilter) (*RsBalanceSheet, error) {
	// Default to today if no date specified
	asOfDate := time.Now().Format("2006-01-02")
	if f.AsOfDate != nil && *f.AsOfDate != "" {
		asOfDate = *f.AsOfDate
	}

	// Parse clinic ID if provided
	var clinicID *uuid.UUID
	if f.ClinicID != nil && *f.ClinicID != "" {
		id, err := uuid.Parse(*f.ClinicID)
		if err != nil {
			return nil, fmt.Errorf("invalid clinic_id: %w", err)
		}
		clinicID = &id
	}

	// Get balance sheet accounts (assets, liabilities, non-calculated equity)
	rows, err := s.repo.GetBalanceSheet(ctx, practitionerID, f)
	if err != nil {
		return nil, err
	}

	// Get automatically calculated owner equity
	ownerEquity, err := s.equitySvc.CalculateOwnerEquity(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("calculate owner equity: %w", err)
	}

	// Organize by account type
	var assets, liabilities, equity []RsAccount
	var totalAssets, totalLiabilities, totalOtherEquity float64

	for _, row := range rows {
		account := row.ToRs()

		fmt.Println(row.ToRs().Balance)

		switch row.AccountType {
		case "Asset":
			assets = append(assets, account)
			totalAssets += row.Balance
		case "Liability":
			liabilities = append(liabilities, account)
			totalLiabilities += row.Balance
		case "Equity":
			fmt.Println("ll", row)
			if row.AccountCode != 880 && row.AccountCode != 881 &&
				row.AccountCode != 960 && row.AccountCode != 970 {
				equity = append(equity, account)
				fmt.Println("row balance", row.Balance)
				totalOtherEquity += row.Balance
			}
		}
	}

	// Add automatically calculated owner fund accounts
	if ownerEquity.ShareCapital != 0 {
		equity = append(equity, RsAccount{
			Code:    970,
			Name:    "Owner A Share Capital",
			Balance: ownerEquity.ShareCapital,
		})
	}

	if ownerEquity.FundsIntroduced != 0 {
		equity = append(equity, RsAccount{
			Code:    881,
			Name:    "Owner A Funds Introduced",
			Balance: ownerEquity.FundsIntroduced,
		})
	}

	if ownerEquity.Drawings != 0 {
		equity = append(equity, RsAccount{
			Code:    880,
			Name:    "Owner A Drawings",
			Balance: -ownerEquity.Drawings, // Show as negative
		})
	}

	if ownerEquity.RetainedEarnings != 0 {
		equity = append(equity, RsAccount{
			Code:    960,
			Name:    "Retained Earnings",
			Balance: ownerEquity.RetainedEarnings,
		})
	}

	if ownerEquity.CurrentYearProfit != 0 {
		equity = append(equity, RsAccount{
			Code:    961,
			Name:    "Current Year Profit",
			Balance: ownerEquity.CurrentYearProfit,
		})
	}

	fmt.Println("total: ", ownerEquity.TotalEquity)

	totalEquity := ownerEquity.TotalEquity + totalOtherEquity
	return &RsBalanceSheet{
		AsOfDate:                  asOfDate,
		Assets:                    assets,
		TotalAssets:               totalAssets,
		Liabilities:               liabilities,
		TotalLiabilities:          totalLiabilities,
		Equity:                    equity,
		CurrentYearProfit:         ownerEquity.CurrentYearProfit,
		TotalEquity:               totalEquity,
		TotalLiabilitiesAndEquity: totalLiabilities + totalEquity,
	}, nil
}
