package entry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form entry not found")

type IRepository interface {
	Create(ctx context.Context, e *FormEntry, values []*FormEntryValue) error
	GetByID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)
	Update(ctx context.Context, e *FormEntry, values []*FormEntryValue) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*FormEntry, error)
	CountByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) (int, error)
	HasSubmittedEntryValuesForField(ctx context.Context, formFieldID uuid.UUID) (bool, error)

	GetByVersionID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)

	ListTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsTransactionRow, error)
	CountTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error)

	// COA-grouped endpoints
	ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error)
	CountCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error)
	ListCoaEntryDetails(ctx context.Context, coaID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntryDetail, error)
	CountCoaEntryDetails(ctx context.Context, coaID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) (int, error)

	// Transaction-based variants
	CreateTx(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error
	UpdateTx(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error
	DeleteTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error

	GetSummedValuesByFieldID(ctx context.Context, fieldID uuid.UUID) (*RsFieldSummary, error)
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) IRepository {
	return &Repository{db: db}
}

// Create implements [IRepository].
func (r *Repository) Create(ctx context.Context, e *FormEntry, values []*FormEntryValue) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO tbl_form_entry (id, form_version_id, clinic_id, submitted_by, submitted_at, status, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		e.ID, e.FormVersionID, e.ClinicID, e.SubmittedBy, e.SubmittedAt, e.Status, e.Date,
	).Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		return fmt.Errorf("create form entry: %w", err)
	}

	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description).
			Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
			return fmt.Errorf("create entry value: %w", err)
		}
	}

	return tx.Commit()
}

// GetByID implements [IRepository].
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error) {
	query := `SELECT id, form_version_id, clinic_id, submitted_by, submitted_at, status, date, created_at, updated_at
		FROM tbl_form_entry WHERE id = $1 AND deleted_at IS NULL`
	var e FormEntry
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.Date, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	valQuery := `SELECT id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, created_at, updated_at
		FROM tbl_form_entry_value
		WHERE entry_id = $1 AND updated_at IS NULL
		`
	var values []*FormEntryValue
	if err := r.db.SelectContext(ctx, &values, valQuery, id); err != nil {
		return nil, nil, fmt.Errorf("get entry values: %w", err)
	}
	return &e, values, nil
}

// Update implements [IRepository].
func (r *Repository) Update(ctx context.Context, e *FormEntry, values []*FormEntryValue) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Update the parent entry
	query := `
        UPDATE tbl_form_entry
        SET submitted_by = $1, submitted_at = $2, status = $3, date = $4, updated_at = now()
        WHERE id = $5 AND deleted_at IS NULL
        RETURNING created_at, updated_at
    `
	if err := tx.QueryRowContext(ctx, query, e.SubmittedBy, e.SubmittedAt, e.Status, e.Date, e.ID).
		Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update form entry: %w", err)
	}

	// Only handle field values if new values were provided
	// If 'values' is nil or empty, we skip this to avoid duplicating existing IDs
	if len(values) > 0 {
		// Mark previous values as "updated"
		markOldQuery := `
        UPDATE tbl_form_entry_value 
        SET updated_at = now() 
        WHERE entry_id = $1 AND updated_at IS NULL
    `
		if _, err := tx.ExecContext(ctx, markOldQuery, e.ID); err != nil {
			return fmt.Errorf("old entry values: %w", err)
		}

		for _, v := range values {
			v.EntryID = e.ID
			valQuery := `
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL)
			RETURNING created_at
		`
			if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description).
				Scan(&v.CreatedAt); err != nil {
				return fmt.Errorf("insert entry value: %w", err)
			}
			v.UpdatedAt = nil // Set to nil so the API response shows it as null for the new record
		}
	}

	return tx.Commit()
}

// Delete implements [IRepository].
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE tbl_form_entry SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form entry: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByFormVersionID implements [IRepository].
func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*FormEntry, error) {
	var permissionClause string
	// 1. Define the permission check (Same as your Transactions logic)
	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND c.practitioner_id IN (
            SELECT practitioner_id FROM tbl_invitation 
            WHERE accountant_id = ? AND status = 'COMPLETED'
        )`
	} else {
		permissionClause = ` AND c.id IN (
            SELECT id FROM tbl_clinic 
            WHERE practitioner_id = ? AND deleted_at IS NULL
        )`
	}
	allowedColumns := map[string]string{
		"clinic_id":  "clinic_id",
		"created_at": "created_at",
		"status":     "status",
	}

	base := `SELECT e.id, e.form_version_id, e.clinic_id, e.submitted_by, e.submitted_at, e.status, e.date, e.created_at, e.updated_at
        FROM tbl_form_entry e
        INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id
        INNER JOIN tbl_form                fm ON fm.id = fv.form_id
        INNER JOIN tbl_clinic              c  ON c.id  = e.clinic_id
        WHERE e.form_version_id = ? 
        AND e.deleted_at IS NULL` + permissionClause

	q, qArgs := common.BuildQuery(base, f, allowedColumns, []string{"e.status"}, false)

	args := []interface{}{formVersionID, actorID}
	args = append(args, qArgs...)

	q = r.db.Rebind(q)

	var list []*FormEntry
	if err := r.db.SelectContext(ctx, &list, q, args...); err != nil {
		return nil, fmt.Errorf("list form entries: %w", err)
	}
	return list, nil
}

// CountByFormVersionID implements [IRepository].
func (r *Repository) CountByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	var permissionClause string
	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND c.practitioner_id IN (
            SELECT practitioner_id FROM tbl_invitation 
            WHERE accountant_id = ? AND status = 'COMPLETED'
        )`
	} else {
		permissionClause = ` AND c.id IN (
            SELECT id FROM tbl_clinic 
            WHERE practitioner_id = ? AND deleted_at IS NULL
        )`
	}

	allowedColumns := map[string]string{
		"clinic_id":  "clinic_id",
		"created_at": "created_at",
		"status":     "status",
	}

	base := `FROM tbl_form_entry e
        INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id
        INNER JOIN tbl_form                fm ON fm.id = fv.form_id
        INNER JOIN tbl_clinic              c  ON c.id  = e.clinic_id
        WHERE e.form_version_id = ? 
        AND e.deleted_at IS NULL` + permissionClause

	q, qArgs := common.BuildQuery(base, f, allowedColumns, []string{"e.status"}, true)
	args := []interface{}{formVersionID, actorID}
	args = append(args, qArgs...)

	q = r.db.Rebind(q)
	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count form entries: %w", err)
	}
	return total, nil
}

// HasSubmittedEntryValuesForField implements [IRepository]. Returns true if the field has any entry values in SUBMITTED entries.
func (r *Repository) HasSubmittedEntryValuesForField(ctx context.Context, formFieldID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS (
		SELECT 1 FROM tbl_form_entry_value v
		INNER JOIN tbl_form_entry e ON e.id = v.entry_id AND e.deleted_at IS NULL
		WHERE v.form_field_id = $1 AND e.status = $2
	)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, formFieldID, EntryStatusSubmitted).Scan(&exists); err != nil {
		return false, fmt.Errorf("has submitted entry values for field: %w", err)
	}
	return exists, nil
}

func (r *Repository) GetByVersionID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error) {
	query := `SELECT id, form_version_id, clinic_id, submitted_by, submitted_at, status, date, created_at, updated_at
		FROM tbl_form_entry WHERE form_version_id = $1 AND deleted_at IS NULL`
	var e FormEntry
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.Date, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	valQuery := `SELECT id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, created_at, updated_at
		FROM tbl_form_entry_value
		WHERE entry_id = $1 AND updated_at IS NULL
		`
	var values []*FormEntryValue
	if err := r.db.SelectContext(ctx, &values, valQuery, e.ID); err != nil {
		return nil, nil, fmt.Errorf("get entry values: %w", err)
	}
	return &e, values, nil
}

var allowedTransactionColumns = map[string]string{
	"clinic_id":       "e.clinic_id",
	"version_id":      "e.form_version_id",
	"form_id":         "fm.id",
	"coa_id":          "ff.coa_id",
	"tax_type_id":     "at2.id",
	"status":          "e.status",
	"created_at":      "ev.created_at",
	"practitioner_id": "c.practitioner_id",
	"date_from":       "COALESCE(e.date, ev.created_at)",
	"date_to":         "COALESCE(e.date, ev.created_at)",
	"date":            "e.date",
}

func (r *Repository) ListTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsTransactionRow, error) {
	// Build permission clause based on role
	var permissionClause string
	if strings.EqualFold(role, util.RoleAccountant) {
		// Accountant: show transactions for invited practitioners (clinic-based + expenses)
		permissionClause = ` AND (
			c.practitioner_id IN (SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED')
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	} else {
		// Practitioner: show own clinic transactions + own expenses
		permissionClause = ` AND (
			c.practitioner_id = ?
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	base := `
		SELECT
			ev.id,
			e.id AS entry_id,
			ff.id AS form_field_id,
			ff.label AS form_field_name,
			coa.id AS coa_id,
			coa.name AS coa_name,
			at2.id AS tax_type_id,
			at2.name AS tax_type_name,
			fm.id AS form_id,
			fm.name AS form_name,
			e.clinic_id,
			COALESCE(c.name, 'Expense') AS clinic_name,
			ev.net_amount,
			ev.gst_amount,
			ev.gross_amount,
			ev.created_at,
			ev.updated_at,
			e.date,
			(e.clinic_id = '00000000-0000-0000-0000-000000000000') AS is_expense
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry e ON e.id = ev.entry_id AND e.deleted_at IS NULL
		INNER JOIN tbl_form_field ff ON ff.id = ev.form_field_id AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL AND coa.is_system = FALSE
		LEFT JOIN tbl_account_tax at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		LEFT JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL AND ev.updated_at IS NULL` + permissionClause

	searchCols := []string{"ff.label", "coa.name", "fm.name", "c.name"}
	q, qArgs := common.BuildQuery(base, f, allowedTransactionColumns, searchCols, false)
	args := []any{actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var rows []*transactionFlatRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	result := make([]*RsTransactionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, &RsTransactionRow{
			ID:            row.ID,
			EntryID:       row.EntryID,
			FormFieldID:   row.FormFieldID,
			FormFieldName: row.FormFieldName,
			CoaID:         row.CoaID,
			CoaName:       row.CoaName,
			TaxTypeID:     row.TaxTypeID,
			TaxTypeName:   row.TaxTypeName,
			FormID:        row.FormID,
			FormName:      row.FormName,
			ClinicID:      row.ClinicID,
			ClinicName:    row.ClinicName,
			NetAmount:     row.NetAmount,
			GstAmount:     row.GstAmount,
			GrossAmount:   row.GrossAmount,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			Date:          row.Date,
			IsExpense:     row.IsExpense,
		})
	}
	return result, nil
}

func (r *Repository) CountTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	// Build permission clause based on role
	var permissionClause string
	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND (
			c.practitioner_id IN (SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED')
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	} else {
		permissionClause = ` AND (
			c.practitioner_id = ?
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	base := `
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry e ON e.id = ev.entry_id AND e.deleted_at IS NULL
		INNER JOIN tbl_form_field ff ON ff.id = ev.form_field_id AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL AND coa.is_system = FALSE
		LEFT JOIN tbl_account_tax at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		LEFT JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL AND ev.updated_at IS NULL` + permissionClause

	searchCols := []string{"ff.label", "coa.name", "fm.name", "c.name"}
	q, qArgs := common.BuildQuery(base, f, allowedTransactionColumns, searchCols, true)
	args := []any{actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count transactions: %w", err)
	}
	return total, nil
}

// CreateTx - Transaction variant of Create
func (r *Repository) CreateTx(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error {
	query := `
		INSERT INTO tbl_form_entry (id, form_version_id, clinic_id, submitted_by, submitted_at, status, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		e.ID, e.FormVersionID, e.ClinicID, e.SubmittedBy, e.SubmittedAt, e.Status, e.Date,
	).Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		return fmt.Errorf("create form entry tx: %w", err)
	}

	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description).
			Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
			return fmt.Errorf("create entry value tx: %w", err)
		}
	}

	return nil
}

// UpdateTx - Transaction variant of Update
func (r *Repository) UpdateTx(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error {
	// Update the parent entry
	query := `
        UPDATE tbl_form_entry
        SET submitted_by = $1, submitted_at = $2, status = $3, date = $4, updated_at = now()
        WHERE id = $5 AND deleted_at IS NULL
        RETURNING created_at, updated_at
    `
	if err := tx.QueryRowContext(ctx, query, e.SubmittedBy, e.SubmittedAt, e.Status, e.Date, e.ID).
		Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update form entry tx: %w", err)
	}

	// Mark previous active values as "updated"
	markOldQuery := `
        UPDATE tbl_form_entry_value 
        SET updated_at = now() 
        WHERE entry_id = $1 AND updated_at IS NULL
    `
	if _, err := tx.ExecContext(ctx, markOldQuery, e.ID); err != nil {
		return fmt.Errorf("mark old entry values tx: %w", err)
	}

	// Insert new values as the current active records (updated_at stays NULL)
	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
            INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL)
            RETURNING created_at
        `
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description).
			Scan(&v.CreatedAt); err != nil {
			return fmt.Errorf("insert entry value tx: %w", err)
		}
		v.UpdatedAt = nil
	}

	return nil
}

// DeleteTx - Transaction variant of Delete
func (r *Repository) DeleteTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	query := `UPDATE tbl_form_entry SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form entry tx: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetSummedValuesByFieldID(ctx context.Context, fieldID uuid.UUID) (*RsFieldSummary, error) {
	query := `
		SELECT 
			ff.id,
			ff.label,
			ff.section_type,
			ff.payment_responsibility,
			ff.tax_type,
			COALESCE(SUM(ev.net_amount), 0)   AS total_net,
			COALESCE(SUM(ev.gst_amount), 0)   AS total_gst,
			COALESCE(SUM(ev.gross_amount), 0) AS total_gross
		FROM tbl_form_field ff
		LEFT JOIN tbl_form_entry_value ev ON ev.form_field_id = ff.id AND ev.updated_at IS NULL
		WHERE ff.id = $1 AND ff.deleted_at IS NULL
		GROUP BY ff.id, ff.label, ff.section_type, ff.payment_responsibility, ff.tax_type`

	var summary RsFieldSummary
	err := r.db.QueryRowContext(ctx, query, fieldID).Scan(
		&summary.FormFieldID,
		&summary.Label,
		&summary.SectionType,
		&summary.Responsibility,
		&summary.TaxType,
		&summary.TotalNet,
		&summary.TotalGst,
		&summary.TotalGross,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repository sum field values with metadata: %w", err)
	}

	return &summary, nil
}

// ListCoaEntries returns grouped COA rows with aggregated amounts and entry counts
func (r *Repository) ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		// Accountant: show clinic entries they have access to + expense entries from those practitioners
		permissionClause = ` AND (
			c.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation 
				WHERE accountant_id = ? AND status = 'COMPLETED'
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation 
				WHERE accountant_id = ? AND status = 'COMPLETED'
			))
		)`
	} else {
		// Practitioner: show own clinic entries + own expense entries
		permissionClause = ` AND (
			c.id IN (
				SELECT id FROM tbl_clinic 
				WHERE practitioner_id = ? AND deleted_at IS NULL
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	// Map filter fields to use start_date/end_date instead of date_from/date_to
	allowedColumns := map[string]string{
		"clinic_id":       "e.clinic_id",
		"form_id":         "fm.id",
		"coa_id":          "coa.id",
		"tax_type_id":     "at2.id",
		"practitioner_id": "COALESCE(c.practitioner_id, fv.practitioner_id)",
		"start_date":      "ev.created_at",
		"end_date":        "ev.created_at",
	}

	base := `
		SELECT
			coa.id          AS coa_id,
			coa.name        AS coa_name,
			COALESCE(SUM(ev.net_amount), 0)   AS total_net_amount,
			COALESCE(SUM(ev.gross_amount), 0) AS total_gross_amount,
			COUNT(DISTINCT ev.id)             AS entry_count
		FROM tbl_chart_of_accounts coa
		INNER JOIN tbl_form_field              ff  ON ff.coa_id = coa.id         AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_form_entry_value        ev  ON ev.form_field_id = ff.id   AND ev.updated_at IS NULL
		INNER JOIN tbl_form_entry              e   ON e.id = ev.entry_id         AND e.deleted_at IS NULL
		INNER JOIN tbl_custom_form_version     fv  ON fv.id = e.form_version_id  AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id = fv.form_id         AND fm.deleted_at IS NULL
		LEFT  JOIN tbl_clinic                  c   ON c.id = e.clinic_id         AND c.deleted_at IS NULL
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		WHERE coa.deleted_at IS NULL AND coa.is_system = FALSE` + permissionClause

	searchCols := []string{"coa.name"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, false)
	groupByClause := ` GROUP BY coa.id, coa.name`
	args := []any{actorID, actorID}

	args = append(args, qArgs...)
	if strings.Contains(q, "ORDER BY") {
		q = strings.Replace(q, "ORDER BY", groupByClause+" ORDER BY", 1)
	} else if strings.Contains(q, "LIMIT") {
		q = strings.Replace(q, "LIMIT", groupByClause+" LIMIT", 1)
	} else {
		q += groupByClause
	}
	q = r.db.Rebind(q)

	type coaEntryRow struct {
		CoaID            uuid.UUID `db:"coa_id"`
		CoaName          string    `db:"coa_name"`
		TotalNetAmount   float64   `db:"total_net_amount"`
		TotalGrossAmount float64   `db:"total_gross_amount"`
		EntryCount       int       `db:"entry_count"`
	}

	var rows []*coaEntryRow

	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("list coa entries: %w", err)
	}

	result := make([]*RsCoaEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &RsCoaEntry{
			CoaID:            row.CoaID.String(),
			CoaName:          row.CoaName,
			TotalNetAmount:   row.TotalNetAmount,
			TotalGrossAmount: row.TotalGrossAmount,
			EntryCount:       row.EntryCount,
		})
	}
	return result, nil
}

// CountCoaEntries returns the total number of grouped COA rows
func (r *Repository) CountCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		// Accountant: show clinic entries they have access to + expense entries from those practitioners
		permissionClause = ` AND (
			c.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation
				WHERE accountant_id = ? AND status = 'COMPLETED'
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation
				WHERE accountant_id = ? AND status = 'COMPLETED'
			))
		)`
	} else {
		// Practitioner: show own clinic entries + own expense entries
		permissionClause = ` AND (
			c.id IN (
				SELECT id FROM tbl_clinic
				WHERE practitioner_id = ? AND deleted_at IS NULL
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "e.clinic_id",
		"form_id":         "fm.id",
		"coa_id":          "coa.id",
		"tax_type_id":     "at2.id",
		"practitioner_id": "COALESCE(c.practitioner_id, fv.practitioner_id)",
		"start_date":      "ev.created_at",
		"end_date":        "ev.created_at",
	}

	base := `
		FROM tbl_chart_of_accounts coa
		INNER JOIN tbl_form_field              ff  ON ff.coa_id = coa.id         AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_form_entry_value        ev  ON ev.form_field_id = ff.id   AND ev.updated_at IS NULL
		INNER JOIN tbl_form_entry              e   ON e.id = ev.entry_id         AND e.deleted_at IS NULL
		INNER JOIN tbl_custom_form_version     fv  ON fv.id = e.form_version_id  AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id = fv.form_id         AND fm.deleted_at IS NULL
		LEFT  JOIN tbl_clinic                  c   ON c.id = e.clinic_id         AND c.deleted_at IS NULL
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		WHERE coa.deleted_at IS NULL AND coa.is_system = FALSE` + permissionClause

	searchCols := []string{"coa.name"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, true)
	args := []any{actorID, actorID}
	if strings.Contains(strings.ToUpper(q), "COUNT(*)") {
		q = strings.ReplaceAll(q, "COUNT(*)", "COUNT(DISTINCT coa.id)")
	}

	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count coa entries: %w", err)
	}
	return total, nil
}

// ListCoaEntryDetails returns detailed entry rows for a specific COA
func (r *Repository) ListCoaEntryDetails(ctx context.Context, coaID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntryDetail, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		// Accountant: show clinic entries they have access to + expense entries from those practitioners
		permissionClause = ` AND (
			c.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation 
				WHERE accountant_id = ? AND status = 'COMPLETED'
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation 
				WHERE accountant_id = ? AND status = 'COMPLETED'
			))
		)`
	} else {
		// Practitioner: show own clinic entries + own expense entries
		permissionClause = ` AND (
			c.id IN (
				SELECT id FROM tbl_clinic 
				WHERE practitioner_id = ? AND deleted_at IS NULL
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "e.clinic_id",
		"form_id":         "fm.id",
		"tax_type_id":     "at2.id",
		"practitioner_id": "COALESCE(c.practitioner_id, fv.practitioner_id)",
		"start_date":      "ev.created_at",
		"end_date":        "ev.created_at",
		"created_at":      "ev.created_at",
	}

	base := `
		SELECT
			ev.id,
			e.id            AS entry_id,
			ff.id           AS form_field_id,
			coa.id          AS coa_id,
			at2.id          AS tax_type_id,
			fm.id           AS form_id,
			e.clinic_id,
			e.form_version_id AS version_id,
			ff.label        AS form_field_name,
			coa.name        AS coa_name,
			at2.name        AS tax_type_name,
			fm.name         AS form_name,
			fm.method       AS form_method,
			COALESCE(c.name, 'Expense') AS clinic_name,
			ev.net_amount,
			ev.gst_amount,
			ev.gross_amount,
			COALESCE(e.date, ev.created_at) AS created_at,
			ev.updated_at
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry              e   ON e.id   = ev.entry_id          AND e.deleted_at  IS NULL
		INNER JOIN tbl_form_field              ff  ON ff.id  = ev.form_field_id     AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_chart_of_accounts       coa ON coa.id = ff.coa_id            AND coa.deleted_at IS NULL AND coa.is_system = FALSE
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version     fv  ON fv.id  = e.form_version_id    AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id  = fv.form_id           AND fm.deleted_at IS NULL
		LEFT  JOIN tbl_clinic                  c   ON c.id   = e.clinic_id          AND c.deleted_at  IS NULL
		WHERE ev.updated_at IS NULL AND coa.id = ?` + permissionClause

	searchCols := []string{"ff.label", "coa.name", "fm.name", "COALESCE(c.name, 'Expense')"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, false)
	args := []any{coaID, actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	type detailRow struct {
		ID            uuid.UUID `db:"id"`
		EntryID       uuid.UUID `db:"entry_id"`
		FormFieldID   uuid.UUID `db:"form_field_id"`
		CoaID         uuid.UUID `db:"coa_id"`
		TaxTypeID     *int16    `db:"tax_type_id"`
		FormID        uuid.UUID `db:"form_id"`
		ClinicID      uuid.UUID `db:"clinic_id"`
		VersionID     uuid.UUID `db:"version_id"`
		FormFieldName string    `db:"form_field_name"`
		CoaName       string    `db:"coa_name"`
		TaxTypeName   *string   `db:"tax_type_name"`
		FormName      string    `db:"form_name"`
		FormMethod    string    `db:"form_method"`
		ClinicName    string    `db:"clinic_name"`
		NetAmount     *float64  `db:"net_amount"`
		GstAmount     *float64  `db:"gst_amount"`
		GrossAmount   *float64  `db:"gross_amount"`
		CreatedAt     string    `db:"created_at"`
		UpdatedAt     *string   `db:"updated_at"`
	}

	var rows []*detailRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("list coa entry details: %w", err)
	}

	result := make([]*RsCoaEntryDetail, 0, len(rows))
	for _, row := range rows {
		isExpense := row.FormMethod == "EXPENSE_ENTRY"
		
		detail := &RsCoaEntryDetail{
			ID:            row.ID.String(),
			EntryID:       row.EntryID.String(),
			FormFieldID:   row.FormFieldID.String(),
			CoaID:         row.CoaID.String(),
			TaxTypeID:     row.TaxTypeID,
			FormID:        row.FormID.String(),
			VersionID:     row.VersionID.String(),
			FormFieldName: row.FormFieldName,
			CoaName:       row.CoaName,
			TaxTypeName:   row.TaxTypeName,
			IsExpense:     isExpense,
			NetAmount:     row.NetAmount,
			GstAmount:     row.GstAmount,
			GrossAmount:   row.GrossAmount,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		}
		
		// Only include clinic_id, clinic_name, and form_name for non-expense entries
		if !isExpense {
			clinicID := row.ClinicID.String()
			detail.ClinicID = &clinicID
			detail.ClinicName = &row.ClinicName
			detail.FormName = &row.FormName
		} else {
			// For expense entries, use form_field_name as supplier_name
			detail.SupplierName = &row.FormFieldName
		}
		
		result = append(result, detail)
	}
	return result, nil
}

// CountCoaEntryDetails returns the total number of entry details for a specific COA
func (r *Repository) CountCoaEntryDetails(ctx context.Context, coaID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		// Accountant: show clinic entries they have access to + expense entries from those practitioners
		permissionClause = ` AND (
			c.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation
				WHERE accountant_id = ? AND status = 'COMPLETED'
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id IN (
				SELECT practitioner_id FROM tbl_invitation
				WHERE accountant_id = ? AND status = 'COMPLETED'
			))
		)`
	} else {
		// Practitioner: show own clinic entries + own expense entries
		permissionClause = ` AND (
			c.id IN (
				SELECT id FROM tbl_clinic
				WHERE practitioner_id = ? AND deleted_at IS NULL
			)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "e.clinic_id",
		"form_id":         "fm.id",
		"tax_type_id":     "at2.id",
		"practitioner_id": "COALESCE(c.practitioner_id, fv.practitioner_id)",
		"start_date":      "ev.created_at",
		"end_date":        "ev.created_at",
		"created_at":      "ev.created_at",
	}

	base := `
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry              e   ON e.id   = ev.entry_id          AND e.deleted_at  IS NULL
		INNER JOIN tbl_form_field              ff  ON ff.id  = ev.form_field_id     AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_chart_of_accounts       coa ON coa.id = ff.coa_id            AND coa.deleted_at IS NULL AND coa.is_system = FALSE
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version     fv  ON fv.id  = e.form_version_id    AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id  = fv.form_id           AND fm.deleted_at IS NULL
		LEFT  JOIN tbl_clinic                  c   ON c.id   = e.clinic_id          AND c.deleted_at  IS NULL
		WHERE ev.updated_at IS NULL AND coa.id = ?` + permissionClause

	searchCols := []string{"ff.label", "coa.name", "fm.name", "COALESCE(c.name, 'Expense')"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, true)
	args := []any{coaID, actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count coa entry details: %w", err)
	}
	return total, nil
}
