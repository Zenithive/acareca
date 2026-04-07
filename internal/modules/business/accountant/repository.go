package accountant

import (
	"context"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)

	GetAllUsers(ctx context.Context, userID string) ([]RsAccountantUser, error)
	GetClinicsForAccountant(ctx context.Context, accountantID string) ([]ClinicDetail, error)
	GetFormsForAccountant(ctx context.Context, accountantID string) ([]RsAccountantForm, error)

	GetSummary(ctx context.Context, accountantID string, ft common.Filter) (*Summary, error)
	GetRecentTransactions(ctx context.Context, accountantID string, ft common.Filter) ([]RecentTransaction, error)
	GetPractitioners(ctx context.Context, accountantID string, ft common.Filter) ([]Practitioner, error)
	GetClinics(ctx context.Context, accountantID string, ft common.Filter) ([]Clinic, error)
	GetForms(ctx context.Context, accountantID string, ft common.Filter) ([]Form, error)
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

func (r *repository) GetSummary(ctx context.Context, accountantID string, ft common.Filter) (*Summary, error) {
	summary := &Summary{}

	// Get total clinics associated with this accountant
	err := r.db.GetContext(ctx, &summary.TotalClinics,
		`SELECT COUNT(DISTINCT c.id) 
		FROM tbl_clinic c
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND c.deleted_at IS NULL`, accountantID)
	if err != nil {
		return nil, err
	}

	// Get total forms associated with this accountant's clinics
	err = r.db.GetContext(ctx, &summary.TotalForms,
		`SELECT COUNT(DISTINCT f.id) 
		FROM tbl_form f
		INNER JOIN tbl_clinic c ON f.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND f.deleted_at IS NULL
		  AND c.deleted_at IS NULL`, accountantID)
	if err != nil {
		return nil, err
	}

	// Get total transactions (form entries) for this accountant's clinics
	err = r.db.GetContext(ctx, &summary.TotalTransactions,
		`SELECT COUNT(*) 
		FROM tbl_form_entry e
		INNER JOIN tbl_clinic c ON e.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND c.deleted_at IS NULL`, accountantID)
	if err != nil {
		return nil, err
	}

	// Get total practitioners associated with this accountant
	err = r.db.GetContext(ctx, &summary.TotalPractitioners,
		`SELECT COUNT(DISTINCT practitioner_id) 
		FROM tbl_invitation 
		WHERE entity_id = $1 
		  AND status = 'COMPLETED'`, accountantID)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (r *repository) GetRecentTransactions(ctx context.Context, accountantID string, ft common.Filter) ([]RecentTransaction, error) {
	transactions := []RecentTransaction{}

	query := `
		SELECT 
			fev.id,
			fe.clinic_id,
			c.name as clinic_name,
			COALESCE(fev.gross_amount, 0) as amount,
			CASE WHEN fev.gross_amount > 0 THEN 'credit' ELSE 'debit' END as type,
			fev.created_at as date,
			CASE WHEN fe.status = 'SUBMITTED' THEN 'completed' ELSE 'draft' END as status
		FROM tbl_form_entry_value fev
		JOIN tbl_form_entry fe ON fev.entry_id = fe.id
		JOIN tbl_clinic c ON fe.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND c.deleted_at IS NULL
		ORDER BY fev.created_at DESC
	`

	// Apply limit if provided
	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &transactions, query, accountantID)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r *repository) GetPractitioners(ctx context.Context, accountantID string, ft common.Filter) ([]Practitioner, error) {
	practitioners := []Practitioner{}

	query := `
		SELECT 
			p.id,
			CONCAT(u.first_name, ' ', u.last_name) as name,
			u.email,
			COUNT(DISTINCT c.id) as clinic_count,
			CASE WHEN u.deleted_at IS NULL THEN 'active' ELSE 'inactive' END as status
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		LEFT JOIN tbl_clinic c ON c.practitioner_id = p.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = p.id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		GROUP BY p.id, u.first_name, u.last_name, u.email, u.deleted_at
		ORDER BY p.created_at DESC
	`

	// Apply limit if provided
	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &practitioners, query, accountantID)
	if err != nil {
		return nil, err
	}

	return practitioners, nil
}

func (r *repository) GetClinics(ctx context.Context, accountantID string, ft common.Filter) ([]Clinic, error) {
	clinics := []Clinic{}

	query := `
		SELECT 
			c.id,
			c.name,
			COALESCE(ca.city, '') as location,
			c.created_at
		FROM tbl_clinic c
		LEFT JOIN tbl_clinic_address ca ON c.id = ca.clinic_id AND ca.is_primary = true
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC
	`

	// Apply limit if provided
	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &clinics, query, accountantID)
	if err != nil {
		return nil, err
	}

	return clinics, nil
}

func (r *repository) GetForms(ctx context.Context, accountantID string, ft common.Filter) ([]Form, error) {
	forms := []Form{}

	query := `
		SELECT 
			f.id,
			f.name,
			f.clinic_id,
			COALESCE('v' || cfv.version::text, 'v1') as version,
			f.created_at
		FROM tbl_form f
		LEFT JOIN tbl_custom_form_version cfv ON f.id = cfv.form_id AND cfv.is_active = true
		INNER JOIN tbl_clinic c ON f.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.entity_id = $1 
		  AND i.status = 'COMPLETED'
		  AND f.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		ORDER BY f.created_at DESC
	`

	// Apply limit if provided
	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &forms, query, accountantID)
	if err != nil {
		return nil, err
	}

	return forms, nil
}
