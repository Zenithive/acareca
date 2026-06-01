package notification

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *sqlx.DB {
	t.Helper()
	return testutil.OpenTestDB(t)
}

func TestConsumerPreferenceMatching(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	c := NewConsumer(nil, repo, nil, db, nil)

	ctx := context.Background()
	// create preference rows for a user
	userID := uuid.New()
	entityID := uuid.New()

	pref := NotificationPreference{
		ID:         uuid.New(),
		UserID:     userID,
		EntityID:   entityID,
		EntityType: string(EntitySystem),
		EventType:  EventSystemActivityAlert,
		Channels: NotificationChannels{
			string(ChannelInApp): true,
			string(ChannelEmail): false,
		},
	}

	require.NoError(t, repo.CreatePreference(ctx, pref))

	// shouldNotifyUser returns true only when matching preference event type
	should := c.shouldNotifyUser(ctx, userID, entityID, ActorSystem, EventSystemError)
	require.True(t, should)

	// getEnabledChannels should respect stored channels and requested channels
	allowed := c.getEnabledChannels(ctx, userID, entityID, ActorSystem, EventSystemError, []Channel{ChannelInApp, ChannelEmail})
	require.Len(t, allowed, 1)
	require.Equal(t, ChannelInApp, allowed[0])
}
