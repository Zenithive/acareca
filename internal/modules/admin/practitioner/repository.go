package practitioner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	ListPractitionersWithSubscriptions(ctx context.Context, f common.Filter, hasActiveSubscription *bool) ([]*RsPractitionerWithSubscription, error)
	UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, lockDate *time.Time) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

var practitionerColumns = map[string]string{
	"id":         "p.id",
	"first_name": "u.first_name",
	"last_name":  "u.last_name",
	"email":      "u.email",
	"created_at": "p.created_at",
}

// Search across practitioner full name and email
var practitionerSearchCols = []string{
	"CONCAT(u.first_name, ' ', u.last_name)",
	"u.email",
}

// ListPractitionersWithSubscriptions retrieves all practitioners with their active subscription details
func (r *repository) ListPractitionersWithSubscriptions(ctx context.Context, f common.Filter, hasActiveSubscription *bool) ([]*RsPractitionerWithSubscription, error) {
	query := r.buildListQuery(f, hasActiveSubscription)

	rows, err := r.db.QueryxContext(ctx, r.db.Rebind(query.SQL), query.Args...)
	if err != nil {
		return nil, fmt.Errorf("list practitioners with subscriptions: %w", err)
	}
	defer rows.Close()

	return r.scanPractitioners(rows)
}

// buildListQuery constructs the SQL query for listing practitioners
func (r *repository) buildListQuery(f common.Filter, hasActiveSubscription *bool) struct {
	SQL  string
	Args []interface{}
} {
	baseQuery := `
		SELECT 
			p.id, p.user_id, p.verified, p.created_at,
			u.email, u.first_name, u.last_name, u.phone,
			ps.id as sub_id, s.name as sub_name, ps.start_date, ps.end_date
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		LEFT JOIN tbl_practitioner_subscription ps ON ps.practitioner_id = p.id 
			AND ps.status = 'ACTIVE' 
			AND ps.deleted_at IS NULL
		LEFT JOIN tbl_subscription s ON s.id = ps.subscription_id AND s.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
	`

	baseQuery = r.applySubscriptionFilter(baseQuery, hasActiveSubscription)
	sql, args := common.BuildQuery(baseQuery, f, practitionerColumns, practitionerSearchCols, false)

	return struct {
		SQL  string
		Args []interface{}
	}{SQL: sql, Args: args}
}

// applySubscriptionFilter adds subscription filter to query
func (r *repository) applySubscriptionFilter(query string, hasActiveSubscription *bool) string {
	if hasActiveSubscription == nil {
		return query
	}

	if *hasActiveSubscription {
		return query + " AND ps.id IS NOT NULL"
	}
	return query + " AND ps.id IS NULL"
}

// scanPractitioners scans database rows into practitioner models
func (r *repository) scanPractitioners(rows *sqlx.Rows) ([]*RsPractitionerWithSubscription, error) {
	var result []*RsPractitionerWithSubscription

	for rows.Next() {
		dbModel, err := r.scanPractitionerRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, dbModel.MapToResponse())
	}

	return result, nil
}

// scanPractitionerRow scans a single row into db model
func (r *repository) scanPractitionerRow(rows *sqlx.Rows) (*dbPractitionerWithSubscription, error) {
	var (
		id        uuid.UUID
		userID    uuid.UUID
		verified  bool
		createdAt time.Time
		email     string
		firstName string
		lastName  string
		phone     *string
		subID     sql.NullInt64
		subName   sql.NullString
		startDate sql.NullTime
		endDate   sql.NullTime
	)

	err := rows.Scan(
		&id, &userID, &verified, &createdAt,
		&email, &firstName, &lastName, &phone,
		&subID, &subName, &startDate, &endDate,
	)
	if err != nil {
		return nil, fmt.Errorf("scan practitioner row: %w", err)
	}

	dbModel := &dbPractitionerWithSubscription{
		ID:        id,
		UserID:    userID,
		Verified:  verified,
		CreatedAt: createdAt,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
	}

	r.mapSubscriptionData(dbModel, subID, subName, startDate, endDate)
	return dbModel, nil
}

// mapSubscriptionData maps subscription data to db model
func (r *repository) mapSubscriptionData(dbModel *dbPractitionerWithSubscription, subID sql.NullInt64, subName sql.NullString, startDate, endDate sql.NullTime) {
	if !subID.Valid {
		return
	}

	subIDInt := int(subID.Int64)
	dbModel.SubID = &subIDInt

	if subName.Valid {
		dbModel.SubName = &subName.String
	}
	if startDate.Valid {
		dbModel.StartDate = &startDate.Time
	}
	if endDate.Valid {
		dbModel.EndDate = &endDate.Time
	}
}

func (r *repository) UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, lockDate *time.Time) error {
	query := `
        UPDATE practitioners 
        SET 
            lock_date = $1, 
            updated_at = NOW() 
        WHERE id = $2
    `

	result, err := r.db.ExecContext(ctx, query, lockDate, practitionerID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("practitioner not found")
	}

	return nil
}
