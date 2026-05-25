package accountant

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateAccountant(ctx context.Context, req *RqCreateAccountant, tx *sqlx.Tx) (*RsAccountant, error)
	GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error)
	UpdateAccountantProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateAccountant) error
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
		INSERT INTO tbl_accountant (user_id, entity_type, entity_name, abn, acn, address, tax_agent_number, profession)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, verified, entity_type, entity_name, abn, acn, tax_agent_number, address, profession
	`
	var a Accountant
	if err := tx.QueryRowxContext(ctx, query, req.UserID, req.EntityType, req.EntityName, req.ABN, req.ACN, req.Address, req.TaxAgentNumber, req.Profession).StructScan(&a); err != nil {
		return nil, err
	}

	settingQuery := `INSERT INTO tbl_accountant_setting (accountant_id, settings) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, settingQuery, a.ID, "{}"); err != nil {
		return nil, err
	}

	return r.mapToRsAccountant(&a), nil
}

func (r *repository) GetAccountantByUserID(ctx context.Context, userID string) (*RsAccountant, error) {
	query := `SELECT id, user_id, verified, entity_type, entity_name, address, abn, acn, profession, tax_agent_number FROM tbl_accountant WHERE user_id = $1 AND deleted_at IS NULL`
	var a Accountant
	if err := r.db.GetContext(ctx, &a, query, userID); err != nil {
		return nil, err
	}
	return r.mapToRsAccountant(&a), nil
}

func (r *repository) UpdateAccountantProfile(ctx context.Context, userID uuid.UUID, req *RqUpdateAccountant) error {
	query := `
		UPDATE tbl_accountant 
		SET 
			abn = COALESCE($1, abn),
			entity_type = CASE WHEN $2::text = '' THEN entity_type ELSE $2::business_entity_type END,
			entity_name = CASE WHEN $3 = '' THEN entity_name ELSE $3 END,
			acn = COALESCE($4, acn),
			address = COALESCE($5, address),
			profession = COALESCE($6, profession),
			tax_agent_number = COALESCE($7, tax_agent_number),
			updated_at = NOW()
		WHERE user_id = $8 AND deleted_at IS NULL`

	_, err := r.db.ExecContext(ctx, query, req.ABN, req.EntityType, req.EntityName, req.ACN, req.Address, req.Profession, req.TaxAgentNumber, userID)
	return err
}

func (r *repository) GetAllUsers(ctx context.Context, userID string) ([]RsAccountantUser, error) {
	var users []RsAccountantUser
	query := `
        SELECT 
            u.id, u.email, u.first_name, u.last_name, u.phone, u.created_at, u.updated_at,
            i.status AS invitation_status,
            COALESCE(
                (SELECT jsonb_agg(jsonb_build_object(
                    'name', c.name, 'abn', c.abn, 'description', c.description,
                    'addresses', (
                        SELECT COALESCE(jsonb_agg(jsonb_build_object(
                            'address', ca.address, 'city', ca.city, 'state', ca.state, 'postcode', ca.postcode, 'is_primary', ca.is_primary
                        )), '[]'::jsonb) FROM tbl_clinic_address ca WHERE ca.clinic_id = c.id
                    ),
                    'contacts', (
                        SELECT COALESCE(jsonb_agg(jsonb_build_object(
                            'type', cc.contact_type, 'value', cc.value, 'label', cc.label, 'is_primary', cc.is_primary
                        )), '[]'::jsonb) FROM tbl_clinic_contact cc WHERE cc.clinic_id = c.id
                    )
                )) FROM tbl_clinic c WHERE c.practitioner_id = i.practitioner_id AND c.deleted_at IS NULL
            ), '[]'::jsonb) AS clinics
        FROM tbl_user u
        INNER JOIN tbl_accountant a ON u.id = a.user_id
        INNER JOIN tbl_invitation i ON i.accountant_id = a.id
        WHERE a.id = $1 AND i.status = 'COMPLETED' AND u.deleted_at IS NULL 
        ORDER BY u.created_at DESC`

	if err := r.db.SelectContext(ctx, &users, query, userID); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *repository) GetClinicsForAccountant(ctx context.Context, accountantID string) ([]ClinicDetail, error) {
	var clinics []ClinicDetail
	query := `
        SELECT 
            c.name, c.abn, c.description,
            COALESCE((SELECT address FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as address,
            COALESCE((SELECT city FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as city,
            COALESCE((SELECT postcode FROM tbl_clinic_address WHERE clinic_id = c.id AND is_primary = true LIMIT 1), '') as postcode,
            (SELECT COALESCE(jsonb_agg(jsonb_build_object('type', cc.contact_type, 'value', cc.value, 'label', cc.label)), '[]'::jsonb) 
             FROM tbl_clinic_contact cc WHERE cc.clinic_id = c.id) as contacts
        FROM tbl_clinic c
        INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
        WHERE i.accountant_id = $1 AND c.deleted_at IS NULL`

	err := r.db.SelectContext(ctx, &clinics, query, accountantID)
	return clinics, err
}

func (r *repository) GetFormsForAccountant(ctx context.Context, accountantID string) ([]RsAccountantForm, error) {
	var forms []RsAccountantForm
	query := `
		SELECT 
			f.id, f.clinic_id, c.name as clinic_name, f.name, f.description, f.status, f.method, 
			f.owner_share, f.clinic_share, f.super_component, f.created_at, f.updated_at
		FROM tbl_form f
		INNER JOIN tbl_clinic c ON f.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		WHERE i.accountant_id = $1 AND f.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY f.created_at DESC`

	if err := r.db.SelectContext(ctx, &forms, query, accountantID); err != nil {
		return nil, err
	}
	return forms, nil
}

func (r *repository) GetSummary(ctx context.Context, accountantID string, ft common.Filter) (*Summary, error) {
	summary := &Summary{}
	conditions, args := r.buildAnalyticsFilters(accountantID, ft, "c.id", "f.id", "e.id")

	queries := map[string]string{
		"clinics":      fmt.Sprintf(`SELECT COUNT(DISTINCT c.id) FROM tbl_clinic c INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id %s AND i.status = 'COMPLETED' AND c.deleted_at IS NULL`, conditions["clinic"]),
		"forms":        fmt.Sprintf(`SELECT COUNT(DISTINCT f.id) FROM tbl_form f INNER JOIN tbl_clinic c ON f.clinic_id = c.id INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id %s AND i.status = 'COMPLETED' AND f.deleted_at IS NULL AND c.deleted_at IS NULL`, conditions["form"]),
		"transactions": fmt.Sprintf(`SELECT COUNT(DISTINCT e.id) FROM tbl_form_entry e INNER JOIN tbl_custom_form_version cfv ON e.form_version_id = cfv.id INNER JOIN tbl_form f ON cfv.form_id = f.id INNER JOIN tbl_clinic c ON f.clinic_id = c.id INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id %s AND i.status = 'COMPLETED' AND f.deleted_at IS NULL AND c.deleted_at IS NULL AND e.deleted_at IS NULL`, conditions["entry"]),
	}

	if err := r.db.GetContext(ctx, &summary.TotalClinics, r.db.Rebind(queries["clinics"]), args...); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &summary.TotalForms, r.db.Rebind(queries["forms"]), args...); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &summary.TotalTransactions, r.db.Rebind(queries["transactions"]), args...); err != nil {
		return nil, err
	}

	err := r.db.GetContext(ctx, &summary.TotalPractitioners, `SELECT COUNT(DISTINCT practitioner_id) FROM tbl_invitation WHERE accountant_id = $1 AND status = 'COMPLETED'`, accountantID)
	return summary, err
}

func (r *repository) GetRecentTransactions(ctx context.Context, accountantID string, ft common.Filter) ([]RecentTransaction, error) {
	transactions := []RecentTransaction{}
	conditions, args := r.buildAnalyticsFilters(accountantID, ft, "fe.clinic_id", "cfv.form_id", "fe.id")

	query := fmt.Sprintf(`
		SELECT 
			fev.id, fe.clinic_id, c.name as clinic_name, COALESCE(fev.gross_amount, 0) as amount,
			CASE WHEN fev.gross_amount > 0 THEN 'credit' ELSE 'debit' END as type,
			COALESCE(fev.date, fe.date, fev.created_at::date) as date,
			CASE WHEN fe.status = 'SUBMITTED' THEN 'completed' ELSE 'draft' END as status
		FROM tbl_form_entry_value fev
		INNER JOIN tbl_form_entry fe ON fev.entry_id = fe.id
		INNER JOIN tbl_clinic c ON fe.clinic_id = c.id
		INNER JOIN tbl_custom_form_version cfv ON fe.form_version_id = cfv.id
		INNER JOIN tbl_form f ON cfv.form_id = f.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		%s AND i.status = 'COMPLETED' AND fe.deleted_at IS NULL AND c.deleted_at IS NULL AND f.deleted_at IS NULL
		GROUP BY fev.id, fe.clinic_id, c.name, fev.gross_amount, fev.date, fe.date, fev.created_at, fe.status
		ORDER BY date DESC`, conditions["entry"])

	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &transactions, r.db.Rebind(query), args...)
	return transactions, err
}

func (r *repository) GetPractitioners(ctx context.Context, accountantID string, ft common.Filter) ([]Practitioner, error) {
	practitioners := []Practitioner{}
	conditions, args := r.buildAnalyticsFilters(accountantID, ft, "c.id", "f.id", "")

	query := fmt.Sprintf(`
		SELECT 
			p.id, CONCAT(u.first_name, ' ', u.last_name) as name, u.email, COUNT(DISTINCT c.id) as clinic_count,
			CASE WHEN u.deleted_at IS NULL THEN 'active' ELSE 'inactive' END as status
		FROM tbl_practitioner p
		JOIN tbl_user u ON p.user_id = u.id
		LEFT JOIN tbl_clinic c ON c.practitioner_id = p.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = p.id
		%s AND i.status = 'COMPLETED'
		GROUP BY p.id, u.first_name, u.last_name, u.email, u.deleted_at, p.created_at
		ORDER BY p.created_at DESC`, conditions["clinic"])

	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &practitioners, r.db.Rebind(query), args...)
	return practitioners, err
}

func (r *repository) GetClinics(ctx context.Context, accountantID string, ft common.Filter) ([]Clinic, error) {
	clinics := []Clinic{}
	conditions, args := r.buildAnalyticsFilters(accountantID, ft, "c.id", "", "")

	query := fmt.Sprintf(`
		SELECT c.id, c.name, COALESCE(ca.city, '') as location, c.created_at
		FROM tbl_clinic c
		LEFT JOIN tbl_clinic_address ca ON c.id = ca.clinic_id AND ca.is_primary = true
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		%s AND i.status = 'COMPLETED' AND c.deleted_at IS NULL
		ORDER BY c.created_at DESC`, conditions["clinic"])

	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &clinics, r.db.Rebind(query), args...)
	return clinics, err
}

func (r *repository) GetForms(ctx context.Context, accountantID string, ft common.Filter) ([]Form, error) {
	forms := []Form{}
	conditions, args := r.buildAnalyticsFilters(accountantID, ft, "f.clinic_id", "f.id", "")

	query := fmt.Sprintf(`
		SELECT f.id, f.name, f.clinic_id, COALESCE('v' || cfv.version::text, 'v1') as version, f.created_at
		FROM tbl_form f
		LEFT JOIN tbl_custom_form_version cfv ON f.id = cfv.form_id AND cfv.is_active = true
		INNER JOIN tbl_clinic c ON f.clinic_id = c.id
		INNER JOIN tbl_invitation i ON i.practitioner_id = c.practitioner_id
		%s AND i.status = 'COMPLETED' AND f.deleted_at IS NULL AND c.deleted_at IS NULL
		ORDER BY f.created_at DESC`, conditions["form"])

	if ft.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *ft.Limit)
	}

	err := r.db.SelectContext(ctx, &forms, r.db.Rebind(query), args...)
	return forms, err
}

func (r *repository) buildAnalyticsFilters(accountantID string, ft common.Filter, clinicField, formField, entryField string) (map[string]string, []any) {
	baseClauses := []string{"i.accountant_id = ?"}
	args := []any{accountantID}

	clinicClauses := append([]string{}, baseClauses...)
	formClauses := append([]string{}, baseClauses...)
	entryClauses := append([]string{}, baseClauses...)

	// Safely iterate through conditions inside standard ft.Where structure
	for _, condition := range ft.Where {
		switch condition.Field {
		case "clinic_id":
			if clinicField != "" {
				clinicClauses = append(clinicClauses, fmt.Sprintf("%s = ?", clinicField))
			}
			if formField != "" {
				formClauses = append(formClauses, fmt.Sprintf("%s = ?", clinicField))
			}
			if entryField != "" {
				entryClauses = append(entryClauses, fmt.Sprintf("%s = ?", clinicField))
			}
			args = append(args, condition.Value)
		case "form_id":
			if formField != "" {
				formClauses = append(formClauses, fmt.Sprintf("%s = ?", formField))
			}
			if entryField != "" {
				entryClauses = append(entryClauses, fmt.Sprintf("%s = ?", formField))
			}
			args = append(args, condition.Value)
		case "practitioner_id":
			clause := "i.practitioner_id = ?"
			clinicClauses = append(clinicClauses, clause)
			formClauses = append(formClauses, clause)
			entryClauses = append(entryClauses, clause)
			args = append(args, condition.Value)
		}
	}

	return map[string]string{
		"clinic": "WHERE " + strings.Join(clinicClauses, " AND "),
		"form":   "WHERE " + strings.Join(formClauses, " AND "),
		"entry":  "WHERE " + strings.Join(entryClauses, " AND "),
	}, args
}

func (r *repository) mapToRsAccountant(a *Accountant) *RsAccountant {
	return &RsAccountant{
		ID:             a.ID,
		UserID:         a.UserID.String(),
		Verified:       a.Verified,
		EntityType:     a.EntityType,
		EntityName:     a.EntityName,
		ABN:            a.ABN,
		ACN:            a.ACN,
		Address:        a.Address,
		Profession:     a.Profession,
		TaxAgentNumber: a.TaxAgentNumber,
	}
}
