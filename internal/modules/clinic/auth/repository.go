package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	// Clinic Profile
	CreateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error)
	FindByEmail(ctx context.Context, email string) (*Clinic, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Clinic, error)
	UpdateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error)
	DeleteClinic(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, clinicID uuid.UUID, hashedPassword string) error

	// Clinic Addresses
	CreateAddress(ctx context.Context, addr *Address, tx *sqlx.Tx) (*Address, error)
	ListAddressesByClinicID(ctx context.Context, clinicID uuid.UUID) ([]Address, error)
	UpdateAddress(ctx context.Context, addr *Address, tx *sqlx.Tx) (*Address, error)
	DeleteAddressByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error
	CountActiveAddresses(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error)
	GetAddressByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) (*Address, error)

	// Clinic Contacts
	CreateContact(ctx context.Context, contact *Contact, tx *sqlx.Tx) (*Contact, error)
	ListContactsByClinicID(ctx context.Context, clinicID uuid.UUID) ([]Contact, error)
	UpdateContact(ctx context.Context, contact *Contact, tx *sqlx.Tx) (*Contact, error)
	DeleteContactByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error
	CountActiveContacts(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error)
	GetContactByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) (*Contact, error)

	// Session
	CreateSession(ctx context.Context, s *Session) (*Session, error)
	FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error

	// verificaction token
	CreateVerificationToken(ctx context.Context, token *VerificationToken, tx *sqlx.Tx) error
	DeactivateOldTokens(ctx context.Context, clinicID uuid.UUID) error
	GetToken(ctx context.Context, tokenID uuid.UUID) (*VerificationToken, error)
	MarkUserVerified(ctx context.Context, token *VerificationToken) error

	// password reset
	SaveResetToken(ctx context.Context, clinicID string, tokenHash string, expiresAt time.Time) error
	CompletePasswordReset(ctx context.Context, tokenHash string, newPasswordHash string) error

	// Document
	GetDocumentByID(ctx context.Context, documentID *uuid.UUID) (*file.Document, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error) {
	const query = `
		INSERT INTO tbl_invoice_clinic (document_id, clinic_name, description, email, password, role, abn, acn, verified)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9)
		RETURNING id, document_id, clinic_name, description, email, password, role, abn, acn, created_at, updated_at
	`
	var dest Clinic
	var err error

	if tx != nil {
		err = tx.QueryRowxContext(ctx, query,
			clinic.DocumentID, clinic.ClinicName, clinic.Description,
			clinic.Email, clinic.Password, clinic.Role, clinic.ABN, clinic.ACN, clinic.Verified,
		).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query,
			clinic.DocumentID, clinic.ClinicName, clinic.Description,
			clinic.Email, clinic.Password, clinic.Role, clinic.ABN, clinic.ACN, clinic.Verified,
		).StructScan(&dest)
	}

	if err != nil {
		return nil, fmt.Errorf("repository error creating invoice clinic profile: %w", err)
	}
	return &dest, nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*Clinic, error) {
	const query = `
		SELECT id, document_id, clinic_name, description, email, password, role,abn, acn, verified, created_at, updated_at
		FROM tbl_invoice_clinic
		WHERE email = $1 AND deleted_at IS NULL
	`
	var dest Clinic
	if err := r.db.QueryRowxContext(ctx, query, email).StructScan(&dest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repository error finding clinic profile by email context: %w", err)
	}
	return &dest, nil
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*Clinic, error) {
	const query = `
		SELECT id, document_id, clinic_name, description, email, password, role, abn, acn, verified, created_at, updated_at
		FROM tbl_invoice_clinic
		WHERE id = $1 AND deleted_at IS NULL
	`
	var dest Clinic
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&dest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repository error finding clinic profile by database ID: %w", err)
	}
	return &dest, nil
}

func (r *repository) CreateAddress(ctx context.Context, addr *Address, tx *sqlx.Tx) (*Address, error) {
	const query = `
		INSERT INTO tbl_invoice_clinic_address (clinic_id, address, city, state, postcode, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
	`
	var dest Address
	var err error

	if tx != nil {
		err = tx.QueryRowxContext(ctx, query,
			addr.ClinicID, addr.Address, addr.City, addr.State, addr.Postcode, addr.IsPrimary,
		).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query,
			addr.ClinicID, addr.Address, addr.City, addr.State, addr.Postcode, addr.IsPrimary,
		).StructScan(&dest)
	}

	if err != nil {
		return nil, fmt.Errorf("repository error creating clinic location breakdown: %w", err)
	}
	return &dest, nil
}

func (r *repository) ListAddressesByClinicID(ctx context.Context, clinicID uuid.UUID) ([]Address, error) {
	const query = `
		SELECT id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
		FROM tbl_invoice_clinic_address
		WHERE clinic_id = $1 AND deleted_at IS NULL
		ORDER BY is_primary DESC, created_at ASC
	`
	dest := make([]Address, 0)
	if err := r.db.SelectContext(ctx, &dest, query, clinicID); err != nil {
		return nil, fmt.Errorf("repository error resolving clinic address array dataset: %w", err)
	}
	return dest, nil
}

func (r *repository) CreateContact(ctx context.Context, contact *Contact, tx *sqlx.Tx) (*Contact, error) {
	const query = `
		INSERT INTO tbl_invoice_clinic_contacts (clinic_id, contact_type, value, label, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
	`
	var dest Contact
	var err error

	if tx != nil {
		err = tx.QueryRowxContext(ctx, query,
			contact.ClinicID, contact.ContactType, contact.Value, contact.Label, contact.IsPrimary,
		).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query,
			contact.ClinicID, contact.ContactType, contact.Value, contact.Label, contact.IsPrimary,
		).StructScan(&dest)
	}

	if err != nil {
		return nil, fmt.Errorf("repository error setting clinic contact pathway mapping: %w", err)
	}
	return &dest, nil
}

func (r *repository) ListContactsByClinicID(ctx context.Context, clinicID uuid.UUID) ([]Contact, error) {
	const query = `
		SELECT id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
		FROM tbl_invoice_clinic_contacts
		WHERE clinic_id = $1 AND deleted_at IS NULL
		ORDER BY is_primary DESC, created_at ASC
	`
	dest := make([]Contact, 0)
	if err := r.db.SelectContext(ctx, &dest, query, clinicID); err != nil {
		return nil, fmt.Errorf("repository error resolving clinic contact routing list: %w", err)
	}
	return dest, nil
}

func (r *repository) UpdateAddress(ctx context.Context, addr *Address, tx *sqlx.Tx) (*Address, error) {
	const query = `
		UPDATE tbl_invoice_clinic_address
		SET address = $2, city = $3, state = $4, postcode = $5, is_primary = $6, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
	`
	var dest Address
	var err error
	if tx != nil {
		err = tx.QueryRowxContext(ctx, query, addr.ID, addr.Address, addr.City, addr.State, addr.Postcode, addr.IsPrimary).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query, addr.ID, addr.Address, addr.City, addr.State, addr.Postcode, addr.IsPrimary).StructScan(&dest)
	}
	if err != nil {
		return nil, fmt.Errorf("update address: %w", err)
	}
	return &dest, nil
}

func (r *repository) DeleteAddressByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	query := `UPDATE tbl_invoice_clinic_address SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, id)
	} else {
		_, err = r.db.ExecContext(ctx, query, id)
	}
	return err
}

func (r *repository) CountActiveAddresses(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error) {
	query := `SELECT COUNT(*) FROM tbl_invoice_clinic_address WHERE clinic_id = $1 AND deleted_at IS NULL`
	var count int
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, query, clinicID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, query, clinicID).Scan(&count)
	}
	return count, err
}

func (r *repository) UpdateContact(ctx context.Context, contact *Contact, tx *sqlx.Tx) (*Contact, error) {
	const query = `
		UPDATE tbl_invoice_clinic_contacts
		SET contact_type = $2, value = $3, label = $4, is_primary = $5, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
	`
	var dest Contact
	var err error
	if tx != nil {
		err = tx.QueryRowxContext(ctx, query, contact.ID, contact.ContactType, contact.Value, contact.Label, contact.IsPrimary).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query, contact.ID, contact.ContactType, contact.Value, contact.Label, contact.IsPrimary).StructScan(&dest)
	}
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	return &dest, nil
}

func (r *repository) DeleteContactByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	query := `UPDATE tbl_invoice_clinic_contacts SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, id)
	} else {
		_, err = r.db.ExecContext(ctx, query, id)
	}
	return err
}

func (r *repository) CountActiveContacts(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error) {
	query := `SELECT COUNT(*) FROM tbl_invoice_clinic_contacts WHERE clinic_id = $1 AND deleted_at IS NULL`
	var count int
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, query, clinicID).Scan(&count)
	} else {
		err = r.db.QueryRowContext(ctx, query, clinicID).Scan(&count)
	}
	return count, err
}

func (r *repository) CreateSession(ctx context.Context, s *Session) (*Session, error) {
	query := `
		INSERT INTO tbl_clinic_session (id, clinic_id, refresh_token, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, clinic_id, refresh_token, user_agent, ip_address, expires_at, created_at, updated_at
	`
	var sess Session
	if err := r.db.QueryRowxContext(ctx, query,
		s.ID, s.ClinicID, s.RefreshToken,
		s.UserAgent, s.IPAddress, s.ExpiresAt,
	).StructScan(&sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &sess, nil
}

func (r *repository) FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	query := `
		SELECT id, clinic_id, refresh_token, user_agent, ip_address, expires_at, created_at, updated_at
		FROM tbl_clinic_session
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
	query := `UPDATE tbl_clinic_session SET deleted_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *repository) CreateVerificationToken(ctx context.Context, token *VerificationToken, tx *sqlx.Tx) error {
	query := `
        INSERT INTO tbl_clinic_verification_token (id, clinic_id, role, status, expires_at)
        VALUES ($1, $2, $3, $4, $5)`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, token.ID, token.ClinicID, token.Role, token.Status, token.ExpiresAt)
	} else {
		_, err = r.db.ExecContext(ctx, query, token.ID, token.ClinicID, token.Role, token.Status, token.ExpiresAt)
	}
	return err
}

func (r *repository) DeactivateOldTokens(ctx context.Context, clinicID uuid.UUID) error {
	query := `UPDATE tbl_clinic_verification_token SET status = 'RESENT' WHERE clinic_id = $1 AND status = 'PENDING'`
	_, err := r.db.ExecContext(ctx, query, clinicID)
	return err
}

func (r *repository) GetToken(ctx context.Context, tokenID uuid.UUID) (*VerificationToken, error) {
	var t VerificationToken
	query := `SELECT * FROM tbl_clinic_verification_token WHERE id = $1`
	if err := r.db.GetContext(ctx, &t, query, tokenID); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *repository) MarkUserVerified(ctx context.Context, token *VerificationToken) error {
	// Update the verified status
	verifyQuery := "UPDATE tbl_invoice_clinic SET verified = true, updated_at = NOW() WHERE id = $1"
	res, err := r.db.ExecContext(ctx, verifyQuery, token.ClinicID)
	if err != nil {
		return fmt.Errorf("failed to update tbl_invoice_clinic verification: %w", err)
	}

	// Check if the row actually existed
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("Entity %s not found in tbl_invoice_clinic", token.ClinicID)
	}

	// Mark the verification token as USED so it can't be reused
	tokenUpdateQuery := `UPDATE tbl_clinic_verification_token SET status = 'USED' WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, tokenUpdateQuery, token.ID); err != nil {
		return fmt.Errorf("failed to update token status: %w", err)
	}
	return nil

}

func (r *repository) UpdateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error) {
	const query = `
		UPDATE tbl_invoice_clinic
		SET clinic_name = $2, description = $3, document_id = $4, abn = $5, acn = $6, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, document_id, clinic_name, description, email, password, role, abn, acn, verified, created_at, updated_at
	`
	var dest Clinic
	var err error
	if tx != nil {
		err = tx.QueryRowxContext(ctx, query,
			clinic.ID, clinic.ClinicName, clinic.Description, clinic.DocumentID, clinic.ABN, clinic.ACN,
		).StructScan(&dest)
	} else {
		err = r.db.QueryRowxContext(ctx, query,
			clinic.ID, clinic.ClinicName, clinic.Description, clinic.DocumentID, clinic.ABN, clinic.ACN,
		).StructScan(&dest)
	}
	if err != nil {
		return nil, fmt.Errorf("update clinic: %w", err)
	}
	return &dest, nil
}

func (r *repository) DeleteClinic(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_invoice_clinic SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete clinic: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) UpdatePassword(ctx context.Context, clinicID uuid.UUID, hashedPassword string) error {
	query := `UPDATE tbl_invoice_clinic SET password = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, hashedPassword, clinicID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *repository) SaveResetToken(ctx context.Context, clinicID string, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO tbl_clinic_password_resets (clinic_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, clinicID, tokenHash, expiresAt)
	return err
}

func (r *repository) CompletePasswordReset(ctx context.Context, tokenHash string, newPasswordHash string) error {
	query := `
	WITH updated_token AS (
		UPDATE tbl_clinic_password_resets
		SET status = CASE
			WHEN expires_at < NOW() THEN 'EXPIRED'::token_status
			ELSE 'USED'::token_status
		END
		WHERE token_hash = $1
		  AND status = 'PENDING'::token_status
		RETURNING clinic_id, status
	)
	UPDATE tbl_invoice_clinic
	SET password = $2, updated_at = NOW()
	FROM updated_token
	WHERE tbl_invoice_clinic.id = updated_token.clinic_id
	  AND updated_token.status = 'USED'::token_status`

	result, err := r.db.ExecContext(ctx, query, tokenHash, newPasswordHash)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("invalid or expired reset link")
	}
	return nil
}

func (r *repository) GetDocumentByID(ctx context.Context, documentID *uuid.UUID) (*file.Document, error) {
	query := `
		SELECT id, owner_id, owner_role, object_key, bucket, original_name, extension,
		       mime_type, size_bytes, checksum, status, is_public, uploaded_at,
		       created_at, updated_at, deleted_at
		FROM tbl_document
		WHERE id = $1 AND deleted_at IS NULL
	`

	var doc file.Document
	if err := r.db.GetContext(ctx, &doc, query, documentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get document by id: %w", err)
	}
	return &doc, nil
}

func (r *repository) GetAddressByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) (*Address, error) {
	query := `
		SELECT id,clinic_id,address,city,state,postcode,is_primary,created_at,updated_at 
		FROM tbl_invoice_clinic_address 
		WHERE id = $1 AND deleted_at IS NULL
	`

	var address Address
	if err := tx.GetContext(ctx, &address, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get address by id: %w", err)
	}

	return &address, nil
}

func (r *repository) GetContactByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) (*Contact, error) {
	query := `
		SELECT id,clinic_id,contact_type,"value",label,is_primary,created_at,updated_at 
		FROM tbl_invoice_clinic_contacts 
		WHERE id = $1 AND deleted_at IS NULL
	`

	var contact Contact
	if err := tx.GetContext(ctx, &contact, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact by id: %w", err)
	}

	return &contact, nil
}
