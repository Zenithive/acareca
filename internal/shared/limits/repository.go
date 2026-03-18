package limits

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type repository interface {
	getLimit(ctx context.Context, practitionerID uuid.UUID, key string) (int, error)
	countUsage(ctx context.Context, practitionerID uuid.UUID, key string) (int, error)
}

type repoImpl struct {
	db *sqlx.DB
}

// getLimit fetches the usage_limit for the practitioner's active subscription + permission key.
// Returns ErrNoActiveSubscription if no active subscription row is found.
func (r *repoImpl) getLimit(ctx context.Context, practitionerID uuid.UUID, key string) (int, error) {
	const q = `
		SELECT sp.usage_limit
		FROM tbl_practitioner_subscription ps
		JOIN tbl_subscription_permission sp ON sp.subscription_id = ps.subscription_id
		JOIN tbl_plan_permission pp ON pp.id = sp.permission_id
		WHERE ps.practitioner_id = $1
		  AND ps.status = 'ACTIVE'
		  AND ps.deleted_at IS NULL
		  AND pp.key = $2
		  AND sp.is_enabled = TRUE
		  AND sp.deleted_at IS NULL
		  AND pp.deleted_at IS NULL
		ORDER BY ps.start_date DESC
		LIMIT 1
	`
	var limit int
	err := r.db.QueryRowContext(ctx, q, practitionerID, key).Scan(&limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNoActiveSubscription
		}
		return 0, fmt.Errorf("query limit: %w", err)
	}
	return limit, nil
}

// countUsage returns the current number of resources the practitioner has created
// for the given permission key.
func (r *repoImpl) countUsage(ctx context.Context, practitionerID uuid.UUID, key string) (int, error) {
	var count int
	var err error

	switch key {
	case KeyClinicCreate:
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tbl_clinic
			WHERE practitioner_id = $1 AND deleted_at IS NULL
		`, practitionerID).Scan(&count)

	case KeyFormCreate:
		// tbl_form is scoped to clinic_id, so join through tbl_clinic
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tbl_form f
			JOIN tbl_clinic c ON c.id = f.clinic_id
			WHERE c.practitioner_id = $1
			  AND f.deleted_at IS NULL
			  AND c.deleted_at IS NULL
		`, practitionerID).Scan(&count)

	case KeyTransactionCreate:
		// tbl_form_entry is scoped to clinic_id, join through tbl_clinic
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tbl_form_entry fe
			JOIN tbl_clinic c ON c.id = fe.clinic_id
			WHERE c.practitioner_id = $1
			  AND fe.deleted_at IS NULL
			  AND c.deleted_at IS NULL
		`, practitionerID).Scan(&count)

	case KeyUserInvite:
		// No invite table yet — placeholder returns 0 so limit=0 still blocks correctly
		count = 0

	default:
		return 0, fmt.Errorf("unknown permission key: %q", key)
	}

	if err != nil {
		return 0, fmt.Errorf("count usage for %q: %w", key, err)
	}
	return count, nil
}
