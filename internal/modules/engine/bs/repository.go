package bs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all DB queries for the Balance Sheet module
type Repository interface {
	GetBalanceSheet(ctx context.Context, practitionerID uuid.UUID, f *BSFilter) ([]*BSRow, error)
	GetCurrentYearProfit(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetBalanceSheet(ctx context.Context, practitionerID uuid.UUID, f *BSFilter) ([]*BSRow, error) {
	query := `
		SELECT 
			practitioner_id,
			clinic_id,
			account_type,
			account_code,
			account_name,
			coa_id,
			balance,
			entry_count,
			TO_CHAR(last_transaction_date, 'YYYY-MM-DD') AS last_transaction_date
		FROM vw_balance_sheet_summary
		WHERE practitioner_id = $1
	`
	args := []interface{}{practitionerID}
	idx := 2

	if f.ClinicID != nil && *f.ClinicID != "" {
		clinicID, err := uuid.Parse(*f.ClinicID)
		if err != nil {
			return nil, fmt.Errorf("invalid clinic_id: %w", err)
		}
		query += fmt.Sprintf(" AND clinic_id = $%d", idx)
		args = append(args, clinicID)
		idx++
	}

	// TODO: Add as_of_date filter if needed
	// This would require modifying the view or adding a subquery to filter by submitted_at <= as_of_date

	query += " ORDER BY account_type, account_code"

	var rows []*BSRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get balance sheet: %w", err)
	}
	return rows, nil
}

// GetCurrentYearProfit fetches net profit from P&L for current year
// This is added to equity section of balance sheet
func (r *repository) GetCurrentYearProfit(ctx context.Context, practitionerID uuid.UUID, clinicID *uuid.UUID, asOfDate string) (float64, error) {
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
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&profit); err != nil {
		return 0, fmt.Errorf("get current year profit: %w", err)
	}
	return profit, nil
}
