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
	GetReport(ctx context.Context, practitionerID uuid.UUID, from, to string) (*BASReportRow, error)
	GetQuarterDates(ctx context.Context, quarterID uuid.UUID) (start, end string, err error)

	GetBASLineItems(ctx context.Context, practitionerIDs []uuid.UUID, clinicID *uuid.UUID, f *BASFilter) ([]*BASLineItemRow, error)
	GetQuarterInfoByDate(ctx context.Context, date time.Time) (*BASQuarterInfo, error)
	GetQuarterInfoByID(ctx context.Context, id uuid.UUID) (*BASQuarterInfo, error)
	GetAllQuartersInYear(ctx context.Context, quarterID uuid.UUID) ([]BASQuarterInfo, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
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
            -- Sum everything from the line items directly
            COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS g1_total_sales_gross,
            COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION'), 0) AS label_1a_gst_on_sales,
            
            -- This will capture your $220 regardless of the tax mismatch
            COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST', 'EXPENSE_ENTRY')), 0) AS g11_total_purchases_gross,
            COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST', 'EXPENSE_ENTRY')), 0) AS label_1b_gst_on_purchases
        FROM vw_bas_line_items
        WHERE practitioner_id = $1
          AND submitted_at::DATE >= $2::DATE
          AND submitted_at::DATE <= $3::DATE
    `
	var row BASReportRow
	if err := r.db.QueryRowxContext(ctx, query, practitionerID, from, to).StructScan(&row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *repository) GetBASLineItems(ctx context.Context, practitionerIDs []uuid.UUID, clinicID *uuid.UUID, f *BASFilter) ([]*BASLineItemRow, error) {
	query := `
        SELECT 
            period_quarter,
            section_type,
            bas_category,
            coa_id,
            account_name,
            SUM(net_amount) AS net_amount,
            SUM(gst_amount) AS gst_amount,
            SUM(gross_amount) AS gross_amount
        FROM vw_bas_line_items
        WHERE practitioner_id IN (?)
    `
	args := []interface{}{practitionerIDs}

	// Note: clinicID parameter is ignored since we removed clinic_id from the view
	// Keeping the parameter for backward compatibility but not using it

	if len(f.ParsedQuarterIDs) > 0 {
		query += ` AND period_quarter >= (SELECT MIN(start_date) FROM tbl_financial_quarter WHERE id IN (?))
               AND period_quarter <= (SELECT MAX(end_date) FROM tbl_financial_quarter WHERE id IN (?))`

		args = append(args, f.ParsedQuarterIDs, f.ParsedQuarterIDs)
	}

	// Handle Financial Year (Fall-through logic)
	if len(f.ParsedQuarterIDs) == 0 && f.FinancialYearID != nil {
		query += ` AND period_quarter BETWEEN (
                SELECT start_date FROM tbl_financial_year WHERE id = ?
            ) AND (
                SELECT end_date FROM tbl_financial_year WHERE id = ?
            )`

		args = append(args, *f.FinancialYearID, *f.FinancialYearID)
	}

	query += ` GROUP BY period_quarter, section_type, bas_category, coa_id, account_name 
               ORDER BY period_quarter ASC`

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

func (r *repository) GetAllQuartersInYear(ctx context.Context, financialYearID uuid.UUID) ([]BASQuarterInfo, error) {
	var list []BASQuarterInfo

	query := `
        SELECT 
            id::text, 
            label as name, 
            TO_CHAR(start_date, 'YYYY-MM-DD') as startDate,
            TO_CHAR(end_date, 'YYYY-MM-DD') as endDate,
            TO_CHAR(start_date, 'Mon') || ' - ' || TO_CHAR(end_date, 'Mon') as displayRange
        FROM tbl_financial_quarter 
        WHERE financial_year_id = $1
        ORDER BY start_date ASC
		`
	if err := r.db.SelectContext(ctx, &list, query, financialYearID); err != nil {
		return nil, err
	}
	return list, nil
}
