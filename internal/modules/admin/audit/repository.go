package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Insert(ctx context.Context, entry *LogEntry) error
	List(ctx context.Context, params QueryParams) ([]*AuditLog, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Insert(ctx context.Context, entry *LogEntry) error {
	query := `
		INSERT INTO tbl_audit_log (
			practice_id, user_id, action, module, entity_type, entity_id,
			before_state, after_state, ip_address, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	beforeJSON, err := toJSON(entry.BeforeState)
	if err != nil {
		return fmt.Errorf("marshal before_state: %w", err)
	}

	afterJSON, err := toJSON(entry.AfterState)
	if err != nil {
		return fmt.Errorf("marshal after_state: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		entry.PracticeID,
		entry.UserID,
		entry.Action,
		entry.Module,
		entry.EntityType,
		entry.EntityID,
		beforeJSON,
		afterJSON,
		entry.IPAddress,
		entry.UserAgent,
	)

	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

func (r *repository) List(ctx context.Context, params QueryParams) ([]*AuditLog, error) {
	query := `SELECT * FROM tbl_audit_log WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	if params.PracticeID != nil {
		query += fmt.Sprintf(" AND practice_id = $%d", argPos)
		args = append(args, *params.PracticeID)
		argPos++
	}

	if params.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, *params.UserID)
		argPos++
	}

	if params.Module != nil {
		query += fmt.Sprintf(" AND module = $%d", argPos)
		args = append(args, *params.Module)
		argPos++
	}

	if params.Action != nil {
		query += fmt.Sprintf(" AND action = $%d", argPos)
		args = append(args, *params.Action)
		argPos++
	}

	if params.EntityType != nil {
		query += fmt.Sprintf(" AND entity_type = $%d", argPos)
		args = append(args, *params.EntityType)
		argPos++
	}

	if params.EntityID != nil {
		query += fmt.Sprintf(" AND entity_id = $%d", argPos)
		args = append(args, *params.EntityID)
		argPos++
	}

	if params.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argPos)
		args = append(args, *params.StartDate)
		argPos++
	}

	if params.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argPos)
		args = append(args, *params.EndDate)
		argPos++
	}

	query += " ORDER BY created_at DESC"

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, params.Limit)
		argPos++
	} else {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, 100) // Default limit
		argPos++
	}

	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, params.Offset)
	}

	var logs []*AuditLog
	if err := r.db.SelectContext(ctx, &logs, query, args...); err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}

	return logs, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*AuditLog, error) {
	query := `SELECT * FROM tbl_audit_log WHERE id = $1`
	var log AuditLog
	if err := r.db.GetContext(ctx, &log, query, id); err != nil {
		return nil, fmt.Errorf("get audit log: %w", err)
	}
	return &log, nil
}

// Helper function to convert interface{} to JSON
func toJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// BuildFilterQuery is a helper for complex queries (future use)
func BuildFilterQuery(baseQuery string, filters map[string]interface{}) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argPos := 1

	for key, value := range filters {
		if value != nil {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", key, argPos))
			args = append(args, value)
			argPos++
		}
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	return baseQuery, args
}
