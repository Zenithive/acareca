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
		FROM vw_balance_sheet_line_items
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

	if f.AsOfDate != nil && *f.AsOfDate != "" {
		query += fmt.Sprintf(" AND submitted_at::DATE <= $%d::DATE", idx)
		args = append(args, *f.AsOfDate)
		idx++
	}

	query = fmt.Sprintf(`
		SELECT
			practitioner_id, clinic_id, account_type, account_code, account_name, coa_id,
			SUM(signed_amount)          AS balance,
			COUNT(DISTINCT entry_id)    AS entry_count,
			TO_CHAR(MAX(submitted_at), 'YYYY-MM-DD') AS last_transaction_date
		FROM (%s) filtered
		GROUP BY practitioner_id, clinic_id, account_type, account_code, account_name, coa_id
		ORDER BY account_type, account_code
	`, query)

	var rows []*BSRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get balance sheet: %w", err)
	}
	return rows, nil
}
