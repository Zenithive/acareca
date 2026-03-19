package bas

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all DB queries for the BAS module.
type Repository interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASSummaryRow, error)

	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASByAccountRow, error)

	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASMonthlyRow, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASSummaryRow, error) {
	query := `
		SELECT
			clinic_id,
			practitioner_id,
			period_quarter,
			period_year,
			g1_total_sales_gross,
			g3_gst_free_sales,
			g8_taxable_sales,
			label_1a_gst_on_sales,
			g11_total_purchases_gross,
			g14_gst_free_purchases,
			g15_taxable_purchases,
			label_1b_gst_on_purchases,
			net_gst_payable,
			total_sales_net,
			total_purchases_net
		FROM vw_bas_summary
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND period_quarter >= DATE_TRUNC('quarter', $%d::DATE)", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND period_quarter <= DATE_TRUNC('quarter', $%d::DATE)", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	// Filter by financial year via a join on tbl_financial_year date range
	if f.FinancialYearID != nil {
		query += fmt.Sprintf(`
			AND period_quarter BETWEEN (
				SELECT DATE_TRUNC('quarter', start_date) FROM tbl_financial_year WHERE id = $%d
			) AND (
				SELECT DATE_TRUNC('quarter', end_date)   FROM tbl_financial_year WHERE id = $%d
			)`, idx, idx)
		args = append(args, *f.FinancialYearID)
		idx++
	}

	query += " ORDER BY period_year ASC, period_quarter ASC"

	var rows []*BASSummaryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get bas quarterly summary: %w", err)
	}
	return rows, nil
}

func (r *repository) GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASByAccountRow, error) {
	query := `
		SELECT
			clinic_id,
			practitioner_id,
			period_quarter,
			period_year,
			section_type,
			bas_category,
			account_code,
			account_name,
			tax_name,
			tax_rate,
			entry_count,
			total_net,
			total_gst,
			total_gross
		FROM vw_bas_by_account
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND period_quarter >= DATE_TRUNC('quarter', $%d::DATE)", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND period_quarter <= DATE_TRUNC('quarter', $%d::DATE)", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	if f.FinancialYearID != nil {
		query += fmt.Sprintf(`
			AND period_quarter BETWEEN (
				SELECT DATE_TRUNC('quarter', start_date) FROM tbl_financial_year WHERE id = $%d
			) AND (
				SELECT DATE_TRUNC('quarter', end_date)   FROM tbl_financial_year WHERE id = $%d
			)`, idx, idx)
		args = append(args, *f.FinancialYearID)
		idx++
	}

	query += " ORDER BY period_year ASC, period_quarter ASC, section_type ASC, account_code ASC"

	var rows []*BASByAccountRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get bas by account: %w", err)
	}
	return rows, nil
}

func (r *repository) GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASMonthlyRow, error) {
	query := `
		SELECT
			clinic_id,
			practitioner_id,
			period_month,
			g1_total_sales_gross,
			g3_gst_free_sales,
			label_1a_gst_on_sales,
			g11_total_purchases_gross,
			g14_gst_free_purchases,
			label_1b_gst_on_purchases,
			net_gst_payable,
			total_sales_net,
			total_purchases_net
		FROM vw_bas_monthly
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

	var rows []*BASMonthlyRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get bas monthly: %w", err)
	}
	return rows, nil
}
