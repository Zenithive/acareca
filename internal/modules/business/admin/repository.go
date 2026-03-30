package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateAdmin(ctx context.Context, admin *Admin, tx *sqlx.Tx) (*Admin, error)
	CreateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error)
	FindByUserID(ctx context.Context, userID string) (*Admin, error)
	FindByID(ctx context.Context, id uuid.UUID) (*adminUserFlat, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error) {
	const returning = `RETURNING id, email, password, first_name, last_name, phone, role`
	var u User

	// Support both generated UUIDs and database-default UUIDs
	if user.ID == uuid.Nil {
		query := `
			INSERT INTO tbl_user (email, password, first_name, last_name, phone, role)
			VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6)
			` + returning
		err := tx.QueryRowxContext(ctx, query,
			user.Email, user.Password, user.FirstName, user.LastName, user.Phone, user.Role,
		).StructScan(&u)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	} else {
		query := `
			INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role)
			VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7)
			` + returning
		err := tx.QueryRowxContext(ctx, query,
			user.ID, user.Email, user.Password, user.FirstName, user.LastName, user.Phone, user.Role,
		).StructScan(&u)
		if err != nil {
			return nil, fmt.Errorf("create user with id: %w", err)
		}
	}

	return &u, nil
}

func (r *repository) CreateAdmin(ctx context.Context, a *Admin, tx *sqlx.Tx) (*Admin, error) {
	query := `INSERT INTO tbl_admin (user_id) VALUES ($1) RETURNING id, user_id`
	err := tx.GetContext(ctx, a, query, a.UserID)
	return a, err
}

func (r *repository) FindByUserID(ctx context.Context, userID string) (*Admin, error) {
	var a Admin
	query := `SELECT * FROM tbl_admin WHERE user_id = $1 AND deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &a, query, userID); err != nil {
		return nil, err
	}
	return &a, nil
}

// Internal flat struct for SQL scanning
type adminUserFlat struct {
	AdminID   uuid.UUID `db:"admin_id"`
	UserID    uuid.UUID `db:"user_id"`
	Email     string    `db:"email"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Phone     *string   `db:"phone"`
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*adminUserFlat, error) {
	var flat adminUserFlat
	query := `
		SELECT 
			a.id as admin_id, 
			u.id as user_id, 
			u.email, 
			u.first_name, 
			u.last_name, 
			u.phone
		FROM tbl_admin a
		JOIN tbl_user u ON a.user_id = u.id
		WHERE a.id = $1 AND a.deleted_at IS NULL
	`
	if err := r.db.GetContext(ctx, &flat, query, id); err != nil {
		return nil, err
	}
	return &flat, nil
}
