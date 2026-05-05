package bs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all DB queries for the Balance Sheet module
type Repository interface {
	GetBalanceSheet(ctx context.Context, practitionerIDs []uuid.UUID, f *BSFilter) ([]*BSRow, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// repository.go — updated GetBalanceSheet
func (r *repository) GetBalanceSheet(ctx context.Context, practitionerIDs []uuid.UUID, f *BSFilter) ([]*BSRow, error) {
	// Step 1 — build the inner filter query first, with all WHERE conditions
	inner := `
		SELECT *
		FROM vw_balance_sheet_line_items
		WHERE practitioner_id = ANY($1)
	`
	args := []interface{}{practitionerIDs}
	idx := 2

	// if f.ClinicID != nil && *f.ClinicID != "" {
	// 	clinicID, err := uuid.Parse(*f.ClinicID)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("invalid clinic_id: %w", err)
	// 	}
	// 	inner += fmt.Sprintf(" AND clinic_id = $%d", idx)
	// 	args = append(args, clinicID)
	// 	idx++
	// }

	if f.ClinicID != nil && *f.ClinicID != "" {
		clinicID, err := uuid.Parse(*f.ClinicID)
		if err != nil {
			return nil, fmt.Errorf("invalid clinic_id: %w", err)
		}
		inner += fmt.Sprintf(" AND (clinic_id = $%d OR clinic_id IS NULL)", idx)
		args = append(args, clinicID)
		idx++
	}

	if f.AsOfDate != nil && *f.AsOfDate != "" {
		inner += fmt.Sprintf(" AND date::DATE <= $%d::DATE", idx)
		args = append(args, *f.AsOfDate)
		idx++
	}

	if f.FormID != nil && *f.FormID != "" {
		inner += fmt.Sprintf(" AND form_id = $%d", idx)
		args = append(args, *f.FormID)
		idx++
	}

	if f.CoaID != nil && *f.CoaID != "" {
		inner += fmt.Sprintf(" AND coa_id = $%d", idx)
		args = append(args, *f.CoaID)
		idx++
	}

	if f.TaxTypeID != nil && *f.TaxTypeID != 0 {
		inner += fmt.Sprintf(" AND tax_id = $%d", idx)
		args = append(args, *f.TaxTypeID)
		idx++
	}
	// Step 2 — wrap once with the outer aggregation AFTER all filters are applied
	query := fmt.Sprintf(`
		SELECT
			practitioner_id,
			account_type,
			account_code,
			account_name,
			coa_id,
			SUM(signed_amount)                           AS balance,
			COUNT(DISTINCT entry_id)                     AS entry_count,
			COALESCE(TO_CHAR(MAX(date), 'YYYY-MM-DD'), '') AS last_transaction_date
		FROM (%s) filtered
		GROUP BY practitioner_id, account_type, account_code, account_name, coa_id
		ORDER BY account_type, account_code
	`, inner)

	var rows []*BSRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get balance sheet: %w", err)
	}
	return rows, nil
}
