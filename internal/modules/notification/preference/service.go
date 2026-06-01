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

type IService interface {
	Get(ctx context.Context, userID uuid.UUID) ([]Preference, error)
	Update(ctx context.Context, userID, entityID uuid.UUID, role string, rq RqUpdatePreference) error
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

	if len(rq.Channels) == 0 {
		return errors.New("at least one notification channel must be enabled")
	}

	err := util.RunInTransaction(ctx, s.DB, func(ctx context.Context, tx *sqlx.Tx) error {
		if len(rq.EventType) == 0 {
			return errors.New("event type is required")
		}

		if len(rq.Channels) == 0 {
			return errors.New("at least one channel must be selected")
		}
		pref := Preference{
			UserID:     userID,
			EntityID:   entityID,
			EntityType: role,
			EventType:  rq.EventType,
			Channels:   rq.Channels,
		}

		if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
			return fmt.Errorf("failed to create preference: %w", err)
		}

		return nil
	})

	return err
}

func (s *service) PreferenceSetting(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, entityID uuid.UUID, entityType string) error {
	existingPrefs, err := s.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get existing preferences: %w", err)
	}

	for _, et := range existingPrefs {
		pref := Preference{
			UserID:     userID,
			EntityID:   entityID,
			EntityType: entityType,
			EventType:  et.EventType,
			Channels:   et.Channels,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.CreatePreference(ctx, pref, tx); err != nil {
			return fmt.Errorf("failed to create preference: %w", err)
		}
	}

	return nil
}
