package entry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/engine/pl"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form entry not found")

type IRepository interface {
	Create(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error
	Update(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error
	Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
	DeleteSingleEntryValue(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error
	GetByID(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)
	GetByValueID(ctx context.Context, tx *sqlx.Tx, entryId uuid.UUID, valId uuid.UUID) (*FormEntry, []*FormEntryValue, error)
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*FormEntry, error)
	CountByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) (int, error)
	HasSubmittedEntryValuesForField(ctx context.Context, formFieldID uuid.UUID) (bool, error)
	GetByVersionID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)
	ListTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsTransactionRow, error)
	CountTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error)
	// COA-grouped endpoints
	ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error)
	CountCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error)
	ListCoaEntryDetails(ctx context.Context, coaName string, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntryDetail, error)
	CountCoaEntryDetails(ctx context.Context, coaName string, f common.Filter, actorID uuid.UUID, role string) (int, error)
	GetSummedValuesByFieldID(ctx context.Context, fieldID uuid.UUID) (*RsFieldSummary, error)
	GetCoaNameByID(ctx context.Context, id uuid.UUID) (string, error)
	// Document linking
	LinkDocuments(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, documentIDs []uuid.UUID) error
	UnlinkDocuments(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, documentIDs []uuid.UUID) error
	GetDocumentsByEntryID(ctx context.Context, entryID uuid.UUID) ([]*RsEntryDocument, error)
	// Expense-specific helpers (keep raw SQL in repo, not service)
	InsertBalancingEntryValue(ctx context.Context, tx *sqlx.Tx, ev *FormEntryValue) error
	InsertEntryValue(ctx context.Context, tx *sqlx.Tx, ev *FormEntryValue) error
	MarkEntryValueUpdated(ctx context.Context, tx *sqlx.Tx, fieldID uuid.UUID, entryID uuid.UUID) error
	DeleteSystemBalancingValues(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID) error
	GetActiveEntryValuesWithAccountType(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID) ([]*EntryValueWithAccountType, error)
	UpdateEntryDate(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, date string) error
}

type Repository struct {
	db     *sqlx.DB
	plRepo pl.Repository
}

func NewRepository(db *sqlx.DB, plRepo pl.Repository) IRepository {
	return &Repository{db: db, plRepo: plRepo}
}

func (r *Repository) GetByID(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*FormEntry, []*FormEntryValue, error) {
	query := `SELECT 
            e.id, e.form_version_id, e.clinic_id, e.submitted_by, e.submitted_at, 
            e.status, e.date, e.created_at, e.updated_at,
            v.practitioner_id 
        FROM tbl_form_entry e 
        INNER JOIN tbl_custom_form_version v ON e.form_version_id = v.id 
        WHERE e.id = $1 AND e.deleted_at IS NULL`
	var e FormEntry
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.Date, &e.CreatedAt, &e.UpdatedAt, &e.PractitionerID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	valQuery := `SELECT id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, date, business_percentage, created_at, updated_at
		FROM tbl_form_entry_value
		WHERE entry_id = $1 AND updated_at IS NULL AND form_field_id IS NOT NULL
		`
	var values []*FormEntryValue
	if err := tx.SelectContext(ctx, &values, valQuery, id); err != nil {
		return nil, nil, fmt.Errorf("get entry values: %w", err)
	}
	return &e, values, nil
}

func (r *Repository) GetByValueID(ctx context.Context, tx *sqlx.Tx, entryId uuid.UUID, valId uuid.UUID) (*FormEntry, []*FormEntryValue, error) {

	// Use the entryID to load the parent entry
	query := `SELECT 
            e.id, e.form_version_id, e.clinic_id, e.submitted_by, e.submitted_at, 
            e.status, e.date, e.created_at, e.updated_at,
            v.practitioner_id 
        FROM tbl_form_entry e 
        INNER JOIN tbl_custom_form_version v ON e.form_version_id = v.id 
        WHERE e.id = $1 AND e.deleted_at IS NULL`

	var e FormEntry
	if err := tx.QueryRowContext(ctx, query, entryId).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.Date, &e.CreatedAt, &e.UpdatedAt, &e.PractitionerID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	// Load all active values under this parent entry container
	valQuery := `SELECT id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, date, business_percentage, created_at, updated_at
        FROM tbl_form_entry_value
        WHERE entry_id = $1 AND deleted_at IS NULL AND form_field_id IS NOT NULL`

	var values []*FormEntryValue
	if err := tx.SelectContext(ctx, &values, valQuery, entryId); err != nil {
		return nil, nil, fmt.Errorf("get entry values: %w", err)
	}

	return &e, values, nil
}

func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter, actorID uuid.UUID, role string) ([]*FormEntry, error) {
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

	valQuery := `SELECT id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, date, business_percentage , description, created_at, updated_at
		FROM tbl_form_entry_value
		WHERE entry_id = $1 AND updated_at IS NULL AND form_field_id IS NOT NULL
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
	// For expense entries use item-level date; for all others use entry-level date
	"start_date": "COALESCE(CASE WHEN fm.method = 'EXPENSE_ENTRY' THEN ev.date ELSE e.date END, ev.created_at)",
	"end_date":   "COALESCE(CASE WHEN fm.method = 'EXPENSE_ENTRY' THEN ev.date ELSE e.date END, ev.created_at)",
	"date":       "CASE WHEN fm.method = 'EXPENSE_ENTRY' THEN ev.date ELSE e.date END",
}

func (r *Repository) ListTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsTransactionRow, error) {
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
		WITH ranked_ev AS (
			SELECT 
				ev.id,
				ROW_NUMBER() OVER (
					PARTITION BY ev.entry_id, ev.form_field_id 
					ORDER BY (ev.updated_at IS NULL) DESC, ev.created_at DESC
				) as rn
			FROM tbl_form_entry_value ev
			WHERE ev.deleted_at IS NULL
		)
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
			COALESCE(ev.business_percentage, 100.00) AS business_percentage,
			COALESCE(ev.notes, '-') AS notes,
			ev.created_at,
			ev.updated_at,
			CASE WHEN fm.method = 'EXPENSE_ENTRY' THEN ev.date ELSE e.date END AS date,
			(e.clinic_id = '00000000-0000-0000-0000-000000000000') AS is_expense
		FROM ranked_ev
		INNER JOIN tbl_form_entry_value ev ON ev.id = ranked_ev.id AND ranked_ev.rn = 1
		INNER JOIN tbl_form_entry e ON e.id = ev.entry_id AND e.deleted_at IS NULL
		INNER JOIN tbl_form_field ff ON ff.id = ev.form_field_id AND ff.deleted_at IS NULL AND ff.is_formula = FALSE AND ff.label IS NOT NULL AND ff.label != ''
		INNER JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL
		LEFT JOIN tbl_account_tax at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		LEFT JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL` + permissionClause

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
			ID:                 row.ID,
			EntryID:            row.EntryID,
			FormFieldID:        row.FormFieldID,
			FormFieldName:      row.FormFieldName,
			CoaID:              row.CoaID,
			CoaName:            row.CoaName,
			TaxTypeID:          row.TaxTypeID,
			TaxTypeName:        row.TaxTypeName,
			FormID:             row.FormID,
			FormName:           row.FormName,
			ClinicID:           row.ClinicID,
			ClinicName:         row.ClinicName,
			NetAmount:          row.NetAmount,
			GstAmount:          row.GstAmount,
			GrossAmount:        row.GrossAmount,
			BusinessPercentage: row.BusinessPercentage,
			Notes:              row.Description,
			CreatedAt:          row.CreatedAt,
			UpdatedAt:          row.UpdatedAt,
			Date:               row.Date,
			IsExpense:          row.IsExpense,
		})
	}
	return result, nil
}

func (r *Repository) CountTransactions(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error) {
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
		WITH ranked_ev AS (
			SELECT 
				ev.id,
				ROW_NUMBER() OVER (
					PARTITION BY ev.entry_id, ev.form_field_id 
					ORDER BY (ev.updated_at IS NULL) DESC, ev.created_at DESC
				) as rn
			FROM tbl_form_entry_value ev
			WHERE ev.deleted_at IS NULL
		)
		FROM ranked_ev
		INNER JOIN tbl_form_entry_value ev ON ev.id = ranked_ev.id AND ranked_ev.rn = 1
		INNER JOIN tbl_form_entry e ON e.id = ev.entry_id AND e.deleted_at IS NULL
		INNER JOIN tbl_form_field ff ON ff.id = ev.form_field_id AND ff.deleted_at IS NULL AND ff.is_formula = FALSE AND ff.label IS NOT NULL AND ff.label != ''
		INNER JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id AND coa.deleted_at IS NULL
		LEFT JOIN tbl_account_tax at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		LEFT JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL` + permissionClause

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

func (r *Repository) Create(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error {
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
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, date, business_percentage)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at, updated_at
		`
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description, v.Date, v.BusinessPercentage).
			Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
			return fmt.Errorf("create entry value tx: %w", err)
		}
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error {
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

	// Insert new values as the current active records
	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
            INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, description, date, business_percentage, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULL)
            RETURNING created_at
        `
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.CoaID, v.NetAmount, v.GstAmount, v.GrossAmount, v.Description, v.Date, v.BusinessPercentage).
			Scan(&v.CreatedAt); err != nil {
			return fmt.Errorf("insert entry value tx: %w", err)
		}
		v.UpdatedAt = nil
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	// Mark all associated form entry values as deleted
	valDeleteQuery := `UPDATE tbl_form_entry_value SET deleted_at = now() WHERE entry_id = $1 AND deleted_at IS NULL`
	if _, err := tx.ExecContext(ctx, valDeleteQuery, id); err != nil {
		return fmt.Errorf("delete form entry values tx: %w", err)
	}

	// Mark the parent entry as deleted
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

func (r *Repository) DeleteSingleEntryValue(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	query := `UPDATE tbl_form_entry_value SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form entry value tx: %w", err)
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

func (r *Repository) ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND (
            v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation
                WHERE accountant_id = ? AND status = 'COMPLETED'
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation
                WHERE accountant_id = ? AND status = 'COMPLETED'
            ))
        )`
	} else {
		permissionClause = ` AND (
            v.clinic_id IN (
                SELECT id FROM tbl_clinic
                WHERE practitioner_id = ? AND deleted_at IS NULL
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id = ?)
        )`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "v.clinic_id",
		"form_id":         "v.form_id",
		"coa_id":          "v.coa_id",
		"tax_type_id":     "v.tax_id",
		"practitioner_id": "v.practitioner_id",
		"start_date":      "v.entry_date",
		"end_date":        "v.entry_date",
	}

	base := `
    SELECT
        MAX(v.coa_id::text)::uuid                                                      AS coa_id,
        v.account_name                                                                 AS coa_name,
        COALESCE(MAX(coa.is_system::int)::bool, false)                                 AS is_system,
        COALESCE(ABS(ROUND(SUM(fev.net_amount)::numeric, 2))::float8, 0.0)             AS total_net_amount,
        COALESCE(ABS(ROUND(SUM(fev.gst_amount)::numeric, 2))::float8, 0.0)             AS total_gst_amount,
        COALESCE(ABS(ROUND(SUM(fev.gross_amount)::numeric, 2))::float8, 0.0)           AS total_gross_amount,
        COUNT(DISTINCT fev.id)                                                         AS entry_count
    FROM vw_double_entry_line_items v
    LEFT JOIN tbl_chart_of_accounts coa ON coa.id = v.coa_id
    INNER JOIN tbl_form_entry_value fev
		ON fev.entry_id = v.entry_id
			AND fev.deleted_at IS NULL
			AND fev.updated_at IS NULL
			AND (
				(v.form_field_id IS NOT NULL AND fev.form_field_id = v.form_field_id)
				OR
				(v.form_field_id IS NULL AND fev.form_field_id IS NULL AND fev.coa_id = v.coa_id)
			)
    WHERE v.form_field_id IS NOT NULL` + permissionClause

	searchCols := []string{"v.account_name", "v.account_code"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, false)

	groupByClause := ` GROUP BY v.account_name`
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
		IsSystem         bool      `db:"is_system"`
		TotalNetAmount   float64   `db:"total_net_amount"`
		TotalGSTAmount   float64   `db:"total_gst_amount"`
		TotalGrossAmount float64   `db:"total_gross_amount"`
		EntryCount       int       `db:"entry_count"`
	}

	var rows []*coaEntryRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("list coa entries via views: %w", err)
	}

	result := make([]*RsCoaEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &RsCoaEntry{
			CoaID:            row.CoaID.String(),
			CoaName:          row.CoaName,
			IsSystem:         row.IsSystem,
			TotalNetAmount:   row.TotalNetAmount,
			TotalGSTAmount:   row.TotalGSTAmount,
			TotalGrossAmount: row.TotalGrossAmount,
			EntryCount:       row.EntryCount,
		})
	}

	var targetPracIDs []uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		err := r.db.SelectContext(ctx, &targetPracIDs, `SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED'`, actorID)
		if err != nil {
			return result, nil
		}
	} else {
		targetPracIDs = []uuid.UUID{actorID}
	}

	if len(targetPracIDs) > 0 {
		plFilter := &pl.PLReportFilter{}

		// Extract filters from Where conditions
		for _, cond := range f.Where {
			switch cond.Field {
			case "start_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateFrom = &dateStr
				}
			case "end_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateUntil = &dateStr
				}
			case "clinic_id":
				if clinicUUID, ok := cond.Value.(uuid.UUID); ok {
					clinicStr := clinicUUID.String()
					plFilter.ClinicID = &clinicStr
				}
			case "form_id":
				if formUUID, ok := cond.Value.(uuid.UUID); ok {
					formStr := formUUID.String()
					plFilter.FormID = &formStr
				}
			case "tax_type_id":
				if TaxTypeUUID, ok := cond.Value.(uuid.UUID); ok {
					TaxTypeStr := TaxTypeUUID.String()
					plFilter.TaxTypeID = &TaxTypeStr
				}
			}
		}

		summary, err := r.plRepo.GetPLSummary(ctx, targetPracIDs, plFilter)
		if err == nil && summary != nil && summary.NetProfitNet != 0 {
			var reCOA struct {
				ID   uuid.UUID `db:"id"`
				Name string    `db:"name"`
			}
			err = r.db.GetContext(ctx, &reCOA, `
				SELECT v.id, COALESCE(v.name, tpl.name) AS name 
				FROM tbl_chart_of_accounts v
				LEFT JOIN tbl_chart_of_accounts_template tpl ON tpl.id = v.template_id
				WHERE (COALESCE(v.code, tpl.code) = 960 OR LOWER(COALESCE(v.name, tpl.name)) = 'retained earnings')
				  AND v.deleted_at IS NULL LIMIT 1`)

			if err != nil {
				reCOA.ID = uuid.MustParse("00000000-0000-0000-0000-000000000960")
				reCOA.Name = "Retained Earnings"
			}

			found := false
			for _, entry := range result {
				if strings.EqualFold(entry.CoaName, reCOA.Name) || entry.CoaID == reCOA.ID.String() {
					entry.TotalNetAmount = math.Abs(summary.NetProfitNet)
					entry.TotalGrossAmount = math.Abs(summary.NetProfitNet)
					entry.EntryCount = 1
					found = true
					break
				}
			}

			if !found {
				result = append(result, &RsCoaEntry{
					CoaID:            reCOA.ID.String(),
					CoaName:          reCOA.Name,
					IsSystem:         true,
					TotalNetAmount:   math.Abs(summary.NetProfitNet),
					TotalGSTAmount:   0.0,
					TotalGrossAmount: math.Abs(summary.NetProfitNet),
					EntryCount:       1,
				})
			}
		}
	}

	return result, nil
}

func (r *Repository) CountCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND (
            v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation
                WHERE accountant_id = ? AND status = 'COMPLETED'
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation
                WHERE accountant_id = ? AND status = 'COMPLETED'
            ))
        )`
	} else {
		permissionClause = ` AND (
            v.clinic_id IN (
                SELECT id FROM tbl_clinic
                WHERE practitioner_id = ? AND deleted_at IS NULL
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id = ?)
        )`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "v.clinic_id",
		"form_id":         "v.form_id",
		"coa_id":          "v.coa_id",
		"tax_type_id":     "v.tax_id",
		"practitioner_id": "v.practitioner_id",
		"start_date":      "v.entry_date",
		"end_date":        "v.entry_date",
	}

	base := ` FROM vw_double_entry_line_items v
    INNER JOIN tbl_form_entry_value fev
		ON fev.entry_id = v.entry_id
			AND fev.deleted_at IS NULL
			AND fev.updated_at IS NULL
			AND (
			(v.form_field_id IS NOT NULL AND fev.form_field_id = v.form_field_id)
			OR
			(v.form_field_id IS NULL AND fev.form_field_id IS NULL AND fev.coa_id = v.coa_id)
		)
    WHERE v.form_field_id IS NOT NULL` + permissionClause

	searchCols := []string{"v.account_name", "v.account_code"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, true)

	if strings.Contains(strings.ToUpper(q), "COUNT(*)") {
		q = strings.ReplaceAll(q, "COUNT(*)", "COUNT(DISTINCT v.account_name)")
	}

	args := []any{actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count coa entries via views: %w", err)
	}

	var targetPracIDs []uuid.UUID
	if strings.EqualFold(role, util.RoleAccountant) {
		_ = r.db.SelectContext(ctx, &targetPracIDs, `SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED'`, actorID)
	} else {
		targetPracIDs = []uuid.UUID{actorID}
	}

	if len(targetPracIDs) > 0 {
		plFilter := &pl.PLReportFilter{}

		// Extract filters from Where conditions
		for _, cond := range f.Where {
			switch cond.Field {
			case "start_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateFrom = &dateStr
				}
			case "end_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateUntil = &dateStr
				}
			case "clinic_id":
				if clinicUUID, ok := cond.Value.(uuid.UUID); ok {
					clinicStr := clinicUUID.String()
					plFilter.ClinicID = &clinicStr
				}
			case "form_id":
				if formUUID, ok := cond.Value.(uuid.UUID); ok {
					formStr := formUUID.String()
					plFilter.FormID = &formStr
				}
			case "tax_type_id":
				if TaxTypeUUID, ok := cond.Value.(uuid.UUID); ok {
					TaxTypeStr := TaxTypeUUID.String()
					plFilter.TaxTypeID = &TaxTypeStr
				}
			}
		}

		summary, err := r.plRepo.GetPLSummary(ctx, targetPracIDs, plFilter)
		if err == nil && summary != nil && summary.NetProfitNet != 0 {
			total++
		}
	}

	return total, nil
}

func (r *Repository) ListCoaEntryDetails(ctx context.Context, coaName string, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntryDetail, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND (
            v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation 
                WHERE accountant_id = ? AND status = 'COMPLETED'
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation 
                WHERE accountant_id = ? AND status = 'COMPLETED'
            ))
        )`
	} else {
		permissionClause = ` AND (
            v.clinic_id IN (
                SELECT id FROM tbl_clinic 
                WHERE practitioner_id = ? AND deleted_at IS NULL
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id = ?)
        )`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "v.clinic_id",
		"form_id":         "v.form_id",
		"tax_type_id":     "v.tax_id",
		"practitioner_id": "v.practitioner_id",
		"start_date":      "v.entry_date",
		"end_date":        "v.entry_date",
		"created_at":      "v.entry_date",
	}

	base := `
         SELECT
            MD5(COALESCE(v.entry_id::text, '') || COALESCE(v.coa_id::text, '') || COALESCE(fev.id::text, ''))::uuid AS id,
            fev.id                                                                                                         AS form_entry_value_id,
            v.entry_id                                                                                                     AS entry_id,
            v.form_field_id                                                                                                AS form_field_id,
            v.coa_id                                                                                                       AS coa_id,
            v.tax_id                                                                                                       AS tax_type_id,
            v.form_id                                                                                                      AS form_id,
            v.clinic_id                                                                                                    AS clinic_id,
            NULL::uuid                                                                                                     AS line_item_value_id,
            v.form_id                                                                                                      AS version_id,
            COALESCE(ff.label, 'System Accounts')                                                                          AS form_field_name,
            v.account_name                                                                                                 AS coa_name,
            COALESCE(t.name, '')                                                                                           AS tax_type_name,
            COALESCE(f.name, '')                                                                                           AS form_name,
            COALESCE(f.method::text, '')                                                                                   AS form_method,
            COALESCE(c.name, '')                                                                                           AS clinic_name,
            CASE
                WHEN COALESCE(v.debit_amount, 0) > 0 THEN 'DEBIT'
                WHEN COALESCE(v.credit_amount, 0) > 0 THEN 'CREDIT'
                WHEN v.normal_balance = 'DEBIT'  AND COALESCE(v.net_amount, 0) >= 0 THEN 'DEBIT'
                WHEN v.normal_balance = 'DEBIT'  AND COALESCE(v.net_amount, 0) <  0 THEN 'CREDIT'
                WHEN v.normal_balance = 'CREDIT' AND COALESCE(v.net_amount, 0) >= 0 THEN 'CREDIT'
                WHEN v.normal_balance = 'CREDIT' AND COALESCE(v.net_amount, 0) <  0 THEN 'DEBIT'
                ELSE 'UNKNOWN'
            END                                                                                                            AS transaction_type,
            COALESCE(v.account_type, '')                                                                                   AS account_type,
            ROUND(COALESCE(fev.net_amount, 0)::numeric, 2)::float8                                                         AS net_amount,
            ROUND(COALESCE(fev.gst_amount, 0)::numeric, 2)::float8                                                         AS gst_amount,
            ROUND(COALESCE(fev.gross_amount, 0)::numeric, 2)::float8                                                       AS gross_amount,
            COALESCE(v.business_percentage::float8, 100.00::float8)                                                         AS business_percentage,
            COALESCE(v.description, '-')                                                                                   AS description,
            TO_CHAR(v.entry_date, 'YYYY-MM-DD HH24:MI:SS')                                                                 AS created_at
        FROM vw_double_entry_line_items v
        LEFT JOIN tbl_form f        ON f.id = v.form_id
        LEFT JOIN tbl_form_field ff ON ff.id = v.form_field_id
        LEFT JOIN tbl_account_tax t ON t.id = v.tax_id
        LEFT JOIN tbl_clinic c      ON c.id = v.clinic_id AND c.deleted_at IS NULL
       INNER JOIN tbl_form_entry_value fev
        ON fev.entry_id = v.entry_id
            AND fev.deleted_at IS NULL
            AND fev.updated_at IS NULL
            AND (
            (v.form_field_id IS NOT NULL AND fev.form_field_id = v.form_field_id)
            OR
            (v.form_field_id IS NULL AND fev.form_field_id IS NULL AND fev.coa_id = v.coa_id)
        )
        WHERE v.account_name = ? AND v.form_field_id IS NOT NULL` + permissionClause

	if f.SortBy == nil || *f.SortBy == "" {
		defaultSort := "v.entry_date"
		f.SortBy = &defaultSort
		defaultOrder := "DESC, fev.id ASC"
		f.OrderBy = &defaultOrder
	} else {
		extendedOrder := *f.OrderBy + ", fev.id ASC"
		f.OrderBy = &extendedOrder
	}

	searchCols := []string{"ff.label", "v.account_name", "f.name", "c.name"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, false)
	args := []any{coaName, actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	type detailRow struct {
		ID                 uuid.UUID  `db:"id"`
		FormEntryValueID   *string    `db:"form_entry_value_id"`
		EntryID            uuid.UUID  `db:"entry_id"`
		FormFieldID        *string    `db:"form_field_id"`
		CoaID              uuid.UUID  `db:"coa_id"`
		TaxTypeID          *int16     `db:"tax_type_id"`
		FormID             *string    `db:"form_id"`
		ClinicID           uuid.UUID  `db:"clinic_id"`
		LineItemValueID    *uuid.UUID `db:"line_item_value_id"`
		VersionID          *string    `db:"version_id"`
		FormFieldName      string     `db:"form_field_name"`
		CoaName            string     `db:"coa_name"`
		TaxTypeName        *string    `db:"tax_type_name"`
		FormName           *string    `db:"form_name"`
		FormMethod         *string    `db:"form_method"`
		ClinicName         string     `db:"clinic_name"`
		TransactionType    string     `db:"transaction_type"`
		AccountType        string     `db:"account_type"`
		NetAmount          float64    `db:"net_amount"`
		GstAmount          float64    `db:"gst_amount"`
		GrossAmount        float64    `db:"gross_amount"`
		BusinessPercentage float64    `db:"business_percentage"`
		Notes              string     `db:"description"`
		CreatedAt          string     `db:"created_at"`
		UpdatedAt          *string    `db:"updated_at"`
	}

	var rows []*detailRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("list coa entry details via views: %w", err)
	}

	result := make([]*RsCoaEntryDetail, 0, len(rows))
	for _, row := range rows {
		var isExpense bool
		if row.FormMethod != nil && *row.FormMethod != "" {
			isExpense = (*row.FormMethod == "EXPENSE_ENTRY")
		}
		netVal := row.NetAmount
		gstVal := row.GstAmount
		grossVal := row.GrossAmount
		bizPct := row.BusinessPercentage

		if row.TransactionType == "CREDIT" {
			if netVal < 0 {
				netVal = netVal * -1
			}
			if grossVal < 0 {
				grossVal = grossVal * -1
			}
		}

		detail := &RsCoaEntryDetail{
			ID:                 row.ID.String(),
			FormEntryValueID:   row.FormEntryValueID,
			EntryID:            row.EntryID.String(),
			CoaID:              row.CoaID.String(),
			TaxTypeID:          row.TaxTypeID,
			FormFieldName:      row.FormFieldName,
			CoaName:            row.CoaName,
			TaxTypeName:        row.TaxTypeName,
			IsExpense:          isExpense,
			TransactionType:    row.TransactionType,
			AccountType:        row.AccountType,
			NetAmount:          &netVal,
			GstAmount:          &gstVal,
			GrossAmount:        &grossVal,
			BusinessPercentage: &bizPct,
			Notes:              &row.Notes,
			CreatedAt:          row.CreatedAt,
			UpdatedAt:          row.UpdatedAt,
		}

		if row.LineItemValueID != nil {
			detail.ID = row.LineItemValueID.String()
		}
		if row.FormFieldID != nil {
			detail.FormFieldID = *row.FormFieldID
		}
		if row.FormID != nil {
			detail.FormID = *row.FormID
		}
		if row.VersionID != nil {
			detail.VersionID = *row.VersionID
		}

		if !isExpense {
			clinicID := row.ClinicID.String()
			detail.ClinicID = &clinicID
			detail.ClinicName = &row.ClinicName
			if row.FormName != nil {
				detail.FormName = row.FormName
			}
		} else {
			detail.SupplierName = &row.FormFieldName
		}

		result = append(result, detail)
	}

	if strings.EqualFold(coaName, "Retained Earnings") {
		var targetPracIDs []uuid.UUID
		if strings.EqualFold(role, util.RoleAccountant) {
			_ = r.db.SelectContext(ctx, &targetPracIDs, `SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED'`, actorID)
		} else {
			targetPracIDs = []uuid.UUID{actorID}
		}

		plFilter := &pl.PLReportFilter{}

		// Extract filters from Where conditions
		for _, cond := range f.Where {
			switch cond.Field {
			case "start_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateFrom = &dateStr
				}
			case "end_date":
				if dateStr, ok := cond.Value.(string); ok {
					plFilter.DateUntil = &dateStr
				}
			case "clinic_id":
				if clinicUUID, ok := cond.Value.(uuid.UUID); ok {
					clinicStr := clinicUUID.String()
					plFilter.ClinicID = &clinicStr
				}
			case "form_id":
				if formUUID, ok := cond.Value.(uuid.UUID); ok {
					formStr := formUUID.String()
					plFilter.FormID = &formStr
				}
			case "tax_type_id":
				if TaxTypeUUID, ok := cond.Value.(uuid.UUID); ok {
					TaxTypeStr := TaxTypeUUID.String()
					plFilter.TaxTypeID = &TaxTypeStr
				}
			}
		}

		summary, err := r.plRepo.GetPLSummary(ctx, targetPracIDs, plFilter)
		if err == nil && summary != nil && summary.NetProfitNet != 0 {
			netVal := summary.NetProfitNet
			grossVal := summary.NetProfitNet
			txType := "CREDIT"
			if netVal < 0 {
				txType = "DEBIT"
				netVal = netVal * -1
				grossVal = grossVal * -1
			}

			zeroGst := 0.0
			noteStr := "Synced from Profit & Loss"

			result = append(result, &RsCoaEntryDetail{
				ID:              uuid.New().String(),
				CoaName:         coaName,
				FormFieldName:   "Net Profit",
				TransactionType: txType,
				NetAmount:       &netVal,
				GstAmount:       &zeroGst,
				GrossAmount:     &grossVal,
				Notes:           &noteStr,
				CreatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			})
		}
	}

	return result, nil
}

func (r *Repository) CountCoaEntryDetails(ctx context.Context, coaName string, f common.Filter, actorID uuid.UUID, role string) (int, error) {
	var permissionClause string

	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND (
            v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation 
                WHERE accountant_id = ? AND status = 'COMPLETED'
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id IN (
                SELECT practitioner_id FROM tbl_invitation 
                WHERE accountant_id = ? AND status = 'COMPLETED'
            ))
        )`
	} else {
		permissionClause = ` AND (
            v.clinic_id IN (
                SELECT id FROM tbl_clinic 
                WHERE practitioner_id = ? AND deleted_at IS NULL
            )
            OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id = ?)
        )`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "v.clinic_id",
		"form_id":         "v.form_id",
		"tax_type_id":     "v.tax_id",
		"practitioner_id": "v.practitioner_id",
		"start_date":      "v.entry_date",
		"end_date":        "v.entry_date",
		"created_at":      "v.entry_date",
	}

	base := `
        FROM vw_double_entry_line_items v
        INNER JOIN tbl_form_entry_value fev
        ON fev.entry_id = v.entry_id
            AND fev.deleted_at IS NULL
            AND fev.updated_at IS NULL
            AND (
            (v.form_field_id IS NOT NULL AND fev.form_field_id = v.form_field_id)
            OR
            (v.form_field_id IS NULL AND fev.form_field_id IS NULL AND fev.coa_id = v.coa_id)
        )
        WHERE v.account_name = ? AND v.form_field_id IS NOT NULL` + permissionClause

	searchCols := []string{"v.account_name", "v.description"}
	q, qArgs := common.BuildQuery(base, f, allowedColumns, searchCols, true)
	args := []any{coaName, actorID, actorID}
	args = append(args, qArgs...)
	q = r.db.Rebind(q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count coa entry details via views: %w", err)
	}

	if strings.EqualFold(coaName, "Retained Earnings") {
		var targetPracIDs []uuid.UUID
		if strings.EqualFold(role, util.RoleAccountant) {
			_ = r.db.SelectContext(ctx, &targetPracIDs, `SELECT practitioner_id FROM tbl_invitation WHERE accountant_id = ? AND status = 'COMPLETED'`, actorID)
		} else {
			targetPracIDs = []uuid.UUID{actorID}
		}

		if len(targetPracIDs) > 0 {
			plFilter := &pl.PLReportFilter{}

			// Extract filters from Where conditions
			for _, cond := range f.Where {
				switch cond.Field {
				case "start_date":
					if dateStr, ok := cond.Value.(string); ok {
						plFilter.DateFrom = &dateStr
					}
				case "end_date":
					if dateStr, ok := cond.Value.(string); ok {
						plFilter.DateUntil = &dateStr
					}
				case "clinic_id":
					if clinicUUID, ok := cond.Value.(uuid.UUID); ok {
						clinicStr := clinicUUID.String()
						plFilter.ClinicID = &clinicStr
					}
				case "form_id":
					if formUUID, ok := cond.Value.(uuid.UUID); ok {
						formStr := formUUID.String()
						plFilter.FormID = &formStr
					}
				case "tax_type_id":
					if TaxTypeUUID, ok := cond.Value.(uuid.UUID); ok {
						TaxTypeStr := TaxTypeUUID.String()
						plFilter.TaxTypeID = &TaxTypeStr
					}
				}
			}
			summary, err := r.plRepo.GetPLSummary(ctx, targetPracIDs, plFilter)
			if err == nil && summary != nil && summary.NetProfitNet != 0 {
				total++
			}
		}
	}

	return total, nil
}

func (r *Repository) GetCoaNameByID(ctx context.Context, id uuid.UUID) (string, error) {
	var name string
	query := `
		SELECT COALESCE(coa.name, tpl.name, '') AS name
		FROM tbl_chart_of_accounts coa
		LEFT JOIN tbl_chart_of_accounts_template tpl ON tpl.id = coa.template_id
		WHERE coa.id = $1
	`
	err := r.db.GetContext(ctx, &name, query, id)
	if err != nil {
		return "", fmt.Errorf("find coa name: %w", err)
	}
	return name, nil
}

func (r *Repository) LinkDocuments(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, documentIDs []uuid.UUID) error {
	if len(documentIDs) == 0 {
		return nil
	}
	for _, docID := range documentIDs {
		query := `
			INSERT INTO tbl_map_entry_document (entry_id, document_id)
			VALUES ($1, $2)
			ON CONFLICT (entry_id, document_id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, query, entryID, docID); err != nil {
			return fmt.Errorf("link document %s to entry %s: %w", docID, entryID, err)
		}
	}
	return nil
}

func (r *Repository) UnlinkDocuments(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, documentIDs []uuid.UUID) error {
	if len(documentIDs) == 0 {
		return nil
	}
	for _, docID := range documentIDs {
		query := `DELETE FROM tbl_map_entry_document WHERE entry_id = $1 AND document_id = $2`
		if _, err := tx.ExecContext(ctx, query, entryID, docID); err != nil {
			return fmt.Errorf("unlink document %s from entry %s: %w", docID, entryID, err)
		}
	}
	return nil
}

func (r *Repository) GetDocumentsByEntryID(ctx context.Context, entryID uuid.UUID) ([]*RsEntryDocument, error) {
	query := `
		SELECT
			d.id,
			d.original_name,
			d.object_key  AS file_key,
			d.uploaded_at,
			fed.created_at
		FROM tbl_map_entry_document fed
		INNER JOIN tbl_document d ON d.id = fed.document_id AND d.deleted_at IS NULL
		WHERE fed.entry_id = $1
		ORDER BY fed.created_at ASC`

	type row struct {
		ID           uuid.UUID `db:"id"`
		OriginalName string    `db:"original_name"`
		FileKey      string    `db:"file_key"`
		UploadedAt   *string   `db:"uploaded_at"`
		CreatedAt    string    `db:"created_at"`
	}

	var rows []*row
	if err := r.db.SelectContext(ctx, &rows, query, entryID); err != nil {
		return nil, fmt.Errorf("get documents by entry id: %w", err)
	}

	result := make([]*RsEntryDocument, 0, len(rows))
	for _, r := range rows {
		result = append(result, &RsEntryDocument{
			ID:           r.ID,
			OriginalName: r.OriginalName,
			FileKey:      r.FileKey,
			UploadedAt:   r.UploadedAt,
			CreatedAt:    r.CreatedAt,
		})
	}
	return result, nil
}

// InsertBalancingEntryValue inserts a system-generated balancing row (form_field_id = NULL).
func (r *Repository) InsertBalancingEntryValue(ctx context.Context, tx *sqlx.Tx, ev *FormEntryValue) error {
	query := `INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, coa_id, net_amount, gst_amount, gross_amount, business_percentage)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, 100.00))`
	_, err := tx.ExecContext(ctx, query, ev.ID, ev.EntryID, ev.FormFieldID, ev.CoaID, ev.NetAmount, ev.GstAmount, ev.GrossAmount, ev.BusinessPercentage)
	if err != nil {
		return fmt.Errorf("insert balancing entry value: %w", err)
	}
	return nil
}

// InsertEntryValue inserts a new active entry value row for a field.
func (r *Repository) InsertEntryValue(ctx context.Context, tx *sqlx.Tx, ev *FormEntryValue) error {
	query := `INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, description, date, business_percentage)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := tx.ExecContext(ctx, query, ev.ID, ev.EntryID, ev.FormFieldID, ev.NetAmount, ev.GstAmount, ev.GrossAmount, ev.Description, ev.Date, ev.BusinessPercentage)
	if err != nil {
		return fmt.Errorf("insert entry value: %w", err)
	}
	return nil
}

// MarkEntryValueUpdated marks all active values for a given field+entry as superseded.
func (r *Repository) MarkEntryValueUpdated(ctx context.Context, tx *sqlx.Tx, fieldID uuid.UUID, entryID uuid.UUID) error {
	query := `UPDATE tbl_form_entry_value SET updated_at = now()
		WHERE form_field_id = $1 AND entry_id = $2 AND updated_at IS NULL`
	_, err := tx.ExecContext(ctx, query, fieldID, entryID)
	if err != nil {
		return fmt.Errorf("mark entry value updated: %w", err)
	}
	return nil
}

// DeleteSystemBalancingValues removes auto-generated balancing rows (form_field_id IS NULL) for an entry.
func (r *Repository) DeleteSystemBalancingValues(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID) error {
	query := `DELETE FROM tbl_form_entry_value WHERE entry_id = $1 AND form_field_id IS NULL AND updated_at IS NULL`
	_, err := tx.ExecContext(ctx, query, entryID)
	if err != nil {
		return fmt.Errorf("delete system balancing values: %w", err)
	}
	return nil
}

// GetActiveEntryValuesWithAccountType fetches all active entry value rows with their COA account type name.
// Used for rebalancing to determine income vs expense classification via COA, not section_type.
func (r *Repository) GetActiveEntryValuesWithAccountType(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID) ([]*EntryValueWithAccountType, error) {
	query := `
		SELECT fev.id, fev.entry_id, fev.form_field_id, fev.coa_id,
		       fev.net_amount, fev.gst_amount, fev.gross_amount, fev.description,
		       at.name AS account_type_name
		FROM tbl_form_entry_value fev
		LEFT JOIN tbl_form_field ff    ON ff.id = fev.form_field_id
		LEFT JOIN tbl_chart_of_accounts coa ON coa.id = COALESCE(fev.coa_id, ff.coa_id)
		LEFT JOIN tbl_account_type at  ON at.id = coa.account_type_id
		WHERE fev.entry_id = $1 AND fev.updated_at IS NULL`
	var rows []*EntryValueWithAccountType
	if err := tx.SelectContext(ctx, &rows, query, entryID); err != nil {
		return nil, fmt.Errorf("get active entry values with account type: %w", err)
	}
	return rows, nil
}

// UpdateEntryDate updates the date field on the parent entry row.
func (r *Repository) UpdateEntryDate(ctx context.Context, tx *sqlx.Tx, entryID uuid.UUID, date string) error {
	query := `UPDATE tbl_form_entry SET date = $1, updated_at = now() WHERE id = $2`
	_, err := tx.ExecContext(ctx, query, date, entryID)
	if err != nil {
		return fmt.Errorf("update entry date: %w", err)
	}
	return nil
}
