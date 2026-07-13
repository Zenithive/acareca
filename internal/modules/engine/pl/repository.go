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
	query := `
		WITH section_totals AS (
			SELECT 
				practitioner_id, 
				period_month, 
				account_type,
				pl_section,
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

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND date::DATE >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND date::DATE <= $%d::DATE", idx)
		args = append(args, *f.ToDate)
		idx++
	}

	query += `
			GROUP BY practitioner_id, period_month, account_type, pl_section
		)
		SELECT
			practitioner_id, period_month,
			COALESCE(SUM(total_net)   FILTER (WHERE account_type = 'Revenue'),  0) AS income_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE account_type = 'Revenue'),  0) AS income_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE account_type = 'Revenue'),  0) AS income_gross,
			COALESCE(SUM(total_net)   FILTER (WHERE pl_section = '2. Cost of Sales'),  0) AS cogs_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE pl_section = '2. Cost of Sales'),  0) AS cogs_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE pl_section = '2. Cost of Sales'),  0) AS cogs_gross,
			COALESCE(SUM(total_net) FILTER (WHERE account_type = 'Revenue'), 0) - COALESCE(SUM(total_net) FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS gross_profit_net,
			COALESCE(SUM(total_net)   FILTER (WHERE pl_section = '3. Other Expenses'),  0) AS other_expenses_net,
			COALESCE(SUM(total_gst)   FILTER (WHERE pl_section = '3. Other Expenses'),  0) AS other_expenses_gst,
			COALESCE(SUM(total_gross) FILTER (WHERE pl_section = '3. Other Expenses'),  0) AS other_expenses_gross,
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

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND date::DATE >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND date::DATE <= $%d::DATE", idx)
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

	if f.FromDate != nil {
		query += fmt.Sprintf(" AND date::DATE >= $%d::DATE", idx)
		args = append(args, *f.FromDate)
		idx++
	}
	if f.ToDate != nil {
		query += fmt.Sprintf(" AND date::DATE <= $%d::DATE", idx)
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
			COALESCE(v.clinic_id::TEXT, '') AS clinic_id,
			COALESCE(c.name, 'Manual Expense') AS clinic_name,
			v.form_id::TEXT,
			v.form_name,
			v.form_field_id::TEXT,
			v.field_label,
			v.section_type::TEXT,
			v.account_type,
			v.pl_section,
			v.coa_id::TEXT,
			v.account_name,
			v.tax_name,
			SUM(v.net_amount)   AS net_amount,
			SUM(v.gst_amount)   AS gst_amount,
			SUM(v.gross_amount) AS gross_amount
		FROM vw_pl_line_items v
		LEFT JOIN tbl_clinic c ON c.id = v.clinic_id AND c.deleted_at IS NULL
		LEFT JOIN tbl_account_tax t ON t.name = v.tax_name
		WHERE v.practitioner_id IN (?)
	`
	args := []interface{}{practitionerIDs}

	if f.ClinicID != nil && *f.ClinicID != "" {
		zeroUUID := "00000000-0000-0000-0000-000000000000"
		query += " AND (v.clinic_id = ? OR v.clinic_id = ? OR v.clinic_id IS NULL)"
		args = append(args, *f.ClinicID, zeroUUID)
	}

	if f.DateFrom != nil && *f.DateFrom != "" {
		query += " AND v.date::DATE >= ?::DATE"
		args = append(args, *f.DateFrom)
	}

	if f.DateUntil != nil && *f.DateUntil != "" {
		query += " AND v.date::DATE <= ?::DATE"
		args = append(args, *f.DateUntil)
	}

	if f.CoaID != nil && *f.CoaID != "" {
		query += " AND v.coa_id = ?"
		args = append(args, *f.CoaID)
	}

	if f.FormID != nil && *f.FormID != "" {
		query += " AND v.form_id = ?"
		args = append(args, *f.FormID)
	}

	if f.TaxTypeID != nil && *f.TaxTypeID != "" {
		query += " AND t.id = ?"
		args = append(args, *f.TaxTypeID)
	}

	query += `
		GROUP BY
			v.clinic_id, c.name, v.form_id, v.form_name,
			v.form_field_id, v.field_label, v.section_type,
			v.account_type, v.pl_section,
			v.coa_id, v.account_name, v.tax_name
		ORDER BY v.pl_section, v.account_name
	`

	fullQuery, fullArgs, err := sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}

	finalQuery := r.db.Rebind(fullQuery)

	var rows []*PLReportRow
	if err := r.db.SelectContext(ctx, &rows, finalQuery, fullArgs...); err != nil {
		return nil, fmt.Errorf("get report rows: %w", err)
	}
	return rows, nil
}

func (r *repository) GetPLSummary(ctx context.Context, practitionerIDs []uuid.UUID, f *PLReportFilter) (*PLSummaryRow, error) {
	query := `
		WITH filtered_items AS (
			SELECT
				v.practitioner_id,
				v.account_type,
				v.pl_section,
				SUM(v.net_amount) AS total_net,
				SUM(v.signed_net_amount) AS sg_net_amount
			FROM vw_pl_line_items v
			LEFT JOIN tbl_account_tax t ON t.name = v.tax_name
			WHERE v.practitioner_id IN (?)
	`
	args := []interface{}{practitionerIDs}

	if f.ClinicID != nil && *f.ClinicID != "" {
		zeroUUID := "00000000-0000-0000-0000-000000000000"
		query += " AND (v.clinic_id = ? OR v.clinic_id = ? OR v.clinic_id IS NULL)"
		args = append(args, *f.ClinicID, zeroUUID)
	}

	if f.DateFrom != nil && *f.DateFrom != "" {
		query += " AND v.date::DATE >= ?::DATE"
		args = append(args, *f.DateFrom)
	}

	if f.DateUntil != nil && *f.DateUntil != "" {
		query += " AND v.date::DATE <= ?::DATE"
		args = append(args, *f.DateUntil)
	}

	if f.CoaID != nil && *f.CoaID != "" {
		query += " AND v.coa_id = ?"
		args = append(args, *f.CoaID)
	}

	if f.FormID != nil && *f.FormID != "" {
		query += " AND v.form_id = ?"
		args = append(args, *f.FormID)
	}

	if f.TaxTypeID != nil && *f.TaxTypeID != "" {
		query += " AND t.id = ?"
		args = append(args, *f.TaxTypeID)
	}

	query += `
			GROUP BY v.practitioner_id, v.account_type, v.pl_section
		)
		SELECT
			COALESCE(SUM(total_net) FILTER (WHERE account_type = 'Revenue'), 0) - COALESCE(SUM(total_net) FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS gross_profit_net,
			COALESCE(SUM(sg_net_amount), 0) AS net_profit_net
		FROM filtered_items
	`

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
