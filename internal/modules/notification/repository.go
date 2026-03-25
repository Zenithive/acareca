package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateNotification(ctx context.Context, recipientID uuid.UUID, senderID *uuid.UUID, eventType EventType, entityType EntityType, entityID uuid.UUID, payload NotificationPayload) error

	ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter common.Filter) ([]Notification, int, int, error)

	MarkRead(ctx context.Context, recipientID, notificationID uuid.UUID) error
	MarkDismissed(ctx context.Context, recipientID, notificationID uuid.UUID) error

	GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error)
	GetUserIDByPractitionerID(ctx context.Context, practitionerID uuid.UUID) (*uuid.UUID, error)
	GetPractitionerUserIDByClinicID(ctx context.Context, clinicID uuid.UUID) (*uuid.UUID, error)
	GetPractitionerUserIDByFormID(ctx context.Context, formID uuid.UUID) (*uuid.UUID, error)
	GetPractitionerUserIDByEntryID(ctx context.Context, entryID uuid.UUID) (*uuid.UUID, error)
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateNotification(
	ctx context.Context,
	recipientID uuid.UUID,
	senderID *uuid.UUID,
	eventType EventType,
	entityType EntityType,
	entityID uuid.UUID,
	payload NotificationPayload,
) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	const q = `
		INSERT INTO tbl_notification (
			recipient_id, sender_id, event_type, entity_type, entity_id, status, payload
		) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6)
	`
	_, err = r.db.ExecContext(ctx, q, recipientID, senderID, eventType, entityType, entityID, payloadBytes)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *repository) ListByRecipient(ctx context.Context, recipientID uuid.UUID, filter common.Filter) ([]Notification, int, int, error) {

	allowedColumns := map[string]string{
		"status":      "status",
		"event_type":  "event_type",
		"entity_type": "entity_type",
		"created_at":  "created_at",
	}

	base := `FROM tbl_notification WHERE recipient_id = '` + recipientID.String() + `'`

	countQuery, countArgs := common.BuildQuery(base, filter, allowedColumns, nil, true)
	countQuery = r.db.Rebind(countQuery)

	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, 0, fmt.Errorf("count notifications: %w", err)
	}

	// Unread count is always PENDING regardless of applied filters
	var unread int
	unreadQuery := r.db.Rebind(`SELECT COUNT(*) FROM tbl_notification WHERE recipient_id = $1 AND status = 'PENDING'`)
	if err := r.db.GetContext(ctx, &unread, unreadQuery, recipientID); err != nil {
		return nil, 0, 0, fmt.Errorf("count unread notifications: %w", err)
	}

	selectBase := `SELECT id, recipient_id, sender_id, event_type, entity_type, entity_id, status, payload, retry_count, created_at, read_at ` + base
	listQuery, listArgs := common.BuildQuery(selectBase, filter, allowedColumns, nil, false)
	listQuery = r.db.Rebind(listQuery)

	var items []Notification
	if err := r.db.SelectContext(ctx, &items, listQuery, listArgs...); err != nil {
		return nil, 0, 0, fmt.Errorf("list notifications: %w", err)
	}

	return items, unread, total, nil
}

func (r *repository) MarkRead(ctx context.Context, recipientID, notificationID uuid.UUID) error {
	const q = `
		UPDATE tbl_notification
		SET status = 'READ', read_at = NOW()
		WHERE id = $1 AND recipient_id = $2
	`
	res, err := r.db.ExecContext(ctx, q, notificationID, recipientID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("notification not found")
	}
	return nil
}

func (r *repository) MarkDismissed(ctx context.Context, recipientID, notificationID uuid.UUID) error {
	const q = `
		UPDATE tbl_notification
		SET status = 'DISMISSED'
		WHERE id = $1 AND recipient_id = $2
	`
	res, err := r.db.ExecContext(ctx, q, notificationID, recipientID)
	if err != nil {
		return fmt.Errorf("mark dismissed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("notification not found")
	}
	return nil
}

func (r *repository) GetUserIDByEmail(ctx context.Context, email string) (*uuid.UUID, error) {
	var userID uuid.UUID
	const q = `SELECT id FROM tbl_user WHERE email = $1 LIMIT 1`
	err := r.db.QueryRowxContext(ctx, q, email).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user id by email: %w", err)
	}
	return &userID, nil
}

func (r *repository) GetUserIDByPractitionerID(ctx context.Context, practitionerID uuid.UUID) (*uuid.UUID, error) {
	var userID uuid.UUID
	const q = `
		SELECT u.id FROM tbl_practitioner p
		JOIN tbl_user u ON u.id = p.user_id
		WHERE p.id = $1 LIMIT 1
	`
	err := r.db.QueryRowxContext(ctx, q, practitionerID).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user id by practitioner id: %w", err)
	}
	return &userID, nil
}

func (r *repository) GetPractitionerUserIDByClinicID(ctx context.Context, clinicID uuid.UUID) (*uuid.UUID, error) {
	var userID uuid.UUID
	const q = `
		SELECT u.id FROM tbl_clinic c
		JOIN tbl_practitioner p ON p.id = c.practitioner_id
		JOIN tbl_user u ON u.id = p.user_id
		WHERE c.id = $1 AND c.deleted_at IS NULL AND p.deleted_at IS NULL LIMIT 1
	`
	err := r.db.QueryRowxContext(ctx, q, clinicID).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get practitioner user id by clinic id: %w", err)
	}
	return &userID, nil
}

func (r *repository) GetPractitionerUserIDByFormID(ctx context.Context, formID uuid.UUID) (*uuid.UUID, error) {
	var userID uuid.UUID
	const q = `
		SELECT u.id FROM tbl_form f
		JOIN tbl_clinic c ON c.id = f.clinic_id
		JOIN tbl_practitioner p ON p.id = c.practitioner_id
		JOIN tbl_user u ON u.id = p.user_id
		WHERE f.id = $1 AND c.deleted_at IS NULL AND p.deleted_at IS NULL LIMIT 1
	`
	err := r.db.QueryRowxContext(ctx, q, formID).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get practitioner user id by form id: %w", err)
	}
	return &userID, nil
}

func (r *repository) GetPractitionerUserIDByEntryID(ctx context.Context, entryID uuid.UUID) (*uuid.UUID, error) {
	var userID uuid.UUID
	const q = `
		SELECT u.id FROM tbl_form_entry e
		JOIN tbl_clinic c ON c.id = e.clinic_id
		JOIN tbl_practitioner p ON p.id = c.practitioner_id
		JOIN tbl_user u ON u.id = p.user_id
		WHERE e.id = $1 AND e.deleted_at IS NULL AND c.deleted_at IS NULL AND p.deleted_at IS NULL LIMIT 1
	`
	err := r.db.QueryRowxContext(ctx, q, entryID).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get practitioner user id by entry id: %w", err)
	}
	return &userID, nil
}
