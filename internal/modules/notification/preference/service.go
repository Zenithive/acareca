package preference

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

var ErrPreferenceNotFound = errors.New("preference not found")

type IService interface {
	Get(ctx context.Context, userID uuid.UUID) ([]Preference, error)
	Update(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error
	DeleteAll(ctx context.Context, userID, entityID uuid.UUID) error
	PreferenceSetting(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, entityID uuid.UUID, entityType string) error
}

type service struct {
	repo Repository
	DB   *sqlx.DB
}

func NewService(repo Repository, db *sqlx.DB) IService {
	return &service{
		repo: repo,
		DB:   db,
	}
}

func (s *service) Get(ctx context.Context, userID uuid.UUID) ([]Preference, error) {
	prefs, err := s.repo.GetAllPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	if prefs == nil {
		return []Preference{}, nil
	}
	return prefs, nil
}

func (s *service) Update(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error {
	return util.RunInTransaction(ctx, s.DB, func(ctx context.Context, tx *sqlx.Tx) error {

		if err := s.repo.DeleteAllPreferences(ctx, userID, entityID, tx); err != nil {
			return fmt.Errorf("failed to delete all preferences: %w", err)
		}

		if len(rq.Channels) == 0 {
			fmt.Println("2")
			for _, eventType := range rq.EventType {
				if err := s.repo.DeletePreferenceByEventType(ctx, userID, entityID, eventType, tx); err != nil {
					return fmt.Errorf("failed to delete preference for event type %s: %w", eventType, err)
				}
			}
			return nil
		}

		for _, eventType := range rq.EventType {
			pref := Preference{
				UserID:     userID,
				EntityID:   entityID,
				EntityType: role,
				EventType:  []util.NotificationEventType{eventType},
				Channels:   rq.Channels,
			}
			if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
				return fmt.Errorf("failed to create/update preference for event type %s: %w", eventType, err)
			}
		}

		return nil
	})
}

func (s *service) DeleteAll(ctx context.Context, userID, entityID uuid.UUID) error {
	return util.RunInTransaction(ctx, s.DB, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := s.repo.DeleteAllPreferences(ctx, userID, entityID, tx); err != nil {
			return fmt.Errorf("failed to delete all preferences: %w", err)
		}
		return nil
	})
}

func (s *service) createPreferences(ctx context.Context, tx *sqlx.Tx, userID, entityID uuid.UUID, entityType string, eventTypes []util.NotificationEventType, channels []util.Channel) error {
	for _, eventType := range eventTypes {
		pref := Preference{
			UserID:     userID,
			EntityID:   entityID,
			EntityType: entityType,
			EventType:  []util.NotificationEventType{eventType},
			Channels:   channels,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
			return fmt.Errorf("failed to create preference for %s: %w", eventType, err)
		}
	}
	return nil
}

func getDefaultEventTypes(entityType string) []util.NotificationEventType {
	if entityType == util.RoleAdmin {
		return []util.NotificationEventType{
			util.EventSystemErrorAlert,
			util.EventSystemWarningAlert,
			util.EventSystemActivityAlert,
			util.EventBillingAlert,
			util.EventSubscriptionAlert,
			util.EventUserRegistrationAlert,
		}
	}
	
	// Both Practitioner and Accountant get these default preferences
	if entityType == util.RolePractitioner {
		return []util.NotificationEventType{
			util.EventNewTransaction,
			util.EventAccountantActivityAlert,
			util.EventSystemActivityAlert,
		}
	}
	
	// Accountant
	if entityType == util.RoleAccountant {
		return []util.NotificationEventType{
			util.EventNewTransaction,
			util.EventPractitionerActivityAlert,
			util.EventSystemActivityAlert,
		}
	}
	
	// Default fallback
	return []util.NotificationEventType{
		util.EventNewTransaction,
		util.EventAccountantActivityAlert,
		util.EventSystemActivityAlert,
	}
}

func (s *service) PreferenceSetting(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, entityID uuid.UUID, entityType string) error {
	if err := s.repo.DeleteAllPreferences(ctx, userID, entityID, tx); err != nil {
		return fmt.Errorf("failed to clear existing preferences: %w", err)
	}

	const q = `
		SELECT 
			id, user_id, entity_id, entity_type,
			event_type, channels, created_at, updated_at
		FROM tbl_notification_preferences
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	rows, err := tx.QueryxContext(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("failed to get existing preferences: %w", err)
	}
	defer rows.Close()

	existingPrefs := make([]Preference, 0)
	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return fmt.Errorf("failed to scan preference: %w", err)
		}
		existingPrefs = append(existingPrefs, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(existingPrefs) > 0 {
		for _, existingPref := range existingPrefs {
			if err := s.createPreferences(ctx, tx, userID, entityID, entityType, existingPref.EventType, existingPref.Channels); err != nil {
				return err
			}
		}
		return nil
	}

	defaultEventTypes := getDefaultEventTypes(entityType)
	defaultChannels := []util.Channel{util.ChannelInApp}
	return s.createPreferences(ctx, tx, userID, entityID, entityType, defaultEventTypes, defaultChannels)
}
