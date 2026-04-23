package clinic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("clinic not found")

type Repository interface {
	ListClinicByPractitioner(ctx context.Context, practitionerID uuid.UUID, filter common.Filter) ([]*Clinic, error)
	CountClinicByPractitioner(ctx context.Context, practitionerID uuid.UUID, filter common.Filter) (int, error)
	GetClinicByID(ctx context.Context, id uuid.UUID) (*Clinic, error)
	GetClinicByIDAndPractitioner(ctx context.Context, id uuid.UUID, practitionerID uuid.UUID) (*Clinic, error)
	GetClinicAddresses(ctx context.Context, clinicID uuid.UUID) ([]ClinicAddress, error)
	GetClinicContacts(ctx context.Context, clinicID uuid.UUID) ([]ClinicContact, error)
	GetPractitionerIDByUserID(ctx context.Context, userID string) (*uuid.UUID, error)

	DeleteClinic(ctx context.Context, id uuid.UUID) error
	DeleteClinicTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
	BulkDeleteClinics(ctx context.Context, ids []uuid.UUID) error

	GetFinancialSettings(ctx context.Context, clinicID uuid.UUID) (*FinancialSettings, error)

	CreateClinicTx(ctx context.Context, tx *sqlx.Tx, clinic *Clinic) (*Clinic, error)
	CreateClinicAddressTx(ctx context.Context, tx *sqlx.Tx, address *ClinicAddress) (*ClinicAddress, error)
	CreateClinicContactTx(ctx context.Context, tx *sqlx.Tx, contact *ClinicContact) (*ClinicContact, error)
	CreateFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, settings *FinancialSettings) (*FinancialSettings, error)
	GetActiveFinancialYearTx(ctx context.Context, tx *sqlx.Tx) (*uuid.UUID, error)
	GetClinicByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*Clinic, error)
	GetClinicByIDAndPractitionerTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, practitionerID uuid.UUID) (*Clinic, error)
	GetClinicAddressesTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) ([]ClinicAddress, error)
	GetClinicContactsTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) ([]ClinicContact, error)
	GetFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) (*FinancialSettings, error)
	GetAddressByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*ClinicAddress, error)
	GetContactByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*ClinicContact, error)
	UpdateClinicTx(ctx context.Context, tx *sqlx.Tx, clinic *Clinic) (*Clinic, error)
	UpdateClinicAddressTx(ctx context.Context, tx *sqlx.Tx, address *ClinicAddress) error
	UpdateClinicContactTx(ctx context.Context, tx *sqlx.Tx, contact *ClinicContact) error
	UpdateFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, settings *FinancialSettings) error
	UnsetPrimaryAddressTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, excludeID uuid.UUID) error
	UnsetPrimaryContactTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, excludeID uuid.UUID) error
	ListClinicByAccountant(ctx context.Context, accountantID uuid.UUID, filter common.Filter) ([]*Clinic, error)
	CountClinicByAccountant(ctx context.Context, accountantID uuid.UUID, filter common.Filter) (int, error)

	GetAccountantPermission(ctx context.Context, accountantID uuid.UUID, clinicID uuid.UUID) (*AccountantPermission, error)
	IsClinicOwner(ctx context.Context, practitionerID uuid.UUID, clinicID uuid.UUID) (bool, error)
	HasPermission(ctx context.Context, practitionerID uuid.UUID, accountantID uuid.UUID, entityType string, entityID *uuid.UUID, requiredPerm string) (bool, error)
	DeletePermissionsByEntity(ctx context.Context, entityID uuid.UUID, entityType string) error
	IsAccountantInvitedByPractitioner(ctx context.Context, practitionerID uuid.UUID, accountantID uuid.UUID) (bool, error)
	GetPractitionerForAccountant(ctx context.Context, accountantID uuid.UUID) (*uuid.UUID, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
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

func (r *repository) DeleteClinicTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	query := `UPDATE tbl_clinic SET deleted_at = NOW() WHERE id = $1`

	// If tx is provided, use it. Otherwise, use the standard DB pool.
	if tx != nil {
		_, err := tx.ExecContext(ctx, query, id)
		return err
	}

	_, err := r.db.ExecContext(ctx, query, id)
	return err
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

var clinicAllowedColumns = map[string]string{
	"id":         "id",
	"name":       "name",
	"is_active":  "is_active",
	"created_at": "created_at",
}

var clinicSearchColumns = []string{"name", "abn", "description"}

func (r *repository) ListClinicByPractitioner(ctx context.Context, practitionerID uuid.UUID, filter common.Filter) ([]*Clinic, error) {
	base := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE practitioner_id = ? AND deleted_at IS NULL`

	baseArgs := []interface{}{practitionerID}
	query, filterArgs := common.BuildQuery(base, filter, clinicAllowedColumns, clinicSearchColumns, false)
	query = r.db.Rebind(query)

	var list []*Clinic
	if err := r.db.SelectContext(ctx, &list, query, append(baseArgs, filterArgs...)...); err != nil {
		return nil, fmt.Errorf("list clinics: %w", err)
	}
	return list, nil
}

func (r *repository) CountClinicByPractitioner(ctx context.Context, practitionerID uuid.UUID, filter common.Filter) (int, error) {
	base := `
		FROM tbl_clinic
		WHERE practitioner_id = ? AND deleted_at IS NULL`

	query, filterArgs := common.BuildQuery(base, filter, clinicAllowedColumns, clinicSearchColumns, true)
	query = sqlx.Rebind(sqlx.DOLLAR, query)

	args := append([]interface{}{practitionerID}, filterArgs...)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count clinics by practitioner: %w", err)
	}
	return count, nil
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

func (r *repository) BulkDeleteClinics(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	query := `UPDATE tbl_clinic SET deleted_at = now() WHERE id = ANY($1) AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("bulk delete clinics: %w", err)
	}
	return nil
}

// Transaction-based methods
func (r *repository) CreateClinicTx(ctx context.Context, tx *sqlx.Tx, clinic *Clinic) (*Clinic, error) {
	query := `
        INSERT INTO tbl_clinic (practitioner_id, profile_picture, name, abn, description, is_active)
        VALUES ($1, $2, $3, $4, $5, COALESCE($6, TRUE))
        RETURNING id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
    `
	var c Clinic
	err := tx.QueryRowxContext(ctx, query, clinic.PractitionerID, clinic.ProfilePicture, clinic.Name,
		clinic.ABN, clinic.Description, clinic.IsActive,
	).StructScan(&c)
	if err != nil {
		return nil, fmt.Errorf("create clinic tx: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateClinicAddressTx(ctx context.Context, tx *sqlx.Tx, address *ClinicAddress) (*ClinicAddress, error) {
	query := `
		INSERT INTO tbl_clinic_address (clinic_id, address, city, state, postcode, is_primary)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, FALSE))
		RETURNING id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
	`
	var a ClinicAddress
	err := tx.QueryRowxContext(ctx, query,
		address.ClinicID, address.Address, address.City,
		address.State, address.Postcode, address.IsPrimary,
	).StructScan(&a)
	if err != nil {
		return nil, fmt.Errorf("create clinic address tx: %w", err)
	}
	return &a, nil
}

func (r *repository) CreateClinicContactTx(ctx context.Context, tx *sqlx.Tx, contact *ClinicContact) (*ClinicContact, error) {
	query := `
		INSERT INTO tbl_clinic_contact (clinic_id, contact_type, value, label, is_primary)
		VALUES ($1, $2, $3, $4, COALESCE($5, FALSE))
		RETURNING id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
	`
	var c ClinicContact
	err := tx.QueryRowxContext(ctx, query,
		contact.ClinicID, contact.ContactType, contact.Value,
		contact.Label, contact.IsPrimary,
	).StructScan(&c)
	if err != nil {
		return nil, fmt.Errorf("create clinic contact tx: %w", err)
	}
	return &c, nil
}

func (r *repository) CreateFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, settings *FinancialSettings) (*FinancialSettings, error) {
	query := `
		INSERT INTO tbl_financial_settings (clinic_id, financial_year_id, lock_date)
		VALUES ($1, $2, $3)
		RETURNING id, clinic_id, financial_year_id, lock_date, created_at, updated_at
	`
	var fs FinancialSettings
	err := tx.QueryRowxContext(ctx, query,
		settings.ClinicID, settings.FinancialYearID, settings.LockDate,
	).StructScan(&fs)
	if err != nil {
		return nil, fmt.Errorf("create financial settings tx: %w", err)
	}
	return &fs, nil
}

func (r *repository) GetActiveFinancialYearTx(ctx context.Context, tx *sqlx.Tx) (*uuid.UUID, error) {
	query := `SELECT id FROM tbl_financial_year WHERE is_active = TRUE LIMIT 1`
	var id uuid.UUID
	if err := tx.QueryRowxContext(ctx, query).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("no active financial year found")
		}
		return nil, fmt.Errorf("get active financial year tx: %w", err)
	}
	return &id, nil
}

func (r *repository) GetClinicByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*Clinic, error) {
	query := `
		SELECT id, practitioner_id,profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE id = $1 AND deleted_at IS NULL
	`
	var c Clinic
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get clinic by id tx: %w", err)
	}
	return &c, nil
}

func (r *repository) GetClinicByIDAndPractitionerTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, practitionerID uuid.UUID) (*Clinic, error) {
	query := `
		SELECT id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
		FROM tbl_clinic
		WHERE id = $1 AND practitioner_id = $2 AND deleted_at IS NULL
	`
	var c Clinic
	if err := tx.QueryRowxContext(ctx, query, id, practitionerID).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get clinic by id and practitioner tx: %w", err)
	}
	return &c, nil
}

func (r *repository) GetClinicAddressesTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) ([]ClinicAddress, error) {
	query := `
		SELECT id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
		FROM tbl_clinic_address
		WHERE clinic_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`
	var addresses []ClinicAddress
	if err := tx.SelectContext(ctx, &addresses, query, clinicID); err != nil {
		return nil, fmt.Errorf("get clinic addresses tx: %w", err)
	}
	return addresses, nil
}

func (r *repository) GetClinicContactsTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) ([]ClinicContact, error) {
	query := `
		SELECT id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
		FROM tbl_clinic_contact
		WHERE clinic_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`
	var contacts []ClinicContact
	if err := tx.SelectContext(ctx, &contacts, query, clinicID); err != nil {
		return nil, fmt.Errorf("get clinic contacts tx: %w", err)
	}
	return contacts, nil
}

func (r *repository) GetFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID) (*FinancialSettings, error) {
	query := `
		SELECT id, clinic_id, financial_year_id, lock_date, created_at, updated_at
		FROM tbl_financial_settings
		WHERE clinic_id = $1
	`
	var fs FinancialSettings
	if err := tx.QueryRowxContext(ctx, query, clinicID).StructScan(&fs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get financial settings tx: %w", err)
	}
	return &fs, nil
}

func (r *repository) GetAddressByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*ClinicAddress, error) {
	query := `
		SELECT id, clinic_id, address, city, state, postcode, is_primary, created_at, updated_at
		FROM tbl_clinic_address
		WHERE id = $1
	`
	var a ClinicAddress
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get address by id tx: %w", err)
	}
	return &a, nil
}

func (r *repository) GetContactByIDTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*ClinicContact, error) {
	query := `
		SELECT id, clinic_id, contact_type, value, label, is_primary, created_at, updated_at
		FROM tbl_clinic_contact
		WHERE id = $1
	`
	var c ClinicContact
	if err := tx.QueryRowxContext(ctx, query, id).StructScan(&c); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get contact by id tx: %w", err)
	}
	return &c, nil
}

func (r *repository) UpdateClinicTx(ctx context.Context, tx *sqlx.Tx, clinic *Clinic) (*Clinic, error) {
	query := `
		UPDATE tbl_clinic 
		SET practitioner_id = $1, profile_picture = $2, name = $3, abn = $4, 
		    description = $5, is_active = $6, updated_at = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING id, practitioner_id, profile_picture, name, abn, description, is_active, created_at, updated_at
	`
	var c Clinic
	err := tx.QueryRowxContext(ctx, query,
		clinic.PractitionerID, clinic.ProfilePicture, clinic.Name,
		clinic.ABN, clinic.Description, clinic.IsActive, clinic.ID,
	).StructScan(&c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update clinic tx: %w", err)
	}
	return &c, nil
}

func (r *repository) UpdateClinicAddressTx(ctx context.Context, tx *sqlx.Tx, address *ClinicAddress) error {
	query := `
		UPDATE tbl_clinic_address 
		SET address = $1, city = $2, state = $3, postcode = $4, is_primary = $5, updated_at = now()
		WHERE id = $6
	`
	_, err := tx.ExecContext(ctx, query,
		address.Address, address.City, address.State,
		address.Postcode, address.IsPrimary, address.ID,
	)
	if err != nil {
		return fmt.Errorf("update clinic address tx: %w", err)
	}
	return nil
}

func (r *repository) UpdateClinicContactTx(ctx context.Context, tx *sqlx.Tx, contact *ClinicContact) error {
	query := `
		UPDATE tbl_clinic_contact 
		SET value = $1, label = $2, is_primary = $3, updated_at = now()
		WHERE id = $4
	`
	_, err := tx.ExecContext(ctx, query,
		contact.Value, contact.Label, contact.IsPrimary, contact.ID,
	)
	if err != nil {
		return fmt.Errorf("update clinic contact tx: %w", err)
	}
	return nil
}

func (r *repository) UpdateFinancialSettingsTx(ctx context.Context, tx *sqlx.Tx, settings *FinancialSettings) error {
	query := `
		UPDATE tbl_financial_settings 
		SET financial_year_id = $1, lock_date = $2, updated_at = now()
		WHERE id = $3
	`
	_, err := tx.ExecContext(ctx, query,
		settings.FinancialYearID, settings.LockDate, settings.ID,
	)
	if err != nil {
		return fmt.Errorf("update financial settings tx: %w", err)
	}
	return nil
}

func (r *repository) UnsetPrimaryAddressTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, excludeID uuid.UUID) error {
	query := `UPDATE tbl_clinic_address SET is_primary = FALSE WHERE clinic_id = $1 AND id != $2`
	_, err := tx.ExecContext(ctx, query, clinicID, excludeID)
	if err != nil {
		return fmt.Errorf("unset primary address tx: %w", err)
	}
	return nil
}

func (r *repository) UnsetPrimaryContactTx(ctx context.Context, tx *sqlx.Tx, clinicID uuid.UUID, excludeID uuid.UUID) error {
	query := `UPDATE tbl_clinic_contact SET is_primary = FALSE WHERE clinic_id = $1 AND id != $2`
	_, err := tx.ExecContext(ctx, query, clinicID, excludeID)
	if err != nil {
		return fmt.Errorf("unset primary contact tx: %w", err)
	}
	return nil
}

func (r *repository) ListClinicByAccountant(ctx context.Context, accountantID uuid.UUID, filter common.Filter) ([]*Clinic, error) {
	// Join with tbl_invitation to get clinics for this accountant
	base := `
        SELECT 
            c.id, 
			c.practitioner_id,
            c.profile_picture, 
            c.name, 
            c.abn, 
            c.description, 
            c.is_active, 
            c.created_at, 
            c.updated_at
        FROM tbl_clinic c
        INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
        WHERE i.accountant_id = ? 
          AND i.status = 'COMPLETED'
          AND c.deleted_at IS NULL`

	baseArgs := []interface{}{accountantID}

	// If the accountant wants to filter by a specific practitioner they manage
	if filter.PractitionerID != nil && *filter.PractitionerID != uuid.Nil {
		base += ` AND c.practitioner_id = ?`
		baseArgs = append(baseArgs, *filter.PractitionerID)
	}

	// Use the same BuildQuery and Rebind logic as the Practitioner list
	query, filterArgs := common.BuildQuery(base, filter, clinicAllowedColumns, clinicSearchColumns, false)
	query = r.db.Rebind(query)

	var list []*Clinic
	if err := r.db.SelectContext(ctx, &list, query, append(baseArgs, filterArgs...)...); err != nil {
		return nil, fmt.Errorf("list clinics for accountant: %w", err)
	}
	return list, nil
}

func (r *repository) CountClinicByAccountant(ctx context.Context, actorID uuid.UUID, filter common.Filter) (int, error) {
	base := `
        FROM tbl_clinic c
        INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
        WHERE i.accountant_id = ? 
          AND i.status = 'COMPLETED'
          AND c.deleted_at IS NULL`

	query, filterArgs := common.BuildQuery(base, filter, clinicAllowedColumns, clinicSearchColumns, true)
	query = r.db.Rebind(query)

	args := append([]interface{}{actorID}, filterArgs...)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count clinics for accountant: %w", err)
	}
	return count, nil
}

func (r *repository) GetAccountantPermission(ctx context.Context, accountantID uuid.UUID, clinicID uuid.UUID) (*AccountantPermission, error) {
	var permission AccountantPermission

	// Check if accountant has access to this clinic through invitation
	query := `
        SELECT 
            i.practitioner_id, 
            $2 as clinic_id
        FROM tbl_invitation i
        INNER JOIN tbl_clinic c ON c.practitioner_id = i.practitioner_id
        WHERE i.accountant_id = $1 
          AND c.id = $2
          AND i.status = 'COMPLETED'
          AND c.deleted_at IS NULL 
        LIMIT 1`

	err := r.db.GetContext(ctx, &permission, query, accountantID, clinicID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active permission found for this accountant and clinic")
		}
		return nil, fmt.Errorf("querying accountant permissions: %w", err)
	}

	return &permission, nil
}

func (r *repository) IsClinicOwner(ctx context.Context, practitionerID uuid.UUID, clinicID uuid.UUID) (bool, error) {
	var exists bool
	// Adjust table/column names to match your schema (e.g., tbl_clinics)
	query := `SELECT EXISTS(SELECT 1 FROM tbl_clinic WHERE id = $1 AND practitioner_id = $2 AND deleted_at IS NULL)`

	err := r.db.GetContext(ctx, &exists, query, clinicID, practitionerID)
	if err != nil {
		return false, fmt.Errorf("checking clinic ownership: %w", err)
	}

	return exists, nil
}

func (r *repository) HasPermission(ctx context.Context, practitionerID, accountantID uuid.UUID, entityType string, entityID *uuid.UUID, requiredPerm string) (bool, error) {
	var exists bool
	// Check if accountant has access through completed invitation
	query := `
        SELECT EXISTS (
            SELECT 1 FROM tbl_invitation i
            INNER JOIN tbl_invite_permissions ip ON ip.invitation_id = i.id
            INNER JOIN tbl_permission p ON p.id = ip.permission_id
            WHERE i.practitioner_id = $1 
              AND i.accountant_id = $2 
              AND i.status = 'COMPLETED'
              AND p.name = $3
              AND (
                  ($3 = 'sales_purchases' AND ip.can_write = true) OR
                  ($3 = 'lock_dates' AND ip.can_write = true) OR
                  ($3 = 'manage_users' AND ip.can_write = true) OR
                  ($3 = 'reports_view_download' AND ip.can_read = true)
              )
        )`

	err := r.db.GetContext(ctx, &exists, query, practitionerID, accountantID, requiredPerm)
	fmt.Printf(">>> DB PERMISSION CHECK: [Prac: %s] [Acc: %s] [Perm: %s] [Result: %v]\n",
		practitionerID, accountantID, requiredPerm, exists)
	return exists, err
}

// DeletePermissionsByEntity - No longer needed with new schema
// Permissions are tied to invitations, not entities
func (r *repository) DeletePermissionsByEntity(ctx context.Context, entityID uuid.UUID, entityType string) error {
	// No-op: permissions are now managed at invitation level
	return nil
}

// DeletePermissionsByEntityTx - No longer needed with new schema
func (r *repository) DeletePermissionsByEntityTx(ctx context.Context, tx *sqlx.Tx, entityID uuid.UUID, entityType string) error {
	// No-op: permissions are now managed at invitation level
	return nil
}

func (r *repository) IsAccountantInvitedByPractitioner(ctx context.Context, accountantID uuid.UUID, practitionerID uuid.UUID) (bool, error) {
	query := `
        SELECT EXISTS (
            SELECT 1 
            FROM tbl_invitation 
            WHERE accountant_id = $1 
              AND practitioner_id = $2 
              AND status = 'COMPLETED'
        )`

	var exists bool
	err := r.db.GetContext(ctx, &exists, query, accountantID, practitionerID)
	return exists, err
}

func (r *repository) GetPractitionerForAccountant(ctx context.Context, accountantID uuid.UUID) (*uuid.UUID, error) {
	var practitionerID uuid.UUID
	query := `
        SELECT practitioner_id 
        FROM tbl_invitation 
        WHERE accountant_id = $1 
          AND status = 'COMPLETED'
        LIMIT 1` // Gets the first associated practitioner

	err := r.db.GetContext(ctx, &practitionerID, query, accountantID)
	return &practitionerID, err
}
