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

func (r *repository) ListCoaEntries(ctx context.Context, f common.Filter, actorID uuid.UUID, role string) ([]*RsCoaEntry, error) {
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
			v.clinic_id IN (SELECT id FROM tbl_clinic WHERE practitioner_id = ? AND deleted_at IS NULL)
			OR (v.clinic_id = '00000000-0000-0000-0000-000000000000' AND v.practitioner_id = ?)
		)`
	}

	allowedColumns := map[string]string{
		"clinic_id":       "v.clinic_id",
		"form_id":         "v.form_id",
		"coa_id":          "v.coa_id",
		"practitioner_id": "v.practitioner_id",
		"start_date":      "v.entry_date",
		"end_date":        "v.entry_date",
	}

	base := `
        SELECT
            v.coa_id                          AS coa_id,
            v.account_name                    AS coa_name,
            ff.section_type::text             AS section_type,
            ROUND(SUM(ABS(v.net_amount))::numeric, 2)::float8   AS total_net_amount,
			ROUND(SUM(ABS(v.gst_amount))::numeric, 2)::float8   AS total_gst_amount,
            ROUND(SUM(ABS(v.gross_amount))::numeric, 2)::float8 AS total_gross_amount,
            COUNT(fev.id)                     AS entry_count
        FROM vw_double_entry_line_items v
        INNER JOIN tbl_form_field ff ON ff.id = v.form_field_id AND ff.deleted_at IS NULL
        INNER JOIN tbl_form_entry_value fev
			ON fev.entry_id = v.entry_id
				AND fev.deleted_at IS NULL
				AND fev.updated_at IS NULL
				AND fev.form_field_id = v.form_field_id
        WHERE ff.section_type IN ('COST', 'OTHER_COST', 'COLLECTION') ` + permissionClause

	q, qArgs := common.BuildQuery(base, f, allowedColumns, []string{"v.account_name"}, false)

	groupBy := ` GROUP BY v.coa_id, v.account_name, ff.section_type `

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
