package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Insert(ctx context.Context, entry *LogEntry) error
	List(ctx context.Context, f common.Filter) ([]*AuditLog, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
	Count(ctx context.Context, f common.Filter) (int, error)
	GetAdminIDs(ctx context.Context) ([]uuid.UUID, error)
	GetUserIDByPractitionerID(ctx context.Context, practitionerID string) (string, error)
	GetUserName(ctx context.Context, id string) (string, error)
	GetEntityName(ctx context.Context, table string, id string) (string, error)
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

	searchCols := []string{"module", "action"}
	query, args := common.BuildQuery(query, f, allowedCols, searchCols, false)
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
	searchCols := []string{"module", "action"}
	query, args := common.BuildQuery(base, f, allowedCols, searchCols, true)
	query = r.db.Rebind(query)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count audit logs: %w", err)
	}
	return count, nil
}

func (r *repository) GetAdminIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	query := `SELECT id FROM tbl_admin WHERE deleted_at IS NULL`

	err := r.db.SelectContext(ctx, &ids, query)
	return ids, err
}

func (r *repository) GetUserIDByPractitionerID(ctx context.Context, practitionerID string) (string, error) {
	var userID string
	// We fetch the user_id associated with this practitioner
	query := `SELECT user_id FROM tbl_practitioner WHERE id = $1`

	err := r.db.GetContext(ctx, &userID, query, practitionerID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (r *repository) GetUserName(ctx context.Context, id string) (string, error) {
	var name string
	// Using COALESCE to handle potential nulls in names, falling back to an empty string if both are null
	query := `SELECT COALESCE(first_name || ' ' || last_name) FROM tbl_user WHERE id = $1`

	err := r.db.GetContext(ctx, &name, query, id)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (r *repository) GetEntityName(ctx context.Context, table string, id string) (string, error) {
	var name, query string

	// Select 'name' from the provided table name
	query = fmt.Sprintf(`SELECT name FROM %s WHERE id = $1`, table)

	err := r.db.GetContext(ctx, &name, query, id)
	if err != nil {
		return "", err
	}
	return name, nil
}
