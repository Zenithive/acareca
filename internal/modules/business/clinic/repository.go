package clinic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("clinic not found")

type Repository interface {
	CreateClinic(ctx context.Context, clinic *Clinic) (*Clinic, error)
	CreateClinicAddress(ctx context.Context, address *ClinicAddress) (*ClinicAddress, error)
	CreateClinicContact(ctx context.Context, contact *ClinicContact) (*ClinicContact, error)
	CreateFinancialSettings(ctx context.Context, settings *FinancialSettings) (*FinancialSettings, error)

	GetClinics(ctx context.Context) ([]Clinic, error)
	GetClinicsByPractitioner(ctx context.Context, practitionerID uuid.UUID) ([]Clinic, error)
	GetClinicByID(ctx context.Context, id uuid.UUID) (*Clinic, error)
	GetClinicByIDAndPractitioner(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*Clinic, error)
	GetClinicAddresses(ctx context.Context, clinicID uuid.UUID) ([]ClinicAddress, error)
	GetClinicContacts(ctx context.Context, clinicID uuid.UUID) ([]ClinicContact, error)
	GetFinancialSettings(ctx context.Context, clinicID uuid.UUID) (*FinancialSettings, error)
	GetActiveFinancialYear(ctx context.Context) (*uuid.UUID, error)
	GetPractitionerIDByUserID(ctx context.Context, userID string) (*uuid.UUID, error)

	UpdateClinic(ctx context.Context, clinic *Clinic) (*Clinic, error)
	UpdateClinicAddress(ctx context.Context, address *ClinicAddress) error
	UpdateClinicContact(ctx context.Context, contact *ClinicContact) error
	UpdateFinancialSettings(ctx context.Context, settings *FinancialSettings) error

	DeleteClinic(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateClinic(ctx context.Context, clinic *Clinic) (*Clinic, error) {
	query := `
		INSERT INTO tbl_clinic (practitioner_id, profile_picture, name, abn, description, is_active)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, TRUE))
		RETURNING id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
	`
	var c Clinic
	err := r.db.QueryRowxContext(ctx, query,
		clinic.PractitionerID, clinic.ProfilePicture, clinic.Name,
		clinic.ABN, clinic.Description, clinic.IsActive,
	).StructScan(&c)
	if err != nil {
		return nil, fmt.Errorf("create clinic: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateClinicAddress(ctx context.Context, address *ClinicAddress) (*ClinicAddress, error) {
	query := `
		INSERT INTO tbl_clinic_address (clinic_id, address, city, state, postcode, is_primary)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, FALSE))
		RETURNING id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
	`
	var a ClinicAddress
	err := r.db.QueryRowxContext(ctx, query,
		address.ClinicID, address.Address, address.City,
		address.State, address.Postcode, address.IsPrimary,
	).StructScan(&a)
	if err != nil {
		return nil, fmt.Errorf("create clinic address: %w", err)
	}
	return &a, nil
}

func (r *repository) CreateClinicContact(ctx context.Context, contact *ClinicContact) (*ClinicContact, error) {
	query := `
		INSERT INTO tbl_clinic_contact (clinic_id, contact_type, value, label, is_primary)
		VALUES ($1, $2, $3, $4, COALESCE($5, FALSE))
		RETURNING id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
	`
	var c ClinicContact
	err := r.db.QueryRowxContext(ctx, query,
		contact.ClinicID, contact.ContactType, contact.Value,
		contact.Label, contact.IsPrimary,
	).StructScan(&c)
	if err != nil {
		return nil, fmt.Errorf("create clinic contact: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateFinancialSettings(ctx context.Context, settings *FinancialSettings) (*FinancialSettings, error) {
	query := `
		INSERT INTO tbl_financial_settings (clinic_id, financial_year_id, lock_date)
		VALUES ($1, $2, $3)
		RETURNING id, clinic_id, financial_year_id, lock_date, created_at, updated_at
	`
	var fs FinancialSettings
	err := r.db.QueryRowxContext(ctx, query,
		settings.ClinicID, settings.FinancialYearID, settings.LockDate,
	).StructScan(&fs)
	if err != nil {
		return nil, fmt.Errorf("create financial settings: %w", err)
	}
	return &fs, nil
}

func (r *repository) GetClinics(ctx context.Context) ([]Clinic, error) {
	query := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`
	var clinics []Clinic
	if err := r.db.SelectContext(ctx, &clinics, query); err != nil {
		return nil, fmt.Errorf("get clinics: %w", err)
	}
	return clinics, nil
}

func (r *repository) GetClinicByID(ctx context.Context, id uuid.UUID) (*Clinic, error) {
	query := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE id = $1 AND deleted_at IS NULL
	`
	var c Clinic
	if err := r.db.QueryRowxContext(ctx, query, id).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get clinic by id: %w", err)
	}
	return &c, nil
}

func (r *repository) GetClinicAddresses(ctx context.Context, clinicID uuid.UUID) ([]ClinicAddress, error) {
	query := `
		SELECT id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
		FROM tbl_clinic_address
		WHERE clinic_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`
	var addresses []ClinicAddress
	if err := r.db.SelectContext(ctx, &addresses, query, clinicID); err != nil {
		return nil, fmt.Errorf("get clinic addresses: %w", err)
	}
	return addresses, nil
}

func (r *repository) GetClinicContacts(ctx context.Context, clinicID uuid.UUID) ([]ClinicContact, error) {
	query := `
		SELECT id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
		FROM tbl_clinic_contact
		WHERE clinic_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`
	var contacts []ClinicContact
	if err := r.db.SelectContext(ctx, &contacts, query, clinicID); err != nil {
		return nil, fmt.Errorf("get clinic contacts: %w", err)
	}
	return contacts, nil
}

func (r *repository) GetFinancialSettings(ctx context.Context, clinicID uuid.UUID) (*FinancialSettings, error) {
	query := `
		SELECT id, clinic_id, financial_year_id, lock_date, created_at, updated_at
		FROM tbl_financial_settings
		WHERE clinic_id = $1
	`
	var fs FinancialSettings
	if err := r.db.QueryRowxContext(ctx, query, clinicID).StructScan(&fs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get financial settings: %w", err)
	}
	return &fs, nil
}

func (r *repository) DeleteClinic(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_clinic SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete clinic: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
func (r *repository) GetActiveFinancialYear(ctx context.Context) (*uuid.UUID, error) {
	query := `SELECT id FROM tbl_financial_year WHERE is_active = TRUE LIMIT 1`
	var id uuid.UUID
	if err := r.db.QueryRowxContext(ctx, query).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("no active financial year found")
		}
		return nil, fmt.Errorf("get active financial year: %w", err)
	}
	return &id, nil
}

func (r *repository) UpdateClinic(ctx context.Context, clinic *Clinic) (*Clinic, error) {
	query := `
		UPDATE tbl_clinic 
		SET practitioner_id = $1, profile_picture = $2, name = $3, abn = $4, 
		    description = $5, is_active = $6, updated_at = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
	`
	var c Clinic
	err := r.db.QueryRowxContext(ctx, query,
		clinic.PractitionerID, clinic.ProfilePicture, clinic.Name,
		clinic.ABN, clinic.Description, clinic.IsActive, clinic.ID,
	).StructScan(&c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update clinic: %w", err)
	}
	return &c, nil
}
func (r *repository) UpdateClinicAddress(ctx context.Context, address *ClinicAddress) error {
	query := `
		UPDATE tbl_clinic_address 
		SET address = $1, city = $2, state = $3, postcode = $4, is_primary = $5, updated_at = now()
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		address.Address, address.City, address.State,
		address.Postcode, address.IsPrimary, address.ID,
	)
	if err != nil {
		return fmt.Errorf("update clinic address: %w", err)
	}
	return nil
}

func (r *repository) UpdateClinicContact(ctx context.Context, contact *ClinicContact) error {
	query := `
		UPDATE tbl_clinic_contact 
		SET value = $1, label = $2, is_primary = $3, updated_at = now()
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query,
		contact.Value, contact.Label, contact.IsPrimary, contact.ID,
	)
	if err != nil {
		return fmt.Errorf("update clinic contact: %w", err)
	}
	return nil
}

func (r *repository) UpdateFinancialSettings(ctx context.Context, settings *FinancialSettings) error {
	query := `
		UPDATE tbl_financial_settings 
		SET financial_year_id = $1, lock_date = $2, updated_at = now()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query,
		settings.FinancialYearID, settings.LockDate, settings.ID,
	)
	if err != nil {
		return fmt.Errorf("update financial settings: %w", err)
	}
	return nil
}
func (r *repository) GetPractitionerIDByUserID(ctx context.Context, userID string) (*uuid.UUID, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	query := `SELECT id FROM tbl_practitioner WHERE user_id = $1 AND deleted_at IS NULL LIMIT 1`
	var id uuid.UUID
	if err := r.db.QueryRowxContext(ctx, query, userUUID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("practitioner not found for user")
		}
		return nil, fmt.Errorf("get practitioner by user_id: %w", err)
	}
	return &id, nil
}
func (r *repository) GetClinicsByPractitioner(ctx context.Context, practitionerID uuid.UUID) ([]Clinic, error) {
	query := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE practitioner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	var clinics []Clinic
	if err := r.db.SelectContext(ctx, &clinics, query, practitionerID); err != nil {
		return nil, fmt.Errorf("get clinics by practitioner: %w", err)
	}
	return clinics, nil
}

func (r *repository) GetClinicByIDAndPractitioner(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*Clinic, error) {
	query := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE id = $1 AND practitioner_id = $2 AND deleted_at IS NULL
	`

	var c Clinic
	if err := r.db.QueryRowxContext(ctx, query, id, practitionerID).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get clinic by id and practitioner: %w", err)
	}
	return &c, nil
}
