package bs

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetBalanceSheet(ctx context.Context, practitionerIDs []uuid.UUID, f *BSFilter) ([]*BSRow, error)
	GetBalanceSheetGst(ctx context.Context, f common.Filter, endDate string, actorID uuid.UUID, role string) (*RsBalanceSheetGst, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetBalanceSheet(ctx context.Context, practitionerIDs []uuid.UUID, f *BSFilter) ([]*BSRow, error) {
	base := `SELECT * FROM vw_balance_sheet_line_items`
	conditions := []string{"practitioner_id = ANY($1)"}
	args := []interface{}{practitionerIDs}
	idx := 2

	// Apply end date filter to transaction dates
	if f.EndDate != nil && *f.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("date::DATE <= $%d::DATE", idx))
		args = append(args, *f.EndDate)
		idx++
	}

	// Apply user_id filter to transactions submitted by a specific user
	if f.UserID != nil && *f.UserID != "" {
		userID, err := uuid.Parse(*f.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, userID)
		idx++
	}

	innerQuery := fmt.Sprintf("%s WHERE %s", base, strings.Join(conditions, " AND "))

	// Aggregate by account to get totals across all filtered transactions
	query := fmt.Sprintf(`
		SELECT
			practitioner_id,
			account_type,
			account_code,
			account_name,
			coa_id,
			SUM(signed_amount) AS balance
		FROM (%s) AS filtered
		WHERE account_type IN ('Asset', 'Liability', 'Equity')
		GROUP BY practitioner_id, account_type, account_code, account_name, coa_id
		ORDER BY account_type, account_code
	`, innerQuery)

	var rows []*BSRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get balance sheet: %w", err)
	}
	return rows, nil
}

func (r *repository) GetBalanceSheetGst(ctx context.Context, f common.Filter, endDate string, practitionerID uuid.UUID, role string) (*RsBalanceSheetGst, error) {
	query := `
		SELECT 
			COALESCE(SUM(
				CASE 
					WHEN v.normal_balance = 'CREDIT' THEN fev.gst_amount
					WHEN v.normal_balance = 'DEBIT'  THEN -fev.gst_amount
					ELSE fev.gst_amount
				END
			), 0.0)::float8 AS total_gst_payable
		FROM vw_double_entry_line_items v
		INNER JOIN tbl_form_entry_value fev 
			ON fev.entry_id = v.entry_id 
			AND fev.deleted_at IS NULL 
			AND fev.updated_at IS NULL
			AND (
				(v.form_field_id IS NOT NULL AND fev.form_field_id = v.form_field_id)
				OR 
				(v.form_field_id IS NULL AND fev.form_field_id IS NULL AND fev.coa_id = v.coa_id)
			)
		WHERE 1=1
	`

	args := []interface{}{}
	argPos := 1

	if practitionerID != uuid.Nil {
		query += fmt.Sprintf(" AND v.practitioner_id = $%d", argPos)
		args = append(args, practitionerID)
		argPos++
	}

	if endDate != "" {
		query += fmt.Sprintf(" AND v.entry_date <= $%d", argPos)
		args = append(args, endDate)
		argPos++
	}

	var totalGst float64
	err := r.db.GetContext(ctx, &totalGst, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gst total: %w", err)
	}

	res := &RsBalanceSheetGst{
		TotalGstPayable: math.Round(totalGst*100) / 100,
	}

	if res.TotalGstPayable >= 0 {
		res.AccountType = "CURRENT_LIABILITY"
	} else {
		res.AccountType = "CURRENT_ASSET"
		res.TotalGstPayable = math.Abs(res.TotalGstPayable)
	}

	return res, nil
}
