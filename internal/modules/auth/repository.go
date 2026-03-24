package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	// User
	CreateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error)
	UpdateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error)
	DeleteUser(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByPractitionerID(ctx context.Context, pracID uuid.UUID) (*User, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error

	// Auth provider
	UpsertAuthProvider(ctx context.Context, p *AuthProvider, tx *sqlx.Tx) (*AuthProvider, error)
	FindAuthProvider(ctx context.Context, provider string) (*AuthProvider, error)

	// Session
	CreateSession(ctx context.Context, s *Session) (*Session, error)
	FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error

	// verificaction token
	CreateVerificationToken(ctx context.Context, tx *sqlx.Tx, token *VerificationToken) error
	DeactivateOldTokens(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error
	GetToken(ctx context.Context, tokenID uuid.UUID) (*VerificationToken, error)
	MarkUserVerified(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error
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
	var err error
	if user.ID == uuid.Nil {
		query := `
			INSERT INTO tbl_user (email, password, first_name, last_name, phone, role)
			VALUES ($1, NULLIF($2, ''), $3, $4, $5,$6)
			` + returning
		err = tx.QueryRowxContext(ctx, query,
			user.Email, user.Password,
			user.FirstName, user.LastName,
			user.Phone, user.Role,
		).StructScan(&u)
	} else {
		query := `
			INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role)
			VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7)
			` + returning
		err = tx.QueryRowxContext(ctx, query,
			user.ID, user.Email, user.Password,
			user.FirstName, user.LastName,
			user.Phone, user.Role,
		).StructScan(&u)
	}
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *repository) UpdateUser(ctx context.Context, user *User, tx *sqlx.Tx) (*User, error) {
	const returning = `RETURNING id, email, password, first_name, last_name, phone, is_superadmin, created_at, updated_at`

	query := `
        UPDATE tbl_user 
        SET email = $2, 
            first_name = $3, 
            last_name = $4, 
            phone = $5,
            updated_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL
    ` + returning

	var u User
	var err error
	// 1. Check if a transaction was provided
	if tx != nil {
		// Use the transaction (tx)
		err = tx.QueryRowxContext(ctx, query,
			user.ID,        // $1
			user.Email,     // $2
			user.FirstName, // $3
			user.LastName,  // $4
			user.Phone,     // $5
		).StructScan(&u)
	} else {
		// Fallback to the standard DB connection (r.db)
		err = r.db.QueryRowxContext(ctx, query,
			user.ID,        // $1
			user.Email,     // $2
			user.FirstName, // $3
			user.LastName,  // $4
			user.Phone,     // $5
		).StructScan(&u)
	}

	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &u, nil
}

func (r *repository) DeleteUser(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	query := `UPDATE tbl_user SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, id)
	} else {
		_, err = r.db.ExecContext(ctx, query, id)
	}

	if err != nil {
		return fmt.Errorf("repository delete user: %w", err)
	}

	return nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password, first_name, last_name, phone, role, created_at, updated_at
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

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password, first_name, last_name, phone, role, created_at, updated_at
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

func (r *repository) FindByPractitionerID(ctx context.Context, pracID uuid.UUID) (*User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.first_name, u.last_name, u.phone, u.is_superadmin 
		FROM tbl_user u
		JOIN tbl_practitioner p ON p.user_id = u.id
		WHERE p.id = $1 AND u.deleted_at IS NULL
	`
	var u User
	if err := r.db.GetContext(ctx, &u, query, pracID); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repository) UpsertAuthProvider(ctx context.Context, p *AuthProvider, tx *sqlx.Tx) (*AuthProvider, error) {
	const returning = `RETURNING id, user_id, provider, access_token, refresh_token, token_expires_at, created_at, updated_at`
	var ap AuthProvider
	var err error
	if p.ID == uuid.Nil {
		query := `
			INSERT INTO tbl_auth_provider
				(user_id, provider, access_token, refresh_token, token_expires_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, provider) DO UPDATE SET
				access_token     = EXCLUDED.access_token,
				refresh_token    = EXCLUDED.refresh_token,
				token_expires_at = EXCLUDED.token_expires_at
			` + returning
		err = tx.QueryRowxContext(ctx, query,
			p.UserID, p.Provider,
			p.AccessToken, p.RefreshToken, p.TokenExpiresAt,
		).StructScan(&ap)
	} else {
		query := `
			INSERT INTO tbl_auth_provider
				(id, user_id, provider, access_token, refresh_token, token_expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, provider) DO UPDATE SET
				access_token     = EXCLUDED.access_token,
				refresh_token    = EXCLUDED.refresh_token,
				token_expires_at = EXCLUDED.token_expires_at
			` + returning
		err = tx.QueryRowxContext(ctx, query,
			p.ID, p.UserID, p.Provider,
			p.AccessToken, p.RefreshToken, p.TokenExpiresAt,
		).StructScan(&ap)
	}
	if err != nil {
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

func (r *repository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_session SET deleted_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *repository) CreateVerificationToken(ctx context.Context, tx *sqlx.Tx, token *VerificationToken) error {
	query := `
        INSERT INTO tbl_verification_token (id, entity_id, role, status, expires_at)
        VALUES ($1, $2, $3, $4, $5)`
	_, err := tx.ExecContext(ctx, query, token.ID, token.EntityID, token.Role, token.Status, token.ExpiresAt)
	return err
}

func (r *repository) DeactivateOldTokens(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID) error {
	query := `UPDATE tbl_verification_token SET status = 'RESENT' WHERE entity_id = $1 AND status = 'PENDING'`
	_, err := tx.ExecContext(ctx, query, entityID)
	return err
}

func (r *repository) GetToken(ctx context.Context, tokenID uuid.UUID) (*VerificationToken, error) {
	var t VerificationToken
	query := `SELECT * FROM tbl_verification_token WHERE id = $1`
	if err := r.db.GetContext(ctx, &t, query, tokenID); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *repository) MarkUserVerified(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
	return util.RunInTransaction(ctx, r.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Mark user as verified (There is no such column in tbl_user yet)
		// if _, err := tx.ExecContext(ctx, "UPDATE tbl_user SET verified = true WHERE id = $1", userID); err != nil {
		// 	return err
		// }

		// Mark token as used
		if _, err := tx.ExecContext(ctx, "UPDATE tbl_verification_token SET status = 'USED' WHERE id = $1", tokenID); err != nil {
			return err
		}
		return nil
	})
}

func (r *repository) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	query := `UPDATE tbl_user SET password = $1, updated_at = now() WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, hashedPassword, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
