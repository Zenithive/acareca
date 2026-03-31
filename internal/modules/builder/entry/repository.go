package entry

import (
	"context"
	"database/sql"
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

	ListTransactions(ctx context.Context, f common.Filter) ([]*RsTransactionRow, error)
	CountTransactions(ctx context.Context, f common.Filter) (int, error)

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
			INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NULL)
			RETURNING created_at
		`
			if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.NetAmount, v.GstAmount, v.GrossAmount).
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
	"date_from":       "ev.created_at",
	"date_to":         "ev.created_at",
}

func (r *Repository) ListTransactions(ctx context.Context, f common.Filter) ([]*RsTransactionRow, error) {
	base := `
		SELECT
			ev.id,
			e.id            AS entry_id,
			ff.id           AS form_field_id,
			ff.label        AS form_field_name,
			coa.id          AS coa_id,
			coa.name        AS coa_name,
			at2.id          AS tax_type_id,
			at2.name        AS tax_type_name,
			fm.id           AS form_id,
			fm.name         AS form_name,
			e.clinic_id,
			c.name          AS clinic_name,
			ev.net_amount,
			ev.gst_amount,
			ev.gross_amount,
			ev.created_at,
			ev.updated_at
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry              e   ON e.id   = ev.entry_id          AND e.deleted_at  IS NULL
		INNER JOIN tbl_form_field              ff  ON ff.id  = ev.form_field_id     AND ff.deleted_at IS NULL
		INNER JOIN tbl_chart_of_accounts        coa ON coa.id = ff.coa_id            AND coa.deleted_at IS NULL
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version     fv  ON fv.id  = e.form_version_id    AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id  = fv.form_id           AND fm.deleted_at IS NULL
		INNER JOIN tbl_clinic                  c   ON c.id   = e.clinic_id          AND c.deleted_at  IS NULL
		WHERE e.deleted_at IS NULL AND ev.updated_at IS NULL`

	searchCols := []string{"ff.label", "coa.name", "fm.name", "c.name"}
	q, args := common.BuildQuery(base, f, allowedTransactionColumns, searchCols, false)
	q = sqlx.Rebind(sqlx.DOLLAR, q)

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
		})
	}
	return result, nil
}

func (r *Repository) CountTransactions(ctx context.Context, f common.Filter) (int, error) {
	base := `
		FROM tbl_form_entry_value ev
		INNER JOIN tbl_form_entry              e   ON e.id   = ev.entry_id          AND e.deleted_at  IS NULL
		INNER JOIN tbl_form_field              ff  ON ff.id  = ev.form_field_id     AND ff.deleted_at IS NULL
		INNER JOIN tbl_chart_of_accounts        coa ON coa.id = ff.coa_id            AND coa.deleted_at IS NULL
		LEFT  JOIN tbl_account_tax             at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version     fv  ON fv.id  = e.form_version_id    AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                    fm  ON fm.id  = fv.form_id           AND fm.deleted_at IS NULL
		INNER JOIN tbl_clinic                  c   ON c.id   = e.clinic_id          AND c.deleted_at  IS NULL
		WHERE e.deleted_at IS NULL`

	searchCols := []string{"ff.label", "coa.name", "fm.name", "c.name"}
	q, args := common.BuildQuery(base, f, allowedTransactionColumns, searchCols, true)
	q = sqlx.Rebind(sqlx.DOLLAR, q)

	var total int
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count transactions: %w", err)
	}
	return total, nil
}

// CreateTx - Transaction variant of Create
func (r *Repository) CreateTx(ctx context.Context, tx *sqlx.Tx, e *FormEntry, values []*FormEntryValue) error {
	query := `
		INSERT INTO tbl_form_entry (id, form_version_id, clinic_id, submitted_by, submitted_at, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	if err := tx.QueryRowContext(ctx, query,
		e.ID, e.FormVersionID, e.ClinicID, e.SubmittedBy, e.SubmittedAt, e.Status,
	).Scan(&e.CreatedAt, &e.UpdatedAt); err != nil {
		return fmt.Errorf("create form entry tx: %w", err)
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
        SET submitted_by = $1, submitted_at = $2, status = $3, updated_at = now()
        WHERE id = $4 AND deleted_at IS NULL
        RETURNING created_at, updated_at
    `
	if err := tx.QueryRowContext(ctx, query, e.SubmittedBy, e.SubmittedAt, e.Status, e.ID).
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
            INSERT INTO tbl_form_entry_value (id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, NULL)
            RETURNING created_at
        `
		if err := tx.QueryRowContext(ctx, valQuery, v.ID, v.EntryID, v.FormFieldID, v.NetAmount, v.GstAmount, v.GrossAmount).
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
