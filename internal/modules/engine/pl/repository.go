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
	GetReport(ctx context.Context, f *PLReportFilter) ([]*PLReportRow, error)
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
	args := []any{clinicID}
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

func (r *repository) GetReport(ctx context.Context, f *PLReportFilter) ([]*PLReportRow, error) {
	query := `
		SELECT
			li.clinic_id::TEXT,
			c.name        AS clinic_name,
			li.form_id::TEXT,
			li.form_name,
			li.form_field_id::TEXT,
			li.field_label,
			li.section_type::TEXT,
			li.coa_id::TEXT,
			li.account_name,
			li.tax_name,
			SUM(li.net_amount)   AS net_amount,
			SUM(li.gst_amount)   AS gst_amount,
			SUM(li.gross_amount) AS gross_amount
		FROM vw_pl_line_items li
		JOIN tbl_clinic c ON c.id = li.clinic_id AND c.deleted_at IS NULL
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1

	if f.ClinicID != nil {
		query += fmt.Sprintf(" AND li.clinic_id = $%d", idx)
		args = append(args, *f.ClinicID)
		idx++
	} else {
		// scope to practitioner via the view's practitioner_id column
		query += fmt.Sprintf(" AND li.practitioner_id = $%d", idx)
		args = append(args, f.PractitionerID)
		idx++
	}

	if f.DateFrom != nil {
		query += fmt.Sprintf(" AND li.submitted_at::DATE >= $%d::DATE", idx)
		args = append(args, *f.DateFrom)
		idx++
	}
	if f.DateUntil != nil {
		query += fmt.Sprintf(" AND li.submitted_at::DATE <= $%d::DATE", idx)
		args = append(args, *f.DateUntil)
		idx++
	}
	if f.CoaID != nil {
		query += fmt.Sprintf(" AND li.coa_id = $%d", idx)
		args = append(args, *f.CoaID)
		idx++
	}
	if f.TaxTypeID != nil {
		query += fmt.Sprintf(" AND li.tax_name = $%d", idx)
		args = append(args, *f.TaxTypeID)
		idx++
	}
	if f.FormID != nil {
		query += fmt.Sprintf(" AND li.form_id = $%d", idx)
		args = append(args, *f.FormID)
		idx++
	}

	query += `
		GROUP BY
			li.clinic_id, c.name, li.form_id, li.form_name,
			li.form_field_id, li.field_label, li.section_type,
			li.coa_id, li.account_name, li.tax_name
		ORDER BY li.section_type, li.account_name
	`

	var rows []*PLReportRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	return rows, nil
}
