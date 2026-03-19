package pl

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all DB queries for the P&L module.
type Repository interface {
	GetMonthlySummary(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLSummaryRow, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLAccountRow, error)
	GetByResponsibility(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLResponsibilityRow, error)
	GetFYSummary(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLFYSummaryRow, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetMonthlySummary(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLSummaryRow, error) {
	query := `
		SELECT
			practitioner_id, period_month,
			income_net, income_gst, income_gross,
			cogs_net, cogs_gst, cogs_gross,
			gross_profit_net,
			other_expenses_net, other_expenses_gst, other_expenses_gross,
			net_profit_net, net_profit_gross
		FROM vw_pl_summary_monthly
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND period_month >= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND period_month <= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += " ORDER BY period_month ASC"

	var rows []*PLSummaryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get monthly summary: %w", err)
	}
	return rows, nil
}

func (r *repository) GetByAccount(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLAccountRow, error) {
	query := `
		SELECT
			practitioner_id, period_month,
			pl_section, section_type,
			account_code, account_name, account_type,
			tax_name, tax_rate,
			total_net, total_gst, total_gross,
			signed_net, signed_gross,
			entry_count
		FROM vw_pl_by_account
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND period_month >= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND period_month <= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += " ORDER BY period_month ASC, pl_section ASC, account_code ASC"

	var rows []*PLAccountRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get by account: %w", err)
	}
	return rows, nil
}

func (r *repository) GetByResponsibility(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLResponsibilityRow, error) {
	query := `
		SELECT
			practitioner_id, period_month,
			payment_responsibility, section_type, pl_section,
			account_code, account_name,
			total_net, total_gst, total_gross,
			entry_count
		FROM vw_pl_by_responsibility
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND period_month >= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND period_month <= DATE_TRUNC('month', $%d::DATE)", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += " ORDER BY period_month ASC, payment_responsibility ASC, pl_section ASC, account_code ASC"

	var rows []*PLResponsibilityRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get by responsibility: %w", err)
	}
	return rows, nil
}

func (r *repository) GetFYSummary(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLFYSummaryRow, error) {
	query := `
		SELECT
			practitioner_id,
			financial_year_id, financial_year,
			financial_quarter_id, quarter,
			income_net, income_gst, income_gross,
			cogs_net, cogs_gst, cogs_gross,
			gross_profit_net,
			other_expenses_net,
			net_profit_net, net_profit_gross
		FROM vw_pl_fy_summary
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FinancialYearID != nil {
		query += fmt.Sprintf(" AND financial_year_id = $%d", idx)
		args = append(args, *f.FinancialYearID)
		idx++
	}

	query += " ORDER BY financial_year ASC, quarter ASC"

	var rows []*PLFYSummaryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get fy summary: %w", err)
	}
	return rows, nil
}
