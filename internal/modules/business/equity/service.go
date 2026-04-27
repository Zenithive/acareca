package equity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Service handles automatic calculation of owner fund accounts
type Service interface {
	// CalculateOwnerEquity calculates all owner fund balances automatically
	CalculateOwnerEquity(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*OwnerEquityCalculation, error)
	
	// GetRetainedEarnings calculates retained earnings from prior years
	GetRetainedEarnings(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error)
	
	// CalculateCurrentYearEquityMovements calculates net equity changes for current year
	CalculateCurrentYearEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*EquityMovements, error)
}

type service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) Service {
	return &service{db: db}
}

// OwnerEquityCalculation represents the complete owner equity calculation
type OwnerEquityCalculation struct {
	AsOfDate              string           `json:"as_of_date"`
	ShareCapital          float64          `json:"share_capital"`           // Opening capital
	FundsIntroduced       float64          `json:"funds_introduced"`        // Current year contributions
	Drawings              float64          `json:"drawings"`                // Current year withdrawals
	RetainedEarnings      float64          `json:"retained_earnings"`       // Prior years' profits
	CurrentYearProfit     float64          `json:"current_year_profit"`     // This year's profit
	TotalEquity           float64          `json:"total_equity"`            // Sum of all
	EquityMovements       *EquityMovements `json:"equity_movements"`        // Detailed breakdown
}

// EquityMovements represents detailed equity changes
type EquityMovements struct {
	OpeningBalance        float64 `json:"opening_balance"`
	FundsIntroduced       float64 `json:"funds_introduced"`
	Drawings              float64 `json:"drawings"`
	NetEquityMovement     float64 `json:"net_equity_movement"`
	CurrentYearProfit     float64 `json:"current_year_profit"`
	ClosingBalance        float64 `json:"closing_balance"`
}

// CalculateOwnerEquity calculates all owner fund balances automatically
func (s *service) CalculateOwnerEquity(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*OwnerEquityCalculation, error) {
	// 1. Get share capital (opening balance - typically from code 970)
	shareCapital, err := s.getShareCapital(ctx, practitionerID, clinicID)
	if err != nil {
		return nil, fmt.Errorf("get share capital: %w", err)
	}

	// 2. Get retained earnings (prior years' accumulated profits)
	retainedEarnings, err := s.GetRetainedEarnings(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get retained earnings: %w", err)
	}

	// 3. Get current year equity movements (funds introduced and drawings)
	movements, err := s.CalculateCurrentYearEquityMovements(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get equity movements: %w", err)
	}

	// 4. Get current year profit from P&L
	currentYearProfit, err := s.getCurrentYearProfit(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get current year profit: %w", err)
	}

	// 5. Calculate total equity
	totalEquity := shareCapital + retainedEarnings + movements.FundsIntroduced - movements.Drawings + currentYearProfit

	return &OwnerEquityCalculation{
		AsOfDate:          asOfDate,
		ShareCapital:      shareCapital,
		FundsIntroduced:   movements.FundsIntroduced,
		Drawings:          movements.Drawings,
		RetainedEarnings:  retainedEarnings,
		CurrentYearProfit: currentYearProfit,
		TotalEquity:       totalEquity,
		EquityMovements:   movements,
	}, nil
}

// GetRetainedEarnings calculates retained earnings from all prior years
func (s *service) GetRetainedEarnings(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error) {
	// Parse the as_of_date to get the year
	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return 0, fmt.Errorf("invalid date format: %w", err)
	}
	currentYear := asOf.Year()

	// Get all profits from years before the current year
	query := `
		SELECT COALESCE(SUM(net_profit_net), 0) AS retained_earnings
		FROM vw_pl_summary_monthly
		WHERE practitioner_id = $1
		  AND EXTRACT(YEAR FROM period_month) < $2
	`
	args := []interface{}{practitionerID, currentYear}
	idx := 3

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var retainedEarnings float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&retainedEarnings); err != nil {
		return 0, fmt.Errorf("query retained earnings: %w", err)
	}

	return retainedEarnings, nil
}

// CalculateCurrentYearEquityMovements calculates funds introduced and drawings for current year
func (s *service) CalculateCurrentYearEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*EquityMovements, error) {
	// Parse the as_of_date to get the year
	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	currentYear := asOf.Year()
	yearStart := time.Date(currentYear, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	// Get funds introduced (code 881) for current year
	fundsIntroduced, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 881, yearStart, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get funds introduced: %w", err)
	}

	// Get drawings (code 880) for current year
	drawings, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 880, yearStart, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get drawings: %w", err)
	}

	// Get opening balance (share capital + retained earnings from prior years)
	shareCapital, _ := s.getShareCapital(ctx, practitionerID, clinicID)
	retainedEarnings, _ := s.GetRetainedEarnings(ctx, practitionerID, clinicID, asOfDate)
	openingBalance := shareCapital + retainedEarnings

	// Get current year profit
	currentYearProfit, _ := s.getCurrentYearProfit(ctx, practitionerID, clinicID, asOfDate)

	netEquityMovement := fundsIntroduced - drawings
	closingBalance := openingBalance + netEquityMovement + currentYearProfit

	return &EquityMovements{
		OpeningBalance:    openingBalance,
		FundsIntroduced:   fundsIntroduced,
		Drawings:          drawings,
		NetEquityMovement: netEquityMovement,
		CurrentYearProfit: currentYearProfit,
		ClosingBalance:    closingBalance,
	}, nil
}

// getShareCapital gets the share capital balance (code 970)
func (s *service) getShareCapital(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID) (float64, error) {
	query := `
		SELECT COALESCE(SUM(signed_amount), 0) AS balance
		FROM vw_balance_sheet_line_items
		WHERE practitioner_id = $1
		  AND account_code = 970
	`
	args := []interface{}{practitionerID}
	idx := 2

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		return 0, nil // Return 0 if no share capital entries exist
	}

	return balance, nil
}

// getEquityAccountBalance gets balance for a specific equity account within date range
func (s *service) getEquityAccountBalance(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, accountCode int16, fromDate, toDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(signed_amount), 0) AS balance
		FROM vw_balance_sheet_line_items
		WHERE practitioner_id = $1
		  AND account_code = $2
		  AND submitted_at >= $3::DATE
		  AND submitted_at <= $4::DATE
	`
	args := []interface{}{practitionerID, accountCode, fromDate, toDate}
	idx := 5

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		return 0, nil // Return 0 if no entries exist
	}

	return balance, nil
}

// getCurrentYearProfit gets net profit for the current year
func (s *service) getCurrentYearProfit(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(net_profit_net), 0) AS current_year_profit
		FROM vw_pl_summary_monthly
		WHERE practitioner_id = $1
		  AND period_month <= DATE_TRUNC('month', $2::DATE)
		  AND EXTRACT(YEAR FROM period_month) = EXTRACT(YEAR FROM $2::DATE)
	`
	args := []interface{}{practitionerID, asOfDate}
	idx := 3

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var profit float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&profit); err != nil {
		return 0, fmt.Errorf("query current year profit: %w", err)
	}

	return profit, nil
}
