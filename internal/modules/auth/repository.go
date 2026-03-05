package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	CreateUser(ctx context.Context, user *User) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(ctx context.Context, user *User) (*User, error) {
	query := `
		INSERT INTO tbl_user (email, password, first_name, last_name, phone, is_superadmin)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, password, first_name, last_name, phone, is_superadmin, created_at, updated_at
	`
	var u User
	if err := r.db.QueryRowxContext(ctx, query,
		user.Email, user.Password,
		user.FirstName, user.LastName,
		user.Phone, user.IsSuperadmin,
	).StructScan(&u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password, first_name, last_name, phone, is_superadmin, created_at, updated_at
		FROM tbl_user
		WHERE email = $1 AND deleted_at IS NULL
	`
	var u User
	if err := r.db.QueryRowxContext(ctx, query, email).StructScan(&u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &u, nil
}
