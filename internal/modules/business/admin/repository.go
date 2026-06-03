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
	FindByUserID(ctx context.Context, userID uuid.UUID) (*Admin, error)
	FindByID(ctx context.Context, id uuid.UUID) (*RsAdminDetail, error)
	GetAllAdmins(ctx context.Context) ([]RsAdminDetail, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error) {
	const returning = `RETURNING id, email, password, first_name, last_name, phone, role, created_at, updated_at`
	var u User

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
	query := `INSERT INTO tbl_admin (user_id) VALUES ($1) RETURNING id, user_id, created_at, updated_at`
	err := tx.QueryRowxContext(ctx, query, a.UserID).StructScan(a)
	return a, err
}

func (r *repository) FindByUserID(ctx context.Context, userID uuid.UUID) (*Admin, error) {
	var a Admin
	query := `SELECT * FROM tbl_admin WHERE user_id = $1 AND deleted_at IS NULL`
	if err := r.db.GetContext(ctx, &a, query, userID); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*RsAdminDetail, error) {
	var detail RsAdminDetail
	query := `
		SELECT 
			a.id          AS "id", 
			u.id          AS "user.id", 
			u.email       AS "user.email", 
			u.first_name  AS "user.first_name", 
			u.last_name   AS "user.last_name", 
			u.phone       AS "user.phone"
		FROM tbl_admin a
		JOIN tbl_user u ON a.user_id = u.id
		WHERE a.id = $1 AND a.deleted_at IS NULL AND u.deleted_at IS NULL
	`
	if err := r.db.GetContext(ctx, &detail, query, id); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (r *repository) GetAllAdmins(ctx context.Context) ([]RsAdminDetail, error) {
	var admins []RsAdminDetail
	query := `
		SELECT 
			a.id          AS "id", 
			u.id          AS "user.id", 
			u.email       AS "user.email", 
			u.first_name  AS "user.first_name", 
			u.last_name   AS "user.last_name", 
			u.phone       AS "user.phone"
		FROM tbl_admin a
		JOIN tbl_user u ON a.user_id = u.id
		WHERE a.deleted_at IS NULL AND u.deleted_at IS NULL
	`
	if err := r.db.SelectContext(ctx, &admins, query); err != nil {
		return nil, err
	}
	return admins, nil
}
