package bs

import (
	"context"
	"fmt"
	"strings"

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

func (r *repository) GetBalanceSheet(ctx context.Context, practitionerIDs []uuid.UUID, f *BSFilter) ([]*BSRow, error) {
	base := `SELECT * FROM vw_balance_sheet_line_items`

	// Start with practitioner IDs
	conditions := []string{"practitioner_id = ANY($1)"}
	args := []interface{}{practitionerIDs}
	idx := 2

	// Strictly Start and End Date filters
	if f.StartDate != nil && *f.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("date::DATE >= $%d::DATE", idx))
		args = append(args, *f.StartDate)
		idx++
	}

	if f.EndDate != nil && *f.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("date::DATE <= $%d::DATE", idx))
		args = append(args, *f.EndDate)
		idx++
	}

	innerQuery := fmt.Sprintf("%s WHERE %s", base, strings.Join(conditions, " AND "))

	query := fmt.Sprintf(`
		SELECT
			practitioner_id,
			account_type,
			account_code,
			account_name,
			coa_id,
			SUM(signed_amount) AS balance
		FROM (%s) AS filtered
		GROUP BY practitioner_id, account_type, account_code, account_name, coa_id
		ORDER BY account_type, account_code
	`, innerQuery)

	var rows []*BSRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get balance sheet: %w", err)
	}
	return rows, nil
}
