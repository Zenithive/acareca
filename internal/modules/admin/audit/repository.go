package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Insert(ctx context.Context, entry *LogEntry) error
	List(ctx context.Context, f common.Filter) ([]*AuditLog, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
	Count(ctx context.Context, f common.Filter) (int, error)
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

var allowedCols = map[string]string{
	"practice_id": "practice_id",
	"user_id":     "user_id",
	"module":      "module",
	"action":      "action",
	"entity_type": "entity_type",
	"entity_id":   "entity_id",
	"created_at":  "created_at",
}

func (r *repository) List(ctx context.Context, f common.Filter) ([]*AuditLog, error) {
	query := `SELECT * FROM tbl_audit_log WHERE 1=1`

	query, args := common.BuildQuery(query, f, allowedCols, nil, false)
	query = r.db.Rebind(query)

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

func (r *repository) Count(ctx context.Context, f common.Filter) (int, error) {
	base := `FROM tbl_audit_log WHERE 1=1`

	// Pass 'true' as the last argument to generate a COUNT(*) query
	query, args := common.BuildQuery(base, f, allowedCols, nil, true)
	query = r.db.Rebind(query)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count audit logs: %w", err)
	}
	return count, nil
}
