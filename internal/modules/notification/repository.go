package notification

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound           = errors.New("notification not found")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrMaxRetriesExceeded = errors.New("max retry count exceeded")
)

const maxRetries = 5

type Repository interface {
	CreateNotification(ctx context.Context, notification Notification) error
	ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, error)
	MarkDelivered(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	Retry(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateNotification(ctx context.Context, notification Notification) error {
	const q = `
		INSERT INTO tbl_notification (
			recipient_id, recipient_type, sender_id, sender_type,
			event_type, entity_type, entity_id, status, payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'PENDING', $8)
	`
	_, err := r.db.ExecContext(ctx, q,
		notification.RecipientID,
		notification.RecipientType,
		notification.SenderID,
		notification.SenderType,
		notification.EventType,
		notification.EntityType,
		notification.EntityID,
		notification.Payload,
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

// MarkDelivered bulk-transitions PENDING → DELIVERED (called on WS connect).
func (r *repository) MarkDelivered(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(
		`UPDATE tbl_notification
		 SET status = 'DELIVERED'
		 WHERE id IN (?) AND recipient_id = ? AND status = 'PENDING'`,
		ids, recipientID,
	)
	if err != nil {
		return fmt.Errorf("build mark delivered query: %w", err)
	}
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

// MarkRead transitions PENDING|DELIVERED → READ.
func (r *repository) MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'READ', read_at = NOW()
		 WHERE id = $1 AND recipient_id = $2 AND status IN ('PENDING', 'DELIVERED')`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

// MarkDismissed transitions READ → DISMISSED.
func (r *repository) MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'DISMISSED'
		 WHERE id = $1 AND recipient_id = $2 AND status = 'READ'`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

// MarkFailed transitions PENDING|DELIVERED → FAILED (used by delivery workers).
func (r *repository) MarkFailed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'FAILED', retry_count = retry_count + 1
		 WHERE id = $1 AND recipient_id = $2 AND status IN ('PENDING', 'DELIVERED')`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

// Retry resets FAILED → PENDING if under the retry cap.
func (r *repository) Retry(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	// Check current retry_count first
	var retryCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT retry_count FROM tbl_notification WHERE id = $1 AND recipient_id = $2 AND status = 'FAILED'`,
		id, recipientID,
	).Scan(&retryCount)
	if err != nil {
		return ErrNotFound
	}
	if retryCount >= maxRetries {
		return ErrMaxRetriesExceeded
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification SET status = 'PENDING' WHERE id = $1 AND recipient_id = $2 AND status = 'FAILED'`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

// requireOneRow returns errIfZero when no rows were affected (wrong state / not found).
func requireOneRow(res interface{ RowsAffected() (int64, error) }, errIfZero error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errIfZero
	}
	return nil
}
