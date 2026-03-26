package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateNotification(ctx context.Context, notification Notification) error
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
