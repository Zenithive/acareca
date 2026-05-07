package fy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound            = errors.New("financial year not found")
	ErrInvalidFYYearFormat = errors.New("invalid fy_year format, expected YYYY-YYYY (e.g. 2025-2026)")
)

type Repository interface {
	CreateFinancialYear(ctx context.Context, fy *FinancialYear, tx *sqlx.Tx) (*FinancialYear, error)
	CreateFinancialQuarter(ctx context.Context, fq *FinancialQuarter, tx *sqlx.Tx) (*FinancialQuarter, error)
	GetFinancialYears(ctx context.Context) ([]FinancialYear, error)
	GetFinancialYearByID(ctx context.Context, id uuid.UUID) (*FinancialYear, error)
	GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]FinancialQuarter, error)
	UpdateFinancialYear(ctx context.Context, fy *FinancialYear, tx *sqlx.Tx) (*FinancialYear, error)
	DeactivateAllFinancialYearsExcept(ctx context.Context, tx *sqlx.Tx, excludeID uuid.UUID) error
	GetFinancialYearByDate(ctx context.Context, expenseDate time.Time) (*FinancialYear, error)
	DeleteQuartersByFYID(ctx context.Context, fyID uuid.UUID, tx *sqlx.Tx) error
	CountActiveFY(ctx context.Context, tx *sqlx.Tx) (int, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateFinancialYear(ctx context.Context, fy *FinancialYear, tx *sqlx.Tx) (*FinancialYear, error) {
	query := `
		INSERT INTO tbl_financial_year (label, is_active, start_date, end_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, label, is_active, start_date, end_date, created_at, updated_at
	`
	var f FinancialYear
	err := tx.QueryRowxContext(ctx, query,
		fy.Label, fy.IsActive, fy.StartDate, fy.EndDate,
	).StructScan(&f)
	if err != nil {
		return nil, fmt.Errorf("create financial year: %w", err)
	}
	return &f, nil
}

func (r *repository) CreateFinancialQuarter(ctx context.Context, fq *FinancialQuarter, tx *sqlx.Tx) (*FinancialQuarter, error) {
	query := `
		INSERT INTO tbl_financial_quarter (financial_year_id, label, start_date, end_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, financial_year_id, label, start_date, end_date, created_at, updated_at
	`
	var q FinancialQuarter
	err := tx.QueryRowxContext(ctx, query,
		fq.FinancialYearID, fq.Label, fq.StartDate, fq.EndDate,
	).StructScan(&q)
	if err != nil {
		return nil, fmt.Errorf("create financial quarter: %w", err)
	}
	return &q, nil
}

func (r *repository) GetFinancialYears(ctx context.Context) ([]FinancialYear, error) {
	query := `
		SELECT id, label, is_active, start_date, end_date, created_at, updated_at
		FROM tbl_financial_year
		ORDER BY start_date DESC
	`
	var years []FinancialYear
	if err := r.db.SelectContext(ctx, &years, query); err != nil {
		return nil, fmt.Errorf("get financial years: %w", err)
	}
	return years, nil
}

func (r *repository) GetFinancialYearByID(ctx context.Context, id uuid.UUID) (*FinancialYear, error) {
	query := `
		SELECT id, label, is_active, start_date, end_date, created_at, updated_at
		FROM tbl_financial_year
		WHERE id = $1
	`
	var fy FinancialYear
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&fy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get financial year by id: %w", err)
	}
	return &fy, nil
}

func (r *repository) GetFinancialQuarters(ctx context.Context, financialYearID uuid.UUID) ([]FinancialQuarter, error) {
	query := `
		SELECT id, financial_year_id, label, start_date, end_date, created_at, updated_at
		FROM tbl_financial_quarter
		WHERE financial_year_id = $1
		ORDER BY start_date ASC
	`
	var quarters []FinancialQuarter
	if err := r.db.SelectContext(ctx, &quarters, query, financialYearID); err != nil {
		return nil, fmt.Errorf("get financial quarters: %w", err)
	}
	return quarters, nil
}

func (r *repository) UpdateFinancialYear(ctx context.Context, fy *FinancialYear, tx *sqlx.Tx) (*FinancialYear, error) {
	query := `
		UPDATE tbl_financial_year 
		SET label = $1, is_active = $2, start_date = $3, end_date = $4, updated_at = now()
		WHERE id = $5
		RETURNING id, label, is_active, start_date, end_date, created_at, updated_at
	`
	var f FinancialYear
	err := tx.QueryRowxContext(ctx, query, fy.Label, fy.IsActive, fy.StartDate, fy.EndDate, fy.ID).StructScan(&f)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update financial year: %w", err)
	}
	return &f, nil
}

func (r *repository) DeactivateAllFinancialYearsExcept(ctx context.Context, tx *sqlx.Tx, excludeID uuid.UUID) error {
	query := `UPDATE tbl_financial_year SET is_active = FALSE, updated_at = now() WHERE id != $1 AND is_active = TRUE`
	_, err := tx.ExecContext(ctx, query, excludeID)
	return err
}

func (r *repository) GetFinancialYearByDate(ctx context.Context, expenseDate time.Time) (*FinancialYear, error) {
	query := `
        SELECT id, label, is_active, start_date, end_date, created_at, updated_at
        FROM tbl_financial_year
        WHERE $1 BETWEEN start_date AND end_date AND is_active = TRUE
        LIMIT 1
    `
	var fy FinancialYear
	if err := r.db.GetContext(ctx, &fy, query, expenseDate); err != nil {
		return nil, err
	}
	return &fy, nil
}

func (r *repository) DeleteQuartersByFYID(ctx context.Context, fyID uuid.UUID, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM tbl_financial_quarter WHERE financial_year_id = $1`, fyID)
	return err
}

func (r *repository) CountActiveFY(ctx context.Context, tx *sqlx.Tx) (int, error) {
	var count int
	err := tx.GetContext(ctx, &count, `SELECT COUNT(*) FROM tbl_financial_year WHERE is_active = TRUE`)
	return count, err
}
