package accountant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	ListAccountantsWithPractitioners(ctx context.Context, f common.Filter) ([]*RsAccountantWithPractitioners, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

var accountantColumns = map[string]string{
	"id":         "a.id",
	"first_name": "u.first_name",
	"last_name":  "u.last_name",
	"email":      "u.email",
}

// Search across accountant full name and email
var accountantSearchCols = []string{
	"CONCAT(u.first_name, ' ', u.last_name)",
	"u.email",
}

// ListAccountantsWithPractitioners retrieves all accountants with their associated practitioners
func (r *repository) ListAccountantsWithPractitioners(ctx context.Context, f common.Filter) ([]*RsAccountantWithPractitioners, error) {
	query := r.buildListQuery(f)

	rows, err := r.db.QueryxContext(ctx, r.db.Rebind(query.SQL), query.Args...)
	if err != nil {
		return nil, fmt.Errorf("list accountants with practitioners: %w", err)
	}
	defer rows.Close()

	return r.scanAccountants(rows)
}

// buildListQuery constructs the SQL query for listing accountants
func (r *repository) buildListQuery(f common.Filter) struct{ SQL string; Args []interface{} } {
	baseQuery := `
		SELECT 
			a.id, a.user_id,
			u.email, u.first_name, u.last_name, u.phone,
			p.id as practitioner_id,
			pu.email as prac_email,
			pu.first_name as prac_first_name,
			pu.last_name as prac_last_name,
			pu.phone as prac_phone
		FROM tbl_accountant a
		JOIN tbl_user u ON u.id = a.user_id AND u.deleted_at IS NULL
		LEFT JOIN tbl_invitation i ON i.accountant_id = a.id AND i.status = 'COMPLETED'
		LEFT JOIN tbl_practitioner p ON p.id = i.practitioner_id AND p.deleted_at IS NULL
		LEFT JOIN tbl_user pu ON pu.id = p.user_id AND pu.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
	`

	sql, args := common.BuildQuery(baseQuery, f, accountantColumns, accountantSearchCols, false)

	return struct{ SQL string; Args []interface{} }{SQL: sql, Args: args}
}

// scanAccountants scans database rows and groups practitioners by accountant
func (r *repository) scanAccountants(rows *sqlx.Rows) ([]*RsAccountantWithPractitioners, error) {
	accountantMap := make(map[uuid.UUID]*RsAccountantWithPractitioners)
	var orderedIDs []uuid.UUID

	for rows.Next() {
		dbModel, err := r.scanAccountantRow(rows)
		if err != nil {
			return nil, err
		}

		// Check if accountant already exists in map
		if _, exists := accountantMap[dbModel.ID]; !exists {
			accountantMap[dbModel.ID] = &RsAccountantWithPractitioners{
				ID:            dbModel.UserID,
				Name:          dbModel.FirstName + " " + dbModel.LastName,
				Email:         dbModel.Email,
				Phone:         dbModel.Phone,
				Practitioners: []PractitionerInfo{},
			}
			orderedIDs = append(orderedIDs, dbModel.ID)
		}

		// Add practitioner if exists
		if dbModel.PractitionerID.Valid {
			practitionerID, _ := uuid.Parse(dbModel.PractitionerID.String)
			practitioner := PractitionerInfo{
				ID:    practitionerID,
				Name:  dbModel.PracFirstName.String + " " + dbModel.PracLastName.String,
				Email: dbModel.PracEmail.String,
			}
			if dbModel.PracPhone.Valid {
				phone := dbModel.PracPhone.String
				practitioner.Phone = &phone
			}
			accountantMap[dbModel.ID].Practitioners = append(accountantMap[dbModel.ID].Practitioners, practitioner)
		}
	}

	// Convert map to ordered slice
	result := make([]*RsAccountantWithPractitioners, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		result = append(result, accountantMap[id])
	}

	return result, nil
}

// scanAccountantRow scans a single row into db model
func (r *repository) scanAccountantRow(rows *sqlx.Rows) (*dbAccountantWithPractitioners, error) {
	var dbModel dbAccountantWithPractitioners

	err := rows.Scan(
		&dbModel.ID, &dbModel.UserID,
		&dbModel.Email, &dbModel.FirstName, &dbModel.LastName, &dbModel.Phone,
		&dbModel.PractitionerID, &dbModel.PracEmail, &dbModel.PracFirstName, &dbModel.PracLastName, &dbModel.PracPhone,
	)
	if err != nil {
		return nil, fmt.Errorf("scan accountant row: %w", err)
	}

	return &dbModel, nil
}
