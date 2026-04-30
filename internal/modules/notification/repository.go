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
	CreateNotification(ctx context.Context, notification Notification) (uuid.UUID, error)
	CreateDeliveries(ctx context.Context, notificationID uuid.UUID, channels []Channel) error
	ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, int, int, error)
	GetUnreadCount(ctx context.Context, recipientID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	// Delivery worker methods
	ListFailedInAppDeliveries(ctx context.Context, limit int) ([]FailedDelivery, error)
	MarkDeliveryDelivered(ctx context.Context, notificationID uuid.UUID, channel Channel) error
	MarkDeliveryFailed(ctx context.Context, notificationID uuid.UUID, channel Channel, errMsg string) error
	RetryDelivery(ctx context.Context, notificationID uuid.UUID, channel Channel) error
	// Deduplication check for system error/warning notifications
	HasActiveSystemNotification(ctx context.Context, entityID uuid.UUID, eventType EventType) (bool, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateNotification(ctx context.Context, notification Notification) (uuid.UUID, error) {
	const q = `
		INSERT INTO tbl_notification (
			recipient_id, recipient_type, sender_id, sender_type,
			event_type, entity_type, entity_id, status, payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'UNREAD', $8)
		RETURNING id
	`
	var id uuid.UUID
	err := r.db.QueryRowContext(ctx, q,
		notification.RecipientID,
		notification.RecipientType,
		notification.SenderID,
		notification.SenderType,
		notification.EventType,
		notification.EntityType,
		notification.EntityID,
		notification.Payload,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert notification: %w", err)
	}
	return id, nil
}

func (r *repository) CreateDeliveries(ctx context.Context, notificationID uuid.UUID, channels []Channel) error {
	for _, ch := range channels {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO tbl_notification_delivery (notification_id, channel) VALUES ($1, $2)`,
			notificationID, ch,
		)
		if err != nil {
			return fmt.Errorf("insert delivery for channel %s: %w", ch, err)
		}
	}
	return nil
}

func (r *repository) ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, int, int, error) {
	args := []any{recipientID}
	where := "WHERE recipient_id = $1 AND status != 'DISMISSED'"

	if filter.Status != nil && *filter.Status != "" {
		args = append(args, *filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tbl_notification "+where, args...).Scan(&total); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("count notifications: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	q := fmt.Sprintf(`
		SELECT id, recipient_id, sender_id, event_type, entity_type, entity_id,
		       status, payload, created_at, read_at AS readed_at
		FROM tbl_notification
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	// Append limit and offset to args AFTER the where clause params
	args = append(args, limit, offset)

	var rows []Notification
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("list notifications: %w", err)
	}

	return rows, total, page, limit, nil
}

func (r *repository) GetUnreadCount(ctx context.Context, recipientID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM tbl_notification WHERE recipient_id = $1 AND status = 'UNREAD'`
	err := r.db.GetContext(ctx, &count, query, recipientID)
	return count, err
}

// MarkRead transitions UNREAD → READ.
func (r *repository) MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'READ', read_at = NOW()
		 WHERE id = $1 AND recipient_id = $2 AND status = 'UNREAD'`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

func (r *repository) MarkAllRead(ctx context.Context, recipientID uuid.UUID) error {
	query := `
		UPDATE tbl_notification 
		SET status = 'READ', read_at = NOW() 
		WHERE recipient_id = $1 AND status = 'UNREAD'`

	_, err := r.db.ExecContext(ctx, query, recipientID)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

// MarkDismissed transitions UNREAD → DISMISSED or READ → DISMISSED.
func (r *repository) MarkDismissed(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error {
	// First, try to mark as READ if it's UNREAD (this may fail if already READ, which is fine)
	_, _ = r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'READ', read_at = NOW()
		 WHERE id = $1 AND recipient_id = $2 AND status = 'UNREAD'`,
		id, recipientID,
	)

	// Now mark as DISMISSED (works for both UNREAD and READ status)
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification
		 SET status = 'DISMISSED'
		 WHERE id = $1 AND recipient_id = $2 AND status IN ('UNREAD', 'READ')`,
		id, recipientID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

// ListFailedInAppDeliveries returns FAILED in_app deliveries that are still under the retry cap.
func (r *repository) ListFailedInAppDeliveries(ctx context.Context, limit int) ([]FailedDelivery, error) {
	const q = `
		SELECT d.notification_id, n.recipient_id, d.retry_count,
		       n.event_type, n.entity_type, n.entity_id, n.payload, n.created_at
		FROM tbl_notification_delivery d
		JOIN tbl_notification n ON n.id = d.notification_id
		WHERE d.channel = 'email'
		  AND d.status = 'FAILED'
		  AND d.retry_count < $1
		  AND n.status != 'DISMISSED'
		ORDER BY n.created_at ASC
		LIMIT $2
	`
	var rows []FailedDelivery
	if err := r.db.SelectContext(ctx, &rows, q, maxRetries, limit); err != nil {
		return nil, fmt.Errorf("list failed in_app deliveries: %w", err)
	}
	return rows, nil
}

func (r *repository) MarkDeliveryDelivered(ctx context.Context, notificationID uuid.UUID, channel Channel) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification_delivery
		 SET status = 'DELIVERED', delivered_at = NOW(), last_attempted_at = NOW()
		 WHERE notification_id = $1 AND channel = $2 AND status IN ('PENDING', 'FAILED')`,
		notificationID, channel,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

func (r *repository) MarkDeliveryFailed(ctx context.Context, notificationID uuid.UUID, channel Channel, errMsg string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification_delivery
		 SET status = 'FAILED', retry_count = retry_count + 1,
		     last_attempted_at = NOW(), error_message = $3
		 WHERE notification_id = $1 AND channel = $2 AND status IN ('PENDING', 'DELIVERED')`,
		notificationID, channel, errMsg,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res, ErrInvalidTransition)
}

func (r *repository) RetryDelivery(ctx context.Context, notificationID uuid.UUID, channel Channel) error {
	var retryCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT retry_count FROM tbl_notification_delivery
		 WHERE notification_id = $1 AND channel = $2 AND status = 'FAILED'`,
		notificationID, channel,
	).Scan(&retryCount)
	if err != nil {
		return ErrNotFound
	}
	if retryCount >= maxRetries {
		return ErrMaxRetriesExceeded
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification_delivery SET status = 'PENDING'
		 WHERE notification_id = $1 AND channel = $2 AND status = 'FAILED'`,
		notificationID, channel,
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

// HasActiveSystemNotification checks if an UNREAD system notification already exists
// for the given entityID + eventType to prevent duplicate alert fatigue.
func (r *repository) HasActiveSystemNotification(ctx context.Context, entityID uuid.UUID, eventType EventType) (bool, error) {
	var count int
	const q = `
		SELECT COUNT(*) FROM tbl_notification
		WHERE entity_id = $1
		  AND event_type = $2
		  AND entity_type = 'system'
		  AND status = 'UNREAD'
	`
	if err := r.db.QueryRowContext(ctx, q, entityID, eventType).Scan(&count); err != nil {
		return false, fmt.Errorf("check active system notification: %w", err)
	}
	return count > 0, nil
}
