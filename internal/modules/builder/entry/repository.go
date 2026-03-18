package entry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("form entry not found")

type IRepository interface {
	Create(ctx context.Context, e *FormEntry, values []*FormEntryValue) error
	GetByID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)
	Update(ctx context.Context, e *FormEntry, values []*FormEntryValue) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter) ([]*FormEntry, error)
	CountByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter) (int, error)
	HasSubmittedEntryValuesForField(ctx context.Context, formFieldID uuid.UUID) (bool, error)

	GetByVersionID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error)

	ListTransactions(ctx context.Context, f TransactionFilter) ([]*RsTransaction, error)
	CountTransactions(ctx context.Context, f TransactionFilter) (int, error)
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
		INSERT INTO tbl_form_entry (id, form_version_id, clinic_id, submitted_by, submitted_at, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		e.ID, e.FormVersionID, e.ClinicID, e.SubmittedBy, e.SubmittedAt, e.Status,
	).Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		return fmt.Errorf("create form entry: %w", err)
	}

	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, net_amount, gst_amount, gross_amount)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at, updated_at
		`
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.NetAmount, v.GstAmount, v.GrossAmount).
			Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
			return fmt.Errorf("create entry value: %w", err)
		}
	}

	return tx.Commit()
}

// GetByID implements [IRepository].
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*FormEntry, []*FormEntryValue, error) {
	query := `SELECT id, form_version_id, clinic_id, submitted_by, submitted_at, status, created_at, updated_at
		FROM tbl_form_entry WHERE id = $1 AND deleted_at IS NULL`
	var e FormEntry
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	valQuery := `SELECT id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, created_at, updated_at
		FROM tbl_form_entry_value WHERE entry_id = $1`
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

	query := `
		UPDATE tbl_form_entry
		SET submitted_by = $1, submitted_at = $2, status = $3, updated_at = now()
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query, e.SubmittedBy, e.SubmittedAt, e.Status, e.ID).
		Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update form entry: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM tbl_form_entry_value WHERE entry_id = $1`, e.ID); err != nil {
		return fmt.Errorf("delete entry values: %w", err)
	}
	for _, v := range values {
		v.EntryID = e.ID
		valQuery := `
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, net_amount, gst_amount, gross_amount)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at, updated_at
		`
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.NetAmount, v.GstAmount, v.GrossAmount).
			Scan(&v.CreatedAt, &v.UpdatedAt); err != nil {
			return fmt.Errorf("insert entry value: %w", err)
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
func (r *Repository) ListByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter) ([]*FormEntry, error) {
	allowedColumns := map[string]string{
		"clinic_id":  "clinic_id",
		"created_at": "created_at",
		"status":     "status",
	}
	base := `SELECT id, form_version_id, clinic_id, submitted_by, submitted_at, status, created_at, updated_at
		FROM tbl_form_entry WHERE form_version_id = ? AND deleted_at IS NULL`
	q, args := common.BuildQuery(base, f, allowedColumns, []string{"status"}, false)
	q = sqlx.Rebind(sqlx.DOLLAR, q)
	args = append([]interface{}{formVersionID}, args...)
	var list []*FormEntry
	if err := r.db.SelectContext(ctx, &list, q, args...); err != nil {
		return nil, fmt.Errorf("list form entries: %w", err)
	}
	return list, nil
}

// CountByFormVersionID implements [IRepository].
func (r *Repository) CountByFormVersionID(ctx context.Context, formVersionID uuid.UUID, f common.Filter) (int, error) {
	allowedColumns := map[string]string{
		"clinic_id":  "clinic_id",
		"created_at": "created_at",
		"status":     "status",
	}
	base := `FROM tbl_form_entry WHERE form_version_id = ? AND deleted_at IS NULL`
	q, args := common.BuildQuery(base, f, allowedColumns, []string{"status"}, true)
	q = sqlx.Rebind(sqlx.DOLLAR, q)
	args = append([]interface{}{formVersionID}, args...)
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
	query := `SELECT id, form_version_id, clinic_id, submitted_by, submitted_at, status, created_at, updated_at
		FROM tbl_form_entry WHERE form_version_id = $1 AND deleted_at IS NULL`
	var e FormEntry
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.FormVersionID, &e.ClinicID, &e.SubmittedBy, &e.SubmittedAt, &e.Status, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get form entry: %w", err)
	}

	valQuery := `SELECT id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, created_at, updated_at
		FROM tbl_form_entry_value WHERE entry_id = $1`
	var values []*FormEntryValue
	if err := r.db.SelectContext(ctx, &values, valQuery, e.ID); err != nil {
		return nil, nil, fmt.Errorf("get entry values: %w", err)
	}
	return &e, values, nil
}

var transactionAllowedColumns = map[string]string{
	"clinic_id":  "e.clinic_id",
	"form_id":    "fm.id",
	"version_id": "e.form_version_id",
	"status":     "e.status",
	"created_at": "e.created_at",
}

func (r *Repository) CountTransactions(ctx context.Context, f TransactionFilter) (int, error) {
	base := `FROM tbl_form_entry e
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		INNER JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		WHERE e.deleted_at IS NULL`

	q, args := common.BuildQuery(base, f.ToCommonFilter(), transactionAllowedColumns, nil, true)
	q = sqlx.Rebind(sqlx.DOLLAR, q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count transactions: %w", err)
	}
	return total, nil
}

func (r *Repository) ListTransactions(ctx context.Context, f TransactionFilter) ([]*RsTransaction, error) {
	base := `SELECT
			e.id,
			e.form_version_id,
			e.clinic_id,
			c.name AS clinic_name,
			fm.id   AS form_id,
			fm.name AS form_name,
			fm.method,
			e.status AS form_status,
			COALESCE(
				json_agg(
					json_build_object(
						'field_name', ff.label,
						'gst_type',   ff.tax_type,
						'amount',     ev.gross_amount,
						'gst_amount', ev.gst_amount,
						'net_amount', ev.net_amount
					) ORDER BY ff.sort_order
				) FILTER (WHERE ev.id IS NOT NULL),
				'[]'
			) AS entry_detail
		FROM tbl_form_entry e
		INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
		INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
		INNER JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
		LEFT JOIN tbl_form_entry_value ev ON ev.entry_id = e.id
		LEFT JOIN tbl_form_field ff ON ff.id = ev.form_field_id AND ff.deleted_at IS NULL
		WHERE e.deleted_at IS NULL`

	cf := f.ToCommonFilter()
	cf.Limit = 0
	cf.Offset = 0
	q, args := common.BuildQuery(base, cf, transactionAllowedColumns, nil, false)

	cf2 := f.ToCommonFilter()
	q += " GROUP BY e.id, c.name, fm.id, fm.name, fm.method, e.status, e.created_at ORDER BY e.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, cf2.Limit, cf2.Offset)

	q = sqlx.Rebind(sqlx.DOLLAR, q)

	rows, err := r.db.QueryxContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var result []*RsTransaction
	for rows.Next() {
		var row transactionRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("scan transaction row: %w", err)
		}
		tx := &RsTransaction{
			ID:            row.ID,
			FormVersionID: row.FormVersionID,
			ClinicID:      row.ClinicID,
			ClinicName:    row.ClinicName,
			FormID:        row.FormID,
			FormName:      row.FormName,
			Method:        row.Method,
			FormStatus:    row.FormStatus,
			EntryDetail:   []RsTransactionDetail{},
		}
		if len(row.EntryDetailRaw) > 0 {
			if err := json.Unmarshal(row.EntryDetailRaw, &tx.EntryDetail); err != nil {
				return nil, fmt.Errorf("unmarshal entry_detail: %w", err)
			}
		}
		result = append(result, tx)
	}
	return result, rows.Err()
}
