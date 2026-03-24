package accountant

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)

	GetAllUsers(ctx context.Context) ([]RsAccountantUser, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error) {
	query := `
		INSERT INTO tbl_accountant (user_id)
		VALUES ($1)
		RETURNING id, user_id, verified
	`
	var a Accountant
	if err := tx.QueryRowxContext(ctx, query, req.UserID).StructScan(&a); err != nil {
		return nil, err
	}

	settingQuery := `
		INSERT INTO tbl_accountant_setting (accountant_id, settings)
		VALUES ($1, $2)
	`
	if _, err := tx.ExecContext(ctx, settingQuery, a.ID, "{}"); err != nil {
		return nil, err
	}

	return &RsAccountant{
		ID:       a.ID,
		UserID:   a.UserID.String(),
		Verified: a.Verified,
	}, nil
}

func (r *repository) GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error) {
	query := `SELECT id, user_id, verified FROM tbl_accountant WHERE user_id = $1 AND deleted_at IS NULL`
	var a Accountant
	if err := r.db.GetContext(ctx, &a, query, userID); err != nil {
		return nil, err
	}
	return &RsAccountant{ID: a.ID, UserID: a.UserID.String(), Verified: a.Verified}, nil
}

func (r *repository) GetAllUsers(ctx context.Context) ([]RsAccountantUser, error) {
	var users []RsAccountantUser

	query := `
		SELECT id, email, first_name, last_name, phone, is_superadmin, created_at, updated_at
		FROM tbl_user 
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC`

	err := r.db.SelectContext(ctx, &users, query)
	if err != nil {
		return nil, err
	}
	return users, nil
}
