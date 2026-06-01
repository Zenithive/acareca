package equity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/fy"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CalculateOwnerEquity(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (*OwnerEquityCalculation, error)
	GetRetainedEarnings(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate string) (float64, error)
	CalculateCurrentYearEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (*EquityMovements, error)
}

type service struct {
	db     *sqlx.DB
	fyRepo fy.Repository
}

func NewService(db *sqlx.DB, fyRepo fy.Repository) Service {
	return &service{db: db, fyRepo: fyRepo}
}

type OwnerEquityCalculation struct {
	StartDate         string           `json:"start_date"`
	EndDate           string           `json:"end_date"`
	ShareCapital      float64          `json:"share_capital"`
	FundsIntroduced   float64          `json:"funds_introduced"`
	Drawings          float64          `json:"drawings"`
	RetainedEarnings  float64          `json:"retained_earnings"`
	CurrentYearProfit float64          `json:"current_year_profit"`
	TotalEquity       float64          `json:"total_equity"`
	EquityMovements   *EquityMovements `json:"equity_movements"`
}

type EquityMovements struct {
	OpeningBalance    float64 `json:"opening_balance"`
	FundsIntroduced   float64 `json:"funds_introduced"`
	Drawings          float64 `json:"drawings"`
	NetEquityMovement float64 `json:"net_equity_movement"`
	CurrentYearProfit float64 `json:"current_year_profit"`
	ClosingBalance    float64 `json:"closing_balance"`
}

func (s *service) CalculateOwnerEquity(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (*OwnerEquityCalculation, error) {
	shareCapital, err := s.getShareCapital(ctx, practitionerID, clinicID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get share capital: %w", err)
	}

	retainedEarnings, err := s.GetRetainedEarnings(ctx, practitionerID, clinicID, startDate)
	if err != nil {
		return nil, fmt.Errorf("get retained earnings: %w", err)
	}

	fundsIntroduced, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 881, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get funds introduced: %w", err)
	}

	drawings, err := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 880, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get drawings: %w", err)
	}

	currentYearProfit, err := s.getCurrentYearProfit(ctx, practitionerID, clinicID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get current year profit: %w", err)
	}

	movements, err := s.buildEquityMovements(ctx, practitionerID, clinicID, startDate, endDate, shareCapital, retainedEarnings, currentYearProfit)
	if err != nil {
		return nil, fmt.Errorf("build equity movements: %w", err)
	}

	totalEquity := shareCapital + retainedEarnings + fundsIntroduced - drawings + currentYearProfit

	return &OwnerEquityCalculation{
		StartDate:         startDate,
		EndDate:           endDate,
		ShareCapital:      shareCapital,
		FundsIntroduced:   fundsIntroduced,
		Drawings:          drawings,
		RetainedEarnings:  retainedEarnings,
		CurrentYearProfit: currentYearProfit,
		TotalEquity:       totalEquity,
		EquityMovements:   movements,
	}, nil
}

func (s *service) GetRetainedEarnings(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate string) (float64, error) {
	calcDate := startDate
	if calcDate == "" {
		fyStart, _, _ := s.fyBoundaries(ctx, "", time.Now().Format("2006-01-02"))
		calcDate = fyStart.Format("2006-01-02")
	}

	query := `
		SELECT COALESCE(SUM(signed_net_amount), 0)
		FROM vw_pl_line_items
		WHERE practitioner_id = $1 AND date::DATE < $2::DATE
	`
	args := []interface{}{practitionerID, calcDate}

	if clinicID != nil && *clinicID != uuid.Nil {
		query += " AND (clinic_id = $3 OR clinic_id = '00000000-0000-0000-0000-000000000000')"
		args = append(args, *clinicID)
	}

	var retained float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&retained); err != nil {
		return 0, err
	}
	return retained, nil
}

func (s *service) CalculateCurrentYearEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (*EquityMovements, error) {
	sc, err := s.getShareCapital(ctx, practitionerID, clinicID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	re, err := s.GetRetainedEarnings(ctx, practitionerID, clinicID, startDate)
	if err != nil {
		return nil, err
	}
	cy, err := s.getCurrentYearProfit(ctx, practitionerID, clinicID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	return s.buildEquityMovements(ctx, practitionerID, clinicID, startDate, endDate, sc, re, cy)
}

func (s *service) getShareCapital(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (float64, error) {
	query := `SELECT COALESCE(SUM(signed_amount), 0) FROM vw_balance_sheet_line_items WHERE practitioner_id = $1 AND account_code = 970`
	args := []interface{}{practitionerID}

	if startDate != "" {
		query += " AND date::DATE >= $2::DATE AND date::DATE <= $3::DATE"
		args = append(args, startDate, endDate)
	} else {
		query += " AND date::DATE <= $2::DATE"
		args = append(args, endDate)
	}

	if clinicID != nil && *clinicID != uuid.Nil {
		idx := len(args) + 1
		query += fmt.Sprintf(" AND (clinic_id = $%d OR clinic_id = '00000000-0000-0000-0000-000000000000')", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *service) getEquityAccountBalance(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, accountCode int16, startDate, endDate string) (float64, error) {
	query := `SELECT COALESCE(SUM(signed_amount), 0) FROM vw_balance_sheet_line_items WHERE practitioner_id = $1 AND account_code = $2`
	args := []interface{}{practitionerID, accountCode}

	if startDate != "" {
		query += " AND date >= $3::DATE AND date <= $4::DATE"
		args = append(args, startDate, endDate)
	} else {
		query += " AND date <= $3::DATE"
		args = append(args, endDate)
	}

	if clinicID != nil && *clinicID != uuid.Nil {
		idx := len(args) + 1
		query += fmt.Sprintf(" AND (clinic_id = $%d OR clinic_id = '00000000-0000-0000-0000-000000000000')", idx)
		args = append(args, *clinicID)
	}

	var balance float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *service) getCurrentYearProfit(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string) (float64, error) {
	effectiveStart := startDate
	if effectiveStart == "" {
		fyStart, _, err := s.fyBoundaries(ctx, "", endDate)
		if err != nil {
			return 0, err
		}
		effectiveStart = fyStart.Format("2006-01-02")
	}

	query := `SELECT COALESCE(SUM(signed_net_amount), 0) FROM vw_pl_line_items WHERE practitioner_id = $1 AND date::DATE >= $2::DATE AND date::DATE <= $3::DATE`
	args := []interface{}{practitionerID, effectiveStart, endDate}

	if clinicID != nil && *clinicID != uuid.Nil {
		query += " AND (clinic_id = $4 OR clinic_id = '00000000-0000-0000-0000-000000000000')"
		args = append(args, *clinicID)
	}

	var profit float64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&profit); err != nil {
		return 0, err
	}
	return profit, nil
}

func (s *service) fyBoundaries(ctx context.Context, startDate, endDate string) (start, end time.Time, err error) {
	asOf, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	fyRecord, err := s.fyRepo.GetFinancialYearByDate(ctx, asOf)
	if err == nil && fyRecord != nil {
		return fyRecord.StartDate, fyRecord.EndDate, nil
	}

	year := asOf.Year()
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC), nil
}

func (s *service) buildEquityMovements(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, startDate, endDate string, shareCapital, retainedEarnings, currentYearProfit float64) (*EquityMovements, error) {
	effStart := startDate
	if effStart == "" {
		fStart, _, _ := s.fyBoundaries(ctx, "", endDate)
		effStart = fStart.Format("2006-01-02")
	}

	yearFunds, _ := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 881, effStart, endDate)
	yearDrawings, _ := s.getEquityAccountBalance(ctx, practitionerID, clinicID, 880, effStart, endDate)

	openingBalance := shareCapital + retainedEarnings
	netMovement := yearFunds - yearDrawings
	closingBalance := openingBalance + netMovement + currentYearProfit

	return &EquityMovements{
		OpeningBalance:    openingBalance,
		FundsIntroduced:   yearFunds,
		Drawings:          yearDrawings,
		NetEquityMovement: netMovement,
		CurrentYearProfit: currentYearProfit,
		ClosingBalance:    closingBalance,
	}, nil
}
