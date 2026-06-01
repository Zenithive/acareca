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
	GetAllPreferencesByentityID(ctx context.Context, entityId uuid.UUID) ([]Preference, error)
}
type repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &repository{db: db}
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
		var p Preference

		var channelsRaw []byte

		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.EntityID,
			&p.EntityType,
			&p.EventType,
			&channelsRaw,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		//  convert JSONB → map
		if len(channelsRaw) > 0 {
			if err := json.Unmarshal(channelsRaw, &p.Channels); err != nil {
				return nil, err
			}
		} else {
			p.Channels = make([]util.Channel, 0)
		}

		prefs = append(prefs, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefs, nil
}
func (r *repository) GetAllPreferencesByentityID(ctx context.Context, entityId uuid.UUID) ([]Preference, error) {

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

	rows, err := r.db.QueryxContext(ctx, q, entityId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p Preference

		var channelsRaw []byte

		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.EntityID,
			&p.EntityType,
			&p.EventType,
			&channelsRaw,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		//  convert JSONB → map
		if len(channelsRaw) > 0 {
			if err := json.Unmarshal(channelsRaw, &p.Channels); err != nil {
				return nil, err
			}
		} else {
			p.Channels = make([]util.Channel, 0)
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

	_, err = tx.ExecContext(
		ctx,
		q,
		p.UserID,
		p.EntityID,
		p.EntityType,
		p.EventType,
		channelsJSON,
	)
	if err != nil {
		return fmt.Errorf("exec create preference: %w", err)
	}

	return nil
}
