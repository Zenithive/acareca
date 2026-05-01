package calculation

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

// ListCoaEntries returns grouped COA rows with aggregated amounts and section types
func (r *repository) ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error) {
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
			c.id IN (SELECT id FROM tbl_clinic WHERE practitioner_id = ? AND deleted_at IS NULL)
			OR (e.clinic_id = '00000000-0000-0000-0000-000000000000' AND fv.practitioner_id = ?)
		)`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "e.clinic_id",
		"form_id":         "fm.id",
		"coa_id":          "coa.id",
		"practitioner_id": "COALESCE(c.practitioner_id, fv.practitioner_id)",
	}

	base := `
        SELECT
            coa.id                            AS coa_id,
            coa.name                          AS coa_name,
            ff.section_type                   AS section_type,
            COALESCE(SUM(ev.net_amount), 0)   AS total_net_amount,
			COALESCE(SUM(ev.gst_amount), 0)   AS total_gst_amount,
            COALESCE(SUM(ev.gross_amount), 0) AS total_gross_amount,
            COUNT(DISTINCT ev.id)             AS entry_count
        FROM tbl_chart_of_accounts coa
        INNER JOIN tbl_form_field ff ON ff.coa_id = coa.id AND ff.deleted_at IS NULL AND ff.is_formula = FALSE
        INNER JOIN tbl_form_entry_value ev ON ev.form_field_id = ff.id AND ev.updated_at IS NULL
        INNER JOIN tbl_form_entry e ON e.id = ev.entry_id AND e.deleted_at IS NULL
        INNER JOIN tbl_custom_form_version fv ON fv.id = e.form_version_id AND fv.deleted_at IS NULL
        INNER JOIN tbl_form fm ON fm.id = fv.form_id AND fm.deleted_at IS NULL
        LEFT  JOIN tbl_clinic c ON c.id = e.clinic_id AND c.deleted_at IS NULL
        WHERE coa.deleted_at IS NULL AND coa.is_system = FALSE AND ff.section_type IS NOT NULL` + permissionClause

	q, qArgs := common.BuildQuery(base, f, allowedColumns, []string{"coa.name"}, false)

	groupBy := ` GROUP BY coa.id, coa.name, ff.section_type `

	var finalQuery string
	if strings.Contains(q, "ORDER BY") {
		finalQuery = strings.Replace(q, "ORDER BY", groupBy+" ORDER BY", 1)
	} else if strings.Contains(q, "LIMIT") {
		finalQuery = strings.Replace(q, "LIMIT", groupBy+" LIMIT", 1)
	} else {
		finalQuery = q + groupBy
	}

	q = r.db.Rebind(finalQuery)
	actualArgs := append([]any{actorID, actorID}, qArgs...)

	var rows []struct {
		CoaID            uuid.UUID `db:"coa_id"`
		CoaName          string    `db:"coa_name"`
		SectionType      string    `db:"section_type"`
		TotalNetAmount   float64   `db:"total_net_amount"`
		TotalGSTAmount   float64   `db:"total_gst_amount"`
		TotalGrossAmount float64   `db:"total_gross_amount"`
		EntryCount       int       `db:"entry_count"`
	}

	if err := r.db.SelectContext(ctx, &rows, q, actualArgs...); err != nil {
		return nil, err
	}

	result := make([]*RsCoaEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &RsCoaEntry{
			CoaID:            row.CoaID.String(),
			CoaName:          row.CoaName,
			SectionType:      row.SectionType,
			TotalNetAmount:   row.TotalNetAmount,
			TotalGSTAmount:   row.TotalGSTAmount,
			TotalGrossAmount: row.TotalGrossAmount,
			EntryCount:       row.EntryCount,
		})
	}
	return result, nil
}
