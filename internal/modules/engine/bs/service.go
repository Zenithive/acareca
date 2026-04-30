package bs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/equity"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	GetBalanceSheet(ctx context.Context, practitionerID uuid.UUID, f *BSFilter) (*RsBalanceSheet, error)
}

type service struct {
	repo      Repository
	equitySvc equity.Service
	db        sqlx.DB
}

func NewService(repo Repository, equitySvc equity.Service, db sqlx.DB) Service {
	return &service{
		repo:      repo,
		equitySvc: equitySvc,
		db:        db,
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

	// Get balance sheet accounts (assets, liabilities, other equity accounts)
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

		switch row.AccountType {
		case "Asset":
			assets = append(assets, account)
			totalAssets += row.Balance
		case "Liability":
			liabilities = append(liabilities, account)
			totalLiabilities += row.Balance
		case "Equity":
			// Skip owner fund accounts (880, 881, 960, 970) - they're calculated by equity service
			if row.AccountCode != 880 && row.AccountCode != 881 &&
				row.AccountCode != 960 && row.AccountCode != 970 {
				equity = append(equity, account)
				totalOtherEquity += row.Balance
			}
		}
	}

	// Build equity section from calculated values
	if ownerEquity.ShareCapital != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 970)
		if err != nil {
			return nil, err
		}

		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    970,
			Name:    "Owner A Share Capital",
			Balance: ownerEquity.ShareCapital,
		})
	}

	if ownerEquity.FundsIntroduced != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 881)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    881,
			Name:    "Owner A Funds Introduced",
			Balance: ownerEquity.FundsIntroduced,
		})
	}

	if ownerEquity.Drawings != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 880)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    880,
			Name:    "Owner A Drawings",
			Balance: -ownerEquity.Drawings,
		})
	}

	if ownerEquity.RetainedEarnings != 0 {
		coaId, err := s.getCoaIDByAccountCode(ctx, practitionerID, 960)
		if err != nil {
			return nil, err
		}
		equity = append(equity, RsAccount{
			CoaId:   *coaId,
			Code:    960,
			Name:    "Retained Earnings",
			Balance: ownerEquity.RetainedEarnings,
		})
	}

	// Total equity = calculated owner equity + other equity accounts
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

// getCoaIDByAccountCode retrieves the coa_id for a given account code
func (s *service) getCoaIDByAccountCode(ctx context.Context, practitionerID uuid.UUID, accountCode int16) (*uuid.UUID, error) {
	query := `
		SELECT id
		FROM tbl_chart_of_accounts
		WHERE practitioner_id = $1
		  AND code = $2
		  AND deleted_at IS NULL
		LIMIT 1
	`
	args := []interface{}{practitionerID, accountCode}

	var coaID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&coaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("COA account with code %d not found for practitioner %s", accountCode, practitionerID)
		}
		return nil, fmt.Errorf("get coa_id for account code %d: %w", accountCode, err)
	}
	return &coaID, nil
}
