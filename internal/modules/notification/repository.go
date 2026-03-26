package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateNotification(ctx context.Context, notification Notification) error
	ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateNotification(ctx context.Context, notification Notification) error {
	payloadBytes, err := json.Marshal(notification.Payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	const q = `
		INSERT INTO tbl_notification (
			recipient_id, sender_id, event_type, entity_type, entity_id, status, payload
		) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6)
	`
	_, err = r.db.ExecContext(
		ctx,
		q,
		notification.RecipientID,
		notification.SenderID,
		notification.EventType,
		notification.EntityType,
		notification.EntityID,
		payloadBytes,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *repository) ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, error) {
	args := []any{recipientID}
	where := "WHERE recipient_id = $1 AND status != 'DISMISSED'"

	if filter.Status != nil {
		args = append(args, *filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tbl_notification "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, recipient_id, sender_id, event_type, entity_type, entity_id,
		       status, payload, retry_count, created_at, read_at AS readed_at
		FROM tbl_notification
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	var rows []Notification
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return rows, total, nil
}

func (r *repository) MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification SET status = 'READ', read_at = NOW() WHERE id = $1 AND recipient_id = $2`,
		id, recipientID,
	)
	return err
}

func (r *repository) MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification SET status = 'DISMISSED' WHERE id = $1 AND recipient_id = $2`,
		id, recipientID,
	)
	return err
}
