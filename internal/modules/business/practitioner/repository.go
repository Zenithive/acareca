package practitioner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var errNotFound = errors.New("practitioner not found")

type Repository interface {
	CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error)
	GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error)
	DeletePractitioner(ctx context.Context, id uuid.UUID) error
	ListPractitioners(ctx context.Context, f common.Filter) ([]*PractitionerWithUser, error)
	ListPractitionersForAccountant(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]*PractitionerWithUser, error)
	GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error)
	CountPractitioners(ctx context.Context, f common.Filter) (int, error)
	CountPractitionersForAccountant(ctx context.Context, accountantID uuid.UUID, f common.Filter) (int, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	UpdatePractitionerProfile(ctx context.Context, userID uuid.UUID, req *RqUpdatePractitioner) error
	UpdateStripeCustomerID(ctx context.Context, practitionerID uuid.UUID, customerID string) error
	UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error
	GetFinancialSettings(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*FinancialSettings, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreatePractitioner(ctx context.Context, req *RqCreatePractitioner, tx *sqlx.Tx) (*RsPractitioner, error) {
	query := `
		INSERT INTO tbl_practitioner (user_id, entity_type, entity_name, abn, acn, address, profession)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, abn, verified, subscription_status, stripe_customer_id, entity_type, entity_name, acn, address, profession, created_at, updated_at, deleted_at
	`
	var p Practitioner
	if err := tx.QueryRowxContext(ctx, query, req.UserID, req.EntityType, req.EntityName, req.ABN, req.ACN, req.Address, req.Profession).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

func (r *repository) DeletePractitioner(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_practitioner SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errNotFound
	}
	return nil
}

func (r *repository) GetPractitioner(ctx context.Context, id uuid.UUID) (*RsPractitioner, error) {
	query := `
		SELECT p.id, p.user_id, p.abn, p.verified, p.subscription_status, p.stripe_customer_id, p.entity_type, p.entity_name, p.address, p.acn, p.profession, p.created_at, p.updated_at, p.deleted_at,
		       u.email, u.first_name, u.last_name, u.phone
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	var p PractitionerWithUser
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

func (r *repository) GetPractitionerByUserID(ctx context.Context, userID string) (*RsPractitioner, error) {
	query := `
		SELECT id, user_id, abn, verified, subscription_status, stripe_customer_id, entity_type, entity_name, address, acn, profession, created_at, updated_at, deleted_at 
		FROM tbl_practitioner 
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	var p Practitioner
	if err := r.db.QueryRowxContext(ctx, query, userID).StructScan(&p); err != nil {
		return nil, err
	}
	return p.ToRs(), nil
}

var practitionerColumns = map[string]string{
	"id":          "p.id",
	"first_name":  "u.first_name",
	"last_name":   "u.last_name",
	"email":       "u.email",
	"phone":       "u.phone",
	"abn":         "p.abn",
	"acn":         "p.acn",
	"entity_name": "p.entity_name",
	"entity_type": "p.entity_type",
	"profession":  "p.profession",
}

var practitionerSearchCols = []string{"u.first_name", "u.last_name", "u.email", "u.phone", "p.entity_name", "p.profession"}

func (r *repository) ListPractitioners(ctx context.Context, f common.Filter) ([]*PractitionerWithUser, error) {
	base := `
		SELECT p.id, p.user_id, p.abn, p.verified, p.subscription_status, p.stripe_customer_id,
		COALESCE(p.entity_type, 'SOLE_TRADER') as entity_type, 
		COALESCE(p.entity_name, '') as entity_name,
		p.acn, p.address, p.profession, p.created_at, p.updated_at, p.deleted_at,
		u.email, u.first_name, u.last_name, u.phone
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
	`
	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, false)

	var list []*PractitionerWithUser
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), filterArgs...); err != nil {
		return nil, fmt.Errorf("list practitioners repo: %w", err)
	}
	return list, nil
}

func (r *repository) CountPractitioners(ctx context.Context, f common.Filter) (int, error) {
	base := `
        FROM tbl_practitioner p
        JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
        WHERE p.deleted_at IS NULL
    `
	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, true)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), filterArgs...); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) ListPractitionersForAccountant(ctx context.Context, accountantID uuid.UUID, f common.Filter) ([]*PractitionerWithUser, error) {
	base := `
		SELECT p.id, p.user_id, p.abn, p.verified, p.subscription_status, p.stripe_customer_id, p.created_at, p.updated_at, p.deleted_at,
		       u.email, u.first_name, u.last_name, u.phone
		FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
		JOIN tbl_invitation i ON i.practitioner_id = p.id 
		WHERE p.deleted_at IS NULL
		  AND i.accountant_id = ?
		  AND i.status = 'COMPLETED'
	`
	query, args := r.executeAccountantQuery(base, f, accountantID, false)

	var list []*PractitionerWithUser
	if err := r.db.SelectContext(ctx, &list, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("list practitioners for accountant repo: %w", err)
	}
	return list, nil
}

func (r *repository) CountPractitionersForAccountant(ctx context.Context, accountantID uuid.UUID, f common.Filter) (int, error) {
	base := `
        FROM tbl_practitioner p
        JOIN tbl_user u ON u.id = p.user_id AND u.deleted_at IS NULL
        JOIN tbl_invitation i ON i.practitioner_id = p.id
        WHERE p.deleted_at IS NULL
          AND i.accountant_id = ?
          AND i.status = 'COMPLETED'
    `
	query, args := r.executeAccountantQuery(base, f, accountantID, true)

	var count int
	if err := r.db.GetContext(ctx, &count, r.db.Rebind(query), args...); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) UpdatePractitionerProfile(ctx context.Context, userID uuid.UUID, req *RqUpdatePractitioner) error {
	query := `UPDATE tbl_practitioner 
		SET 
			abn = COALESCE($1, abn),
			entity_type = CASE WHEN $2::text = '' THEN entity_type ELSE $2::business_entity_type END,
			entity_name = CASE WHEN $3 = '' THEN entity_name ELSE $3 END,
			acn = COALESCE($4, acn),
			address = COALESCE($5, address),
			profession = COALESCE($6, profession),
			updated_at = NOW()
		WHERE user_id = $7 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query,
		req.ABN, req.EntityType, req.EntityName,
		req.ACN, req.Address, req.Profession, userID)
	return err
}

func (r *repository) UpdateStripeCustomerID(ctx context.Context, practitionerID uuid.UUID, customerID string) error {
	query := `UPDATE tbl_practitioner SET stripe_customer_id = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, practitionerID, customerID)
	return err
}

func (r *repository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	profileQuery := `
        UPDATE tbl_practitioner 
        SET deleted_at = now(), updated_at = now()
        WHERE user_id = $1 AND deleted_at IS NULL
    `
	if _, err := r.db.ExecContext(ctx, profileQuery, userID); err != nil {
		return fmt.Errorf("failed to soft-delete practitioner profile: %w", err)
	}

	subQuery := `
        UPDATE tbl_practitioner_subscription 
        SET deleted_at = now(), updated_at = now(), status = 'CANCELLED'
        WHERE practitioner_id IN (
            SELECT id FROM tbl_practitioner WHERE user_id = $1
        ) AND deleted_at IS NULL
    `
	if _, err := r.db.ExecContext(ctx, subQuery, userID); err != nil {
		return fmt.Errorf("failed to deactivate practitioner subscriptions: %w", err)
	}

	return nil
}

func (r *repository) UpdateLockDate(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID, lockDate *string) error {
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM tbl_financial_settings WHERE practitioner_id = $1 AND financial_year_id = $2)`
	if err := r.db.QueryRowContext(ctx, checkQuery, practitionerID, fyID).Scan(&exists); err != nil {
		return err
	}

	if exists {
		updateQuery := `
			UPDATE tbl_financial_settings 
			SET lock_date = CASE WHEN $1::text IS NULL THEN NULL ELSE TO_DATE($1, 'YYYY-MM-DD') END, updated_at = NOW() 
			WHERE practitioner_id = $2 AND financial_year_id = $3`
		_, err := r.db.ExecContext(ctx, updateQuery, lockDate, practitionerID, fyID)
		return err
	}

	insertQuery := `
		INSERT INTO tbl_financial_settings (practitioner_id, financial_year_id, lock_date, created_at, updated_at)
		VALUES ($1, $2, CASE WHEN $3::text IS NULL THEN NULL ELSE TO_DATE($3, 'YYYY-MM-DD') END, NOW(), NOW())`
	_, err := r.db.ExecContext(ctx, insertQuery, practitionerID, fyID, lockDate)
	return err
}

func (r *repository) GetFinancialSettings(ctx context.Context, practitionerID uuid.UUID, fyID uuid.UUID) (*FinancialSettings, error) {
	query := `
        SELECT id, practitioner_id, financial_year_id, TO_CHAR(lock_date, 'YYYY-MM-DD') as lock_date, created_at, updated_at
        FROM tbl_financial_settings
        WHERE practitioner_id = $1 AND financial_year_id = $2`

	var fs FinancialSettings
	if err := r.db.QueryRowxContext(ctx, query, practitionerID, fyID).StructScan(&fs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get financial settings: %w", err)
	}
	return &fs, nil
}

func (r *repository) executeAccountantQuery(base string, f common.Filter, accountantID uuid.UUID, isCount bool) (string, []interface{}) {
	query, filterArgs := common.BuildQuery(base, f, practitionerColumns, practitionerSearchCols, isCount)
	return query, append([]interface{}{accountantID}, filterArgs...)
}
