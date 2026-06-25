package preference

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]Preference, error)
	CreatePreference(ctx context.Context, pref Preference, tx *sqlx.Tx) error
	GetAllPreferencesByEntityID(ctx context.Context, entityID uuid.UUID) ([]Preference, error)
	GetPreferencesByUserID(ctx context.Context, userID uuid.UUID) (Preference, error)
	DeleteAllPreferences(ctx context.Context, userID, entityID uuid.UUID, tx *sqlx.Tx) error
	DeletePreferenceByEventType(ctx context.Context, userID, entityID uuid.UUID, eventType util.NotificationEventType, tx *sqlx.Tx) error
}

type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
}

func scanPreference(rows *sqlx.Rows) (Preference, error) {
	var p Preference
	var channelsRaw []byte
	var eventTypeValue string

	err := rows.Scan(
		&p.ID,
		&p.UserID,
		&p.EntityID,
		&p.EntityType,
		&eventTypeValue,
		&channelsRaw,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return Preference{}, err
	}

	p.EventType = []util.NotificationEventType{util.NotificationEventType(eventTypeValue)}

	if len(channelsRaw) > 0 {
		if err := json.Unmarshal(channelsRaw, &p.Channels); err != nil {
			return Preference{}, err
		}
	} else {
		p.Channels = make([]util.Channel, 0)
	}
	return p, nil
}

func (r *repository) GetAllPreferences(ctx context.Context, userID uuid.UUID) ([]Preference, error) {
	prefs := make([]Preference, 0)

	const q = `
		SELECT 
			id,
			user_id,
			entity_id,
			entity_type,
			event_type,
			channels,
			created_at,
			updated_at
		FROM tbl_notification_preferences
		WHERE user_id = $1
		AND deleted_at IS NULL
	`

	rows, err := r.db.QueryxContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefs, nil
}

func (r *repository) GetAllPreferencesByEntityID(ctx context.Context, entityID uuid.UUID) ([]Preference, error) {
	prefs := make([]Preference, 0)

	const q = `
		SELECT 
			id,
			user_id,
			entity_id,
			entity_type,
			event_type,
			channels,
			created_at,
			updated_at
		FROM tbl_notification_preferences
		WHERE entity_id = $1
		AND deleted_at IS NULL
	`

	rows, err := r.db.QueryxContext(ctx, q, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefs, nil
}

func (r *repository) CreatePreference(ctx context.Context, p Preference, tx *sqlx.Tx) error {
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

	if len(p.EventType) == 0 {
		return fmt.Errorf("event_type cannot be empty")
	}
	eventTypeValue := string(p.EventType[0])

	_, err = tx.ExecContext(ctx, q,
		p.UserID,
		p.EntityID,
		p.EntityType,
		eventTypeValue,
		channelsJSON,
	)
	if err != nil {
		return fmt.Errorf("exec create preference: %w", err)
	}

	return nil
}

func (r *repository) DeleteAllPreferences(ctx context.Context, userID, entityID uuid.UUID, tx *sqlx.Tx) error {
	const q = `
		UPDATE tbl_notification_preferences
		SET deleted_at = NOW()
		WHERE user_id = $1 AND entity_id = $2 AND deleted_at IS NULL
	`
	_, err := tx.ExecContext(ctx, q, userID, entityID)
	return err
}

func (r *repository) DeletePreferenceByEventType(ctx context.Context, userID, entityID uuid.UUID, eventType util.NotificationEventType, tx *sqlx.Tx) error {
	const q = `
		UPDATE tbl_notification_preferences
		SET deleted_at = NOW()
		WHERE user_id = $1 AND entity_id = $2 AND event_type = $3 AND deleted_at IS NULL
	`
	_, err := tx.ExecContext(ctx, q, userID, entityID, string(eventType))
	return err
}

func (r *repository) GetPreferencesByUserID(ctx context.Context, userID uuid.UUID) (Preference, error) {
	const q = `
		SELECT 
			id,
			user_id,
			entity_id,
			entity_type,
			event_type,
			channels,
			created_at,
			updated_at
		FROM tbl_notification_preferences
		WHERE user_id = $1 AND deleted_at IS NULL
	`

	rows, err := r.db.QueryxContext(ctx, q, userID)
	if err != nil {
		return Preference{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Preference{}, ErrPreferenceNotFound
	}

	return scanPreference(rows)
}
