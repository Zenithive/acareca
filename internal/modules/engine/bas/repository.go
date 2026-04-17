package bas

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all DB queries for the BAS module.
type Repository interface {
	GetQuarterlySummary(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASSummaryRow, error)
	GetByAccount(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASByAccountRow, error)
	GetMonthly(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASMonthlyRow, error)
	GetReport(ctx context.Context, practitionerID uuid.UUID, from, to string) (*BASReportRow, error)
	GetQuarterDates(ctx context.Context, quarterID uuid.UUID) (start, end string, err error)

	GetBASLineItems(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASLineItemRow, error)
	GetQuarterInfoByDate(ctx context.Context, date time.Time) (*BASQuarterInfo, error)
	GetQuarterInfoByID(ctx context.Context, id uuid.UUID) (*BASQuarterInfo, error)
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

func (r *repository) GetQuarterDates(ctx context.Context, quarterID uuid.UUID) (string, string, error) {
	var start, end string
	err := r.db.QueryRowContext(ctx,
		`SELECT TO_CHAR(start_date, 'YYYY-MM-DD'), TO_CHAR(end_date, 'YYYY-MM-DD')
		 FROM tbl_financial_quarter WHERE id = $1`, quarterID,
	).Scan(&start, &end)
	if err != nil {
		return "", "", fmt.Errorf("get quarter dates: %w", err)
	}
	return start, end, nil
}

func (r *repository) GetReport(ctx context.Context, practitionerID uuid.UUID, from, to string) (*BASReportRow, error) {
	query := `
		SELECT
			COALESCE(SUM(g1_total_sales_gross), 0)       AS g1_total_sales_gross,
			COALESCE(SUM(label_1a_gst_on_sales), 0)      AS label_1a_gst_on_sales,
			COALESCE(SUM(g11_total_purchases_gross), 0)  AS g11_total_purchases_gross,
			COALESCE(SUM(label_1b_gst_on_purchases), 0)  AS label_1b_gst_on_purchases
		FROM vw_bas_summary
		WHERE practitioner_id = $1
		  AND period_quarter >= DATE_TRUNC('quarter', $2::DATE)
		  AND period_quarter <= DATE_TRUNC('quarter', $3::DATE)
	`
	var row BASReportRow

	if err := r.db.QueryRowxContext(ctx, query, practitionerID, from, to).StructScan(&row); err != nil {
		return nil, fmt.Errorf("get bas report: %w", err)
	}
	return &row, nil
}

func (r *repository) GetBASLineItems(ctx context.Context, clinicID uuid.UUID, f *BASFilter) ([]*BASLineItemRow, error) {
	// 1. Use ? instead of $1
	query := `
        SELECT 
            period_quarter,
            section_type,
            bas_category,
			account_name,
            SUM(net_amount) AS net_amount,
            SUM(gst_amount) AS gst_amount,
            SUM(gross_amount) AS gross_amount
        FROM vw_bas_line_items
        WHERE clinic_id = ?
    `
	args := []interface{}{clinicID}

	// 2. Handle Quarter IDs
	if len(f.parsedQuarterIDs) > 0 {
		// sqlx.In replaces (?) with (?, ?, ?)
		subQuery, qArgs, err := sqlx.In(" AND period_quarter IN (SELECT start_date FROM tbl_financial_quarter WHERE id IN (?))", f.QuarterIDs)
		if err == nil {
			query += subQuery
			args = append(args, qArgs...)
		}
	}

	// 3. Handle Financial Year (Fall-through logic)
	if len(f.parsedQuarterIDs) == 0 && f.FinancialYearID != nil {
		query += ` AND period_quarter BETWEEN (
                SELECT start_date FROM tbl_financial_year WHERE id = ?
            ) AND (
                SELECT end_date FROM tbl_financial_year WHERE id = ?
            )`
		// We add the ID twice because there are two '?' placeholders
		args = append(args, *f.FinancialYearID, *f.FinancialYearID)
	}

	query += ` GROUP BY period_quarter, section_type, bas_category, account_name 
               ORDER BY period_quarter ASC`

	// --- THE CRITICAL STEP ---
	// sqlx.In returns a query with '?' placeholders.
	// Rebind(?) converts all '?' to '$1', '$2', '$3' automatically.
	fullQuery, fullArgs, err := sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}

	finalQuery := r.db.Rebind(fullQuery)

	var rows []*BASLineItemRow
	if err := r.db.SelectContext(ctx, &rows, finalQuery, fullArgs...); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetQuarterInfoByDate fetches metadata for the "quarter" object in your JSON
func (r *repository) GetQuarterInfoByDate(ctx context.Context, date time.Time) (*BASQuarterInfo, error) {
	var info BASQuarterInfo
	query := `
        SELECT 
            id::text, 
            label as name, 
            TO_CHAR(start_date, 'YYYY-MM-DD') as startDate,
            TO_CHAR(end_date, 'YYYY-MM-DD') as endDate,
            TO_CHAR(start_date, 'Mon') || ' - ' || TO_CHAR(end_date, 'Mon') as displayRange
        FROM tbl_financial_quarter 
		WHERE start_date = $1 
   OR ($1 BETWEEN start_date AND end_date)
        LIMIT 1
    `
	if err := r.db.GetContext(ctx, &info, query, date); err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *repository) GetQuarterInfoByID(ctx context.Context, id uuid.UUID) (*BASQuarterInfo, error) {
	var info BASQuarterInfo
	query := `
        SELECT 
            id::text, 
            label as name, 
            TO_CHAR(start_date, 'YYYY-MM-DD') as startDate,
            TO_CHAR(end_date, 'YYYY-MM-DD') as endDate,
            TO_CHAR(start_date, 'Mon') || ' - ' || TO_CHAR(end_date, 'Mon') as displayRange
        FROM tbl_financial_quarter 
        WHERE id = $1
    `
	if err := r.db.GetContext(ctx, &info, query, id); err != nil {
		return nil, err
	}
	return &info, nil
}
