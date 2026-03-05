package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	// User
	CreateUser(ctx context.Context, user *User) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)

	// Auth provider
	UpsertAuthProvider(ctx context.Context, p *AuthProvider) (*AuthProvider, error)
	FindAuthProvider(ctx context.Context, provider string) (*AuthProvider, error)

	// Session
	CreateSession(ctx context.Context, s *Session) (*Session, error)
	FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error)
	DeleteSession(ctx context.Context, id string) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(ctx context.Context, user *User) (*User, error) {
	query := `
		INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, is_superadmin)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password, first_name, last_name, phone, is_superadmin, created_at, updated_at
	`
	var u User
	if err := r.db.QueryRowxContext(ctx, query,
		user.ID, user.Email, user.Password,
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

func (r *repository) FindByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, email, password, first_name, last_name, phone, is_superadmin, created_at, updated_at
		FROM tbl_user
		WHERE id = $1 AND deleted_at IS NULL
	`
	var u User
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&u); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}

func (r *repository) UpsertAuthProvider(ctx context.Context, p *AuthProvider) (*AuthProvider, error) {
	query := `
		INSERT INTO tbl_auth_provider
			(id, user_id, provider,
			 access_token, refresh_token, token_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider) DO UPDATE SET
			access_token     = EXCLUDED.access_token,
			refresh_token    = EXCLUDED.refresh_token,
			token_expires_at = EXCLUDED.token_expires_at
		RETURNING id, user_id, provider, access_token, refresh_token, token_expires_at, created_at, updated_at
	`
	var ap AuthProvider
	if err := r.db.QueryRowxContext(ctx, query,
		p.ID, p.UserID, p.Provider,
		p.AccessToken, p.RefreshToken, p.TokenExpiresAt,
	).StructScan(&ap); err != nil {
		return nil, fmt.Errorf("upsert auth provider: %w", err)
	}
	return &ap, nil
}

func (r *repository) FindAuthProvider(ctx context.Context, provider string) (*AuthProvider, error) {
	query := `
		SELECT id, user_id, provider, access_token, refresh_token, token_expires_at, created_at, updated_at
		FROM tbl_auth_provider
		WHERE provider = $1 AND deleted_at IS NULL
	`
	var ap AuthProvider
	if err := r.db.QueryRowxContext(ctx, query, provider).StructScan(&ap); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find auth provider: %w", err)
	}
	return &ap, nil
}

func (r *repository) CreateSession(ctx context.Context, s *Session) (*Session, error) {
	query := `
		INSERT INTO tbl_session (id, user_id, refresh_token, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, updated_at
	`
	var sess Session
	if err := r.db.QueryRowxContext(ctx, query,
		s.ID, s.UserID, s.RefreshToken,
		s.UserAgent, s.IPAddress, s.ExpiresAt,
	).StructScan(&sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &sess, nil
}

func (r *repository) FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	query := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at, updated_at
		FROM tbl_session
		WHERE refresh_token = $1 AND deleted_at IS NULL AND expires_at > $2
	`
	var sess Session
	if err := r.db.QueryRowxContext(ctx, query, refreshToken, time.Now()).StructScan(&sess); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find session: %w", err)
	}
	return &sess, nil
}

func (r *repository) DeleteSession(ctx context.Context, id string) error {
	query := `UPDATE tbl_session SET deleted_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
