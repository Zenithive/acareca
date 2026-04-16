package calculation

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetTransactionsByFormID(ctx context.Context, formID string, actorID uuid.UUID, role string) ([]*RsTransactionRow, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetTransactionsByFormID(ctx context.Context, formID string, actorID uuid.UUID, role string) ([]*RsTransactionRow, error) {
	var permissionClause string

	// Handle Permissions based on Role
	if strings.EqualFold(role, util.RoleAccountant) {
		permissionClause = ` AND fm.id IN (
			SELECT entity_id FROM tbl_invite_permissions 
			WHERE accountant_id = ? AND entity_type = 'FORM' AND deleted_at IS NULL
		)`
	} else {
		permissionClause = ` AND c.id IN (
			SELECT id FROM tbl_clinic 
			WHERE practitioner_id = ? AND deleted_at IS NULL
		)`
	}

	query := `
		SELECT
			ev.id,
			e.id            AS entry_id,
			ff.id           AS form_field_id,
			ff.label        AS form_field_name,
			ff.section_type AS section_type,
			ff.tax_type     AS tax_type,
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
		INNER JOIN tbl_form_entry           e   ON e.id   = ev.entry_id          AND e.deleted_at  IS NULL
		INNER JOIN tbl_form_field           ff  ON ff.id  = ev.form_field_id     AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
		INNER JOIN tbl_chart_of_accounts    coa ON coa.id = ff.coa_id            AND coa.deleted_at IS NULL AND coa.is_system = FALSE
		LEFT  JOIN tbl_account_tax          at2 ON at2.id = coa.account_tax_id
		INNER JOIN tbl_custom_form_version  fv  ON fv.id  = e.form_version_id    AND fv.deleted_at IS NULL
		INNER JOIN tbl_form                 fm  ON fm.id  = fv.form_id           AND fm.deleted_at IS NULL
		INNER JOIN tbl_clinic               c   ON c.id   = e.clinic_id          AND c.deleted_at  IS NULL
		WHERE fm.id = ? 
		  AND e.deleted_at IS NULL 
		  AND ev.updated_at IS NULL` + permissionClause

	// Rebind for positional parameters ($1 vs ?)
	query = r.db.Rebind(query)

	var rows []*transactionFlatRow
	// Arguments: 1. formID, 2. actorID (for permissionClause)
	if err := r.db.SelectContext(ctx, &rows, query, formID, actorID); err != nil {
		return nil, fmt.Errorf("list transactions by form: %w", err)
	}

	result := make([]*RsTransactionRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, &RsTransactionRow{
			ID:            row.ID,
			EntryID:       row.EntryID,
			FormFieldID:   row.FormFieldID,
			FormFieldName: row.FormFieldName,
			SectionType:   row.SectionType,
			TaxType:       row.TaxType,
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
