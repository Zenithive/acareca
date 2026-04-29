package equity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
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
	db     *sqlx.DB
	fyRepo fy.Repository
}

func NewService(db *sqlx.DB, fyRepo fy.Repository) Service {
	return &service{db: db, fyRepo: fyRepo}
}

// OwnerEquityCalculation represents the complete owner equity calculation
type OwnerEquityCalculation struct {
	AsOfDate          string           `json:"as_of_date"`
	ShareCapital      float64          `json:"share_capital"`       // Opening capital
	FundsIntroduced   float64          `json:"funds_introduced"`    // Current year contributions
	Drawings          float64          `json:"drawings"`            // Current year withdrawals
	RetainedEarnings  float64          `json:"retained_earnings"`   // Prior years' profits
	CurrentYearProfit float64          `json:"current_year_profit"` // This year's profit
	TotalEquity       float64          `json:"total_equity"`        // Sum of all
	EquityMovements   *EquityMovements `json:"equity_movements"`    // Detailed breakdown
}

// EquityMovements represents detailed equity changes
type EquityMovements struct {
	OpeningBalance    float64 `json:"opening_balance"`
	FundsIntroduced   float64 `json:"funds_introduced"`
	Drawings          float64 `json:"drawings"`
	NetEquityMovement float64 `json:"net_equity_movement"`
	CurrentYearProfit float64 `json:"current_year_profit"`
	ClosingBalance    float64 `json:"closing_balance"`
}

// CalculateOwnerEquity calculates all owner fund balances automatically
func (s *service) CalculateOwnerEquity(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*OwnerEquityCalculation, error) {

	shareCapital, err := s.getShareCapital(ctx, practitionerID, clinicID, asOfDate) // date-bounded (Flaw 4 fix)
	if err != nil {
		return nil, fmt.Errorf("get share capital: %w", err)
	}

	retainedEarnings, err := s.GetRetainedEarnings(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get retained earnings: %w", err)
	}

	// All-time cumulative figures — NOT year-scoped
	allTimeFundsIntroduced, err := s.getAllTimeEquityAccountBalance(ctx, practitionerID, clinicID, 881, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get all-time funds introduced: %w", err)
	}

	allTimeDrawings, err := s.getAllTimeEquityAccountBalance(ctx, practitionerID, clinicID, 880, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get all-time drawings: %w", err)
	}

	currentYearProfit, err := s.getCurrentYearProfit(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get current year profit: %w", err)
	}

	// Year-scoped figures for the equity movements statement only
	movements, err := s.buildEquityMovements(ctx, practitionerID, clinicID, asOfDate, shareCapital, retainedEarnings, currentYearProfit)
	if err != nil {
		return nil, fmt.Errorf("build equity movements: %w", err)
	}

	// Balance sheet total uses all-time equity movements
	totalEquity := shareCapital + retainedEarnings + allTimeFundsIntroduced - allTimeDrawings + currentYearProfit

	return &OwnerEquityCalculation{
		AsOfDate:          asOfDate,
		ShareCapital:      shareCapital,
		FundsIntroduced:   allTimeFundsIntroduced, // shown on balance sheet
		Drawings:          allTimeDrawings,        // shown on balance sheet
		RetainedEarnings:  retainedEarnings,
		CurrentYearProfit: currentYearProfit,
		TotalEquity:       totalEquity,
		EquityMovements:   movements, // year-scoped, for the movements statement
	}, nil
}

func (s *service) GetRetainedEarnings(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error) {
	fyStart, _, err := s.fyBoundaries(ctx, asOfDate)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COALESCE(SUM(signed_net_amount), 0) AS retained_earnings
		FROM vw_pl_line_items
		WHERE practitioner_id = $1
		  AND submitted_at::DATE < $2::DATE
	`
	args := []interface{}{practitionerID, fyStart.Format("2006-01-02")}
	idx := 3

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var retained float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&retained); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get retained earnings: %w", err)
	}
	return retained, nil
}

// CalculateCurrentYearEquityMovements calculates funds introduced and drawings for current year
func (s *service) CalculateCurrentYearEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (*EquityMovements, error) {
	sc, err := s.getShareCapital(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, err
	}
	re, err := s.GetRetainedEarnings(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, err
	}
	cy, err := s.getCurrentYearProfit(ctx, practitionerID, clinicID, asOfDate)
	if err != nil {
		return nil, err
	}
	return s.buildEquityMovements(ctx, practitionerID, clinicID, asOfDate, sc, re, cy)
}

func (s *service) getShareCapital(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(signed_amount), 0) AS balance
		FROM vw_balance_sheet_line_items
		WHERE practitioner_id = $1
		  AND account_code    = 970
		  AND submitted_at   <= $2::DATE     -- ← date boundary added
	`
	args := []interface{}{practitionerID, asOfDate}
	idx := 3

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get share capital: %w", err)
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
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get equity account %d balance: %w", accountCode, err)
	}
	return balance, nil
}

// getCurrentYearProfit gets net profit for the current year
func (s *service) getCurrentYearProfit(
	ctx context.Context,
	practitionerID uuid.UUID,
	clinicID *uuid.UUID,
	asOfDate string,
) (float64, error) {
	fyStart, _, err := s.fyBoundaries(ctx, asOfDate) // from the previously reported Bug 2 fix
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COALESCE(SUM(signed_net_amount), 0) AS current_year_profit
		FROM vw_pl_line_items
		WHERE practitioner_id = $1
		  AND submitted_at::DATE >= $2::DATE
		  AND submitted_at::DATE <= $3::DATE
	`
	args := []interface{}{practitionerID, fyStart.Format("2006-01-02"), asOfDate}
	idx := 4

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var profit float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&profit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get current year profit: %w", err)
	}
	return profit, nil
}

// equity/service.go — new helper

// fyBoundaries returns the start and end dates of the financial year
// that contains asOfDate. Falls back to calendar year if no FY is found.
func (s *service) fyBoundaries(ctx context.Context, asOfDate string) (start, end time.Time, err error) {
	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date format: %w", err)
	}

	fyRecord, err := s.fyRepo.GetFinancialYearByDate(ctx, asOf)
	if err == nil && fyRecord != nil {
		return fyRecord.StartDate, fyRecord.EndDate, nil
	}

	// Fallback: calendar year
	year := asOf.Year()
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC),
		nil
}

func (s *service) buildEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string, shareCapital float64, retainedEarnings float64, currentYearProfit float64) (*EquityMovements, error) {
	fyStart, _, err := s.fyBoundaries(ctx, asOfDate)
	if err != nil {
		return nil, err
	}
	fyStartStr := fyStart.Format("2006-01-02")

	// Current year movements (year-scoped — for the movements statement only)
	yearFundsIntroduced, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 881, fyStartStr, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get year funds introduced: %w", err)
	}
	yearDrawings, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 880, fyStartStr, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get year drawings: %w", err)
	}

	priorFundsIntroduced, err := s.getAllTimeEquityAccountBalance(ctx, practitionerID, clinicID, 881, fyStartStr)
	if err != nil {
		return nil, fmt.Errorf("get prior funds introduced for opening balance: %w", err)
	}
	priorDrawings, err := s.getAllTimeEquityAccountBalance(ctx, practitionerID, clinicID, 880, fyStartStr)
	if err != nil {
		return nil, fmt.Errorf("get prior drawings for opening balance: %w", err)
	}

	openingBalance := shareCapital + retainedEarnings + priorFundsIntroduced - priorDrawings

	netEquityMovement := yearFundsIntroduced - yearDrawings
	closingBalance := openingBalance + netEquityMovement + currentYearProfit

	return &EquityMovements{
		OpeningBalance:    openingBalance,
		FundsIntroduced:   yearFundsIntroduced,
		Drawings:          yearDrawings,
		NetEquityMovement: netEquityMovement,
		CurrentYearProfit: currentYearProfit,
		ClosingBalance:    closingBalance,
	}, nil
}

func (s *service) getAllTimeEquityAccountBalance(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, accountCode int16, asOfDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(signed_amount), 0) AS balance
		FROM vw_balance_sheet_line_items
		WHERE practitioner_id = $1
		  AND account_code    = $2
		  AND submitted_at   <= $3::DATE
	`
	args := []interface{}{practitionerID, accountCode, asOfDate}
	idx := 4

	if clinicID != nil {
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get all-time equity balance for code %d: %w", accountCode, err)
	}
	return balance, nil
}
