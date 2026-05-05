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
	GetReport(ctx context.Context, practitionerIDs []uuid.UUID, f *PLReportFilter) ([]*PLReportRow, error)
	GetPLSummary(ctx context.Context, practitionerIDs []uuid.UUID, f *PLReportFilter) (*PLSummaryRow, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetMonthlySummary(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLSummaryRow, error) {
	// Query vw_pl_line_items directly and aggregate by clinic_id
	query := `
		WITH section_totals AS (
			SELECT 
				practitioner_id, 
				period_month, 
				section_type,
				SUM(net_amount) AS total_net, 
				SUM(gst_amount) AS total_gst, 
				SUM(gross_amount) AS total_gross,
				SUM(signed_net_amount) AS sg_net_amount,
				SUM(signed_gross_amount) AS sg_gross_amount
			FROM vw_pl_line_items
			WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	// Use transaction_date for filtering (the actual date field from tbl_form_entry)
	if f.FromDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) <= $%d::DATE", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += `
			GROUP BY practitioner_id, period_month, section_type
		)
		SELECT
			practitioner_id, period_month,
			COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gross,
			COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),        0) AS cogs_gross,
			COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'), 0) AS gross_profit_net,
			COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gross,
			COALESCE(SUM(sg_net_amount), 0) AS net_profit_net,
			COALESCE(SUM(sg_gross_amount), 0) AS net_profit_gross
		FROM section_totals
		GROUP BY practitioner_id, period_month
		ORDER BY period_month ASC
	`

	var rows []*PLSummaryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get monthly summary: %w", err)
	}
	return rows, nil
}

func (r *repository) GetByAccount(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLAccountRow, error) {
	// Query vw_pl_line_items directly to access transaction_date field
	query := `
		SELECT
			practitioner_id, period_month,
			pl_section, section_type,
			account_code, account_name, account_type,
			tax_name, tax_rate,
			SUM(net_amount) AS total_net,
			SUM(gst_amount) AS total_gst,
			SUM(gross_amount) AS total_gross,
			SUM(signed_net_amount) AS signed_net,
			SUM(signed_gross_amount) AS signed_gross,
			COUNT(DISTINCT entry_id) AS entry_count
		FROM vw_pl_line_items
		WHERE clinic_id = $1
	`
	args := []any{clinicID}
	idx := 2

	// Use transaction_date for filtering (the actual date field from tbl_form_entry)
	if f.FromDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) <= $%d::DATE", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += ` GROUP BY practitioner_id, period_month, pl_section, section_type, account_code, account_name, account_type, tax_name, tax_rate
		ORDER BY period_month ASC, pl_section ASC, account_code ASC`

	var rows []*PLAccountRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get by account: %w", err)
	}
	return rows, nil
}

func (r *repository) GetByResponsibility(ctx context.Context, clinicID uuid.UUID, f *PLFilter) ([]*PLResponsibilityRow, error) {
	// Query vw_pl_line_items directly to access transaction_date field
	query := `
		SELECT
			practitioner_id, period_month,
			payment_responsibility, section_type, pl_section,
			account_code, account_name,
			SUM(net_amount) AS total_net,
			SUM(gst_amount) AS total_gst,
			SUM(gross_amount) AS total_gross,
			COUNT(DISTINCT entry_id) AS entry_count
		FROM vw_pl_line_items
		WHERE clinic_id = $1
	`
	args := []interface{}{clinicID}
	idx := 2

	// Use transaction_date for filtering (the actual date field from tbl_form_entry)
	if f.FromDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND COALESCE(transaction_date, date::DATE) <= $%d::DATE", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += ` GROUP BY practitioner_id, period_month, payment_responsibility, section_type, pl_section, account_code, account_name
		ORDER BY period_month ASC, payment_responsibility ASC, pl_section ASC, account_code ASC`

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

func (r *repository) GetReport(ctx context.Context, practitionerIDs []uuid.UUID, f *PLReportFilter) ([]*PLReportRow, error) {
	query := `
		SELECT
			COALESCE(li.clinic_id::TEXT, '') AS clinic_id,
			COALESCE(c.name, 'Manual Expense') AS clinic_name,
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
		LEFT JOIN tbl_clinic c ON c.id = li.clinic_id AND c.deleted_at IS NULL
		WHERE li.practitioner_id IN (?)
	`
	args := []interface{}{practitionerIDs}

	if f.ClinicID != nil && *f.ClinicID != "" {
		// We match the selected ClinicID OR the Zero UUID (Manual Expenses)
		zeroUUID := "00000000-0000-0000-0000-000000000000"

		query += " AND (li.clinic_id = ? OR li.clinic_id = ? OR li.clinic_id IS NULL)"
		args = append(args, *f.ClinicID, zeroUUID)
	}

	if f.DateFrom != nil && *f.DateFrom != "" {
		query += " AND li.date::DATE >= ?::DATE"
		args = append(args, *f.DateFrom)
	}
	if f.DateUntil != nil && *f.DateUntil != "" {
		query += " AND li.date::DATE <= ?::DATE"
		args = append(args, *f.DateUntil)
	}
	if f.CoaID != nil {
		query += " AND li.coa_id = ?"
		args = append(args, *f.CoaID)
	}
	if f.TaxTypeID != nil {
		query += " AND li.tax_name = ?"
		args = append(args, *f.TaxTypeID)
	}
	if f.FormID != nil {
		query += " AND li.form_id = ?"
		args = append(args, *f.FormID)
	}

	query += `
		GROUP BY
			li.clinic_id, c.name, li.form_id, li.form_name,
			li.form_field_id, li.field_label, li.section_type,
			li.coa_id, li.account_name, li.tax_name
		ORDER BY li.section_type, li.account_name
	`

	fullQuery, fullArgs, err := sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}

	finalQuery := r.db.Rebind(fullQuery)

	var rows []*PLReportRow
	if err := r.db.SelectContext(ctx, &rows, finalQuery, fullArgs...); err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	return rows, nil
}

func (r *repository) GetPLSummary(ctx context.Context, practitionerIDs []uuid.UUID, f *PLReportFilter) (*PLSummaryRow, error) {
	query := `
		SELECT 
			COALESCE(SUM(net_profit_net), 0)   AS net_profit_net,
			COALESCE(SUM(gross_profit_net), 0) AS gross_profit_net
		FROM vw_pl_summary_monthly
		WHERE practitioner_id IN (?)
	`
	args := []interface{}{practitionerIDs}

	if f.ClinicID != nil && *f.ClinicID != "" {
		zeroUUID := "00000000-0000-0000-0000-000000000000"
		query += " AND (clinic_id = ? OR clinic_id = ? OR clinic_id IS NULL)"
		args = append(args, *f.ClinicID, zeroUUID)
	}

	// Use DATE_TRUNC on the input to ensure we match the '2026-04-01' format in the view
	if f.DateFrom != nil && *f.DateFrom != "" {
		query += " AND period_month >= DATE_TRUNC('month', ?::DATE)"
		args = append(args, *f.DateFrom)
	}

	if f.DateUntil != nil && *f.DateUntil != "" {
		// We use the last day of the month for the Until comparison to ensure the selected month is included
		query += " AND period_month <= DATE_TRUNC('month', ?::DATE)"
		args = append(args, *f.DateUntil)
	}

	fullQuery, fullArgs, err := sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}

	finalQuery := r.db.Rebind(fullQuery)

	var summary PLSummaryRow
	if err := r.db.GetContext(ctx, &summary, finalQuery, fullArgs...); err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}
	return &summary, nil
}
