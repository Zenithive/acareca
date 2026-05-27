package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

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
	CreateNotificationWithDeliveries(ctx context.Context, tx *sqlx.Tx, notification Notification, channels []Channel) (uuid.UUID, error)
	ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, error)
	GetUnreadCount(ctx context.Context, recipientID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, id uuid.UUID, recipientID uuid.UUID) error
	MarkAllRead(ctx context.Context, recipientID uuid.UUID) error
	MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error
	// Delivery worker methods
	ListFailedInAppDeliveries(ctx context.Context, limit int) ([]FailedDelivery, error)
	MarkDeliveryDelivered(ctx context.Context, notificationID uuid.UUID, channel Channel) error
	MarkDeliveryFailed(ctx context.Context, notificationID uuid.UUID, channel Channel, errMsg string) error
	RetryDelivery(ctx context.Context, notificationID uuid.UUID, channel Channel) error
	// Deduplication check for system error/warning notifications
	HasActiveSystemNotification(ctx context.Context, entityID uuid.UUID, eventType EventType) (bool, error)
	GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error)
	CreatePreference(ctx context.Context, pref NotificationPreference, tx *sqlx.Tx) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateNotificationWithDeliveries(ctx context.Context, tx *sqlx.Tx, notification Notification, channels []Channel) (uuid.UUID, error) {
	const insertNotificationQuery = `
		INSERT INTO tbl_notification (
			id, recipient_id, recipient_type, sender_id, sender_type,
			event_type, entity_type, entity_id, status, payload, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	var id uuid.UUID
	err := tx.QueryRowContext(ctx, insertNotificationQuery,
		notification.ID,
		notification.RecipientID,
		notification.RecipientType,
		notification.SenderID,
		notification.SenderType,
		notification.EventType,
		notification.EntityType,
		notification.EntityID,
		notification.Status,
		notification.Payload,
		notification.CreatedAt,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert notification: %w", err)
	}

	for _, ch := range channels {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO tbl_notification_delivery (notification_id, channel) VALUES ($1, $2)`,
			id, ch,
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert delivery for channel %s: %w", ch, err)
		}
	}

	return id, nil
}

func (r *repository) ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter FilterNotification) ([]Notification, int, error) {
	args := []any{recipientID}
	where := "WHERE recipient_id = $1 AND status != 'DISMISSED'"

	if filter.Status != nil && *filter.Status != "" {
		args = append(args, *filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if filter.Search != nil && *filter.Search != "" {
		args = append(args, "%"+*filter.Search+"%")
		where += fmt.Sprintf(" AND (event_type ILIKE $%d OR payload::text ILIKE $%d)", len(args), len(args))
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tbl_notification "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	limit := 10
	if filter.Limit != nil && *filter.Limit > 0 {
		limit = *filter.Limit
	}

	offset := 0
	if filter.Offset != nil && *filter.Offset >= 0 {
		offset = *filter.Offset
	}

	q := fmt.Sprintf(`
		SELECT id, recipient_id, sender_id, event_type, entity_type, entity_id,
		       status, payload, created_at, read_at AS readed_at
		FROM tbl_notification
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	args = append(args, limit, offset)

	var rows []Notification
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}

	return rows, total, nil
}

func (r *repository) GetUnreadCount(ctx context.Context, recipientID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM tbl_notification WHERE recipient_id = $1 AND status = 'UNREAD' AND status != 'DISMISSED'`
	err := r.db.GetContext(ctx, &count, query, recipientID)
	return count, err
}

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

func (r *repository) MarkDismissed(ctx context.Context, ids []uuid.UUID, recipientID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification 
         SET status = 'READ', read_at = NOW() 
         WHERE recipient_id = $1 
           AND status = 'UNREAD' 
           AND id = ANY($2)`,
		recipientID, ids,
	)
	if err != nil {
		log.Printf("Warning: Failed to mark notifications as READ before dismissing: %v", err)
	}

	res, err := r.db.ExecContext(ctx,
		`UPDATE tbl_notification 
         SET status = 'DISMISSED' 
         WHERE recipient_id = $1 
           AND status IN ('UNREAD', 'READ') 
           AND id = ANY($2)`,
		recipientID, ids,
	)
	if err != nil {
		return err
	}

	return requireOneRow(res, ErrInvalidTransition)
}

func (r *repository) ListFailedInAppDeliveries(ctx context.Context, limit int) ([]FailedDelivery, error) {
	const q = `
		SELECT d.notification_id, n.recipient_id, d.retry_count,
		       n.event_type, n.entity_type, n.entity_id, n.payload, n.created_at
		FROM tbl_notification_delivery d
		JOIN tbl_notification n ON n.id = d.notification_id
		WHERE d.channel = 'in_app'
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
		 WHERE notification_id = $1 AND channel = $2 AND status IN ('PENDING', 'FAILED')`,
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

func (r *repository) GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]NotificationPreference, error) {
	prefs := make([]NotificationPreference, 0)
	const q = `
		SELECT id, user_id, entity_id, entity_type, event_type, channels, created_at, updated_at
		FROM tbl_notification_preferences
		WHERE user_id = $1 AND deleted_at IS NULL`

	rows, err := r.db.QueryxContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p NotificationPreference
		if err := rows.StructScan(&p); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, nil
}

func (r *repository) CreatePreference(ctx context.Context, p NotificationPreference, tx *sqlx.Tx) error {
	const q = `
		INSERT INTO tbl_notification_preferences (
			user_id,
			entity_id,
			entity_type,
			event_type,
			channels,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, entity_id, event_type)
		DO UPDATE SET
			channels = EXCLUDED.channels,
			entity_type = EXCLUDED.entity_type,
			updated_at = NOW(),
			deleted_at = NULL
	`

	channelsJSON, err := json.Marshal(p.Channels)
	if err != nil {
		return fmt.Errorf("marshal channels: %w", err)
	}

	// event_type is a single enum column — use the first value from the slice
	if len(p.EventType) == 0 {
		return fmt.Errorf("event_type is required")
	}

	_, err = tx.ExecContext(
		ctx,
		q,
		p.UserID,
		p.EntityID,
		p.EntityType,
		string(p.EventType[0]),
		channelsJSON,
	)

	return err
}
