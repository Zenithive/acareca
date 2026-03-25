package invitation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, inv *Invitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	GetByEmail(ctx context.Context, email string) (*Invitation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error
	GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error)
	GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error)

	GetAccountantIDByEmail(ctx context.Context, email string) (*uuid.UUID, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, inv *Invitation) error {
	query := `INSERT INTO tbl_invitation (id, practitioner_id, entity_id, email, status, expires_at) 
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, inv.ID, inv.PractitionerID, inv.EntityID, inv.Email, inv.Status, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create invitation: %w", err)
	}
	return nil
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Invitation, error) {
	inv := &Invitation{}
	query := `SELECT * FROM tbl_invitation WHERE id = $1`
	err := r.db.GetContext(ctx, inv, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}

func (r *repository) GetByEmail(ctx context.Context, email string) (*Invitation, error) {
	inv := &Invitation{}
	query := `SELECT * FROM tbl_invitation WHERE email = $1 ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, inv, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return inv, nil
}

func (r *repository) UpdateStatus(ctx context.Context, id uuid.UUID, status InvitationStatus, entityID *uuid.UUID) error {
	query := `UPDATE tbl_invitation SET status = $1, entity_id = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, entityID, id)
	return err
}

func (r *repository) GetPractitionerName(ctx context.Context, practitionerID uuid.UUID) (string, error) {
	var name struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	// Joining practitioner to user to get the name
	query := `
		SELECT u.first_name, u.last_name 
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		WHERE p.id = $1`

	err := r.db.GetContext(ctx, &name, query, practitionerID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch practitioner name: %w", err)
	}

	return fmt.Sprintf("%s %s", name.FirstName, name.LastName), nil
}

func (r *repository) GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	var userID uuid.UUID
	query := `SELECT id FROM tbl_user WHERE email = $1 LIMIT 1`

	err := r.db.GetContext(ctx, &userID, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // User does not exist
		}
		return nil, err
	}
	return &userID, nil
}

func (r *repository) GetAccountantIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	var accountantID uuid.UUID
	query := `
        SELECT a.id 
        FROM tbl_accountant a
        JOIN tbl_user u ON a.user_id = u.id
        WHERE u.email = $1 
        LIMIT 1`

	err := r.db.GetContext(ctx, &accountantID, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &accountantID, nil
}
