package accountant

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)

	GetAllUsers(ctx context.Context, userID string) ([]RsAccountantUser, error)
	GetClinicsForAccountant(ctx context.Context, accountantID string) ([]ClinicDetail, error)
	GetFormsForAccountant(ctx context.Context, accountantID string) ([]RsAccountantForm, error)
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

func (r *repository) GetAllUsers(ctx context.Context, userID string) ([]RsAccountantUser, error) {
	var users []RsAccountantUser

	query := `
        SELECT 
            u.id, u.email, u.first_name, u.last_name, u.phone, 
            u.created_at, u.updated_at,
            i.status AS invitation_status,
            COALESCE(
                (SELECT jsonb_agg(jsonb_build_object(
                    'name', c.name,
                    'abn', c.abn,
                    'description', c.description,
                    'addresses', (
                        SELECT COALESCE(jsonb_agg(jsonb_build_object(
                            'address', ca.address,
                            'city', ca.city,
                            'state', ca.state,
                            'postcode', ca.postcode,
                            'is_primary', ca.is_primary
                        )), '[]'::jsonb)
                        FROM tbl_clinic_address ca 
                        WHERE ca.clinic_id = c.id
                    ),
                    'contacts', (
                        SELECT COALESCE(jsonb_agg(jsonb_build_object(
                            'type', cc.contact_type,
                            'value', cc.value,
                            'label', cc.label,
                            'is_primary', cc.is_primary
                        )), '[]'::jsonb)
                        FROM tbl_clinic_contact cc 
                        WHERE cc.clinic_id = c.id
                    )
                ))
                FROM tbl_clinic c 
                WHERE c.practitioner_id = i.practitioner_id
                AND c.deleted_at IS NULL
            ), '[]'::jsonb) AS clinics
        FROM tbl_user u
        INNER JOIN tbl_accountant a ON u.id = a.user_id
        INNER JOIN tbl_invitation i ON i.entity_id = a.id
        WHERE a.id = $1                   
          AND i.status = 'COMPLETED'
          AND u.deleted_at IS NULL 
        ORDER BY u.created_at DESC`

	err := r.db.SelectContext(ctx, &users, query, userID)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) GetClinicsForAccountant(ctx context.Context, accountantID string) ([]ClinicDetail, error) {
	var clinics []ClinicDetail
	query := `
        SELECT 
            c.name, 
            c.abn, 
            c.description,
            -- Get the primary address
            COALESCE((SELECT address FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as address,
            COALESCE((SELECT city FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as city,
            COALESCE((SELECT postcode FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as postcode,
            -- Get all contacts as JSON
            (SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'type', cc.contact_type,
                'value', cc.value,
                'label', cc.label
            )), '[]'::jsonb) FROM tbl_clinic_contact cc WHERE cc.clinic_id = c.id) as contacts
        FROM tbl_clinic c
        INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
        WHERE i.entity_id = $1 
          AND c.deleted_at IS NULL
    `
	err := r.db.SelectContext(ctx, &clinics, query, accountantID)
	return clinics, err
}

func (r *repository) GetFormsForAccountant(ctx context.Context, accountantID string) ([]RsAccountantForm, error) {
	var forms []RsAccountantForm
	query := `
		SELECT 
			f.id, 
			f.clinic_id, 
			c.name as clinic_name,
			f.name, 
			f.description,
			f.status, 
			f.method, 
			f.owner_share, 
			f.clinic_share, 
			f.super_component,
			f.created_at, 
			f.updated_at
		FROM tbl_form f
		INNER JOIN tbl_clinic c ON f.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND f.deleted_at IS NULL 
		  AND c.deleted_at IS NULL
		ORDER BY f.created_at DESC
	`
	err := r.db.SelectContext(ctx, &forms, query, accountantID)
	if err != nil {
		return nil, err
	}
	return forms, nil
}
