package notification

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
)

// StreamManager manages per-user notification streams for real-time WebSocket delivery
type StreamManager struct {
	events    sharedEvents.IEvent
	mu        sync.RWMutex
	streams   map[uuid.UUID]*UserStream // userID -> UserStream
	consumers *Consumer                 // Reference to main consumer for filtering
}

// UserStream represents a notification stream for a specific user
type UserStream struct {
	userID   uuid.UUID
	ctx      context.Context
	cancel   context.CancelFunc
	handler  func(NotificationEvent)
	active   bool
	msgCount int
}

func NewStreamManager(events sharedEvents.IEvent, consumer *Consumer) *StreamManager {
	return &StreamManager{
		events:    events,
		streams:   make(map[uuid.UUID]*UserStream),
		consumers: consumer,
	}
}

// AttachUserStream registers a handler for a user's notifications
// When notifications arrive via NATS, they'll be forwarded to this handler
func (sm *StreamManager) AttachUserStream(userID uuid.UUID, handler func(interface{})) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if user already has an active stream
	if existing, ok := sm.streams[userID]; ok {
		if existing.active {
			log.Printf("User %s already has an active stream", userID)
			return nil
		}
	}

	// Create a cancellable context for this user's stream
	ctx, cancel := context.WithCancel(context.Background())

	// Wrap the handler to convert NotificationEvent to interface{}
	wrappedHandler := func(event NotificationEvent) {
		handler(event)
	}

	// Create user stream
	stream := &UserStream{
		userID:  userID,
		ctx:     ctx,
		cancel:  cancel,
		handler: wrappedHandler,
		active:  true,
	}

	sm.streams[userID] = stream

	log.Printf("Attached notification stream for user %s", userID)
	return nil
}

// DetachUserStream stops the notification stream for a user when they disconnect
func (sm *StreamManager) DetachUserStream(userID uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stream, ok := sm.streams[userID]
	if !ok {
		return
	}

	// Mark as inactive
	stream.active = false

	// Cancel the context
	stream.cancel()

	// Remove from map
	delete(sm.streams, userID)

	log.Printf("Detached notification stream for user %s (processed %d messages)", userID, stream.msgCount)
}

// DeliverToUser delivers a notification to a specific user if they have an active stream
// This is called by the consumer when a notification arrives
func (sm *StreamManager) DeliverToUser(userID uuid.UUID, event NotificationEvent) bool {
	sm.mu.RLock()
	stream, ok := sm.streams[userID]
	sm.mu.RUnlock()

	if !ok || !stream.active {
		return false
	}

	// Deliver to user's handler (WebSocket)
	stream.handler(event)
	stream.msgCount++

	return true
}

// GetActiveStreams returns the number of active user streams
func (sm *StreamManager) GetActiveStreams() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, stream := range sm.streams {
		if stream.active {
			count++
		}
	}
	return count
}

// IsUserStreamActive checks if a user has an active stream
func (sm *StreamManager) IsUserStreamActive(userID uuid.UUID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stream, ok := sm.streams[userID]
	return ok && stream.active
}

// GetUserStreamInfo returns information about a user's stream
func (sm *StreamManager) GetUserStreamInfo(userID uuid.UUID) map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stream, ok := sm.streams[userID]
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"user_id":       stream.userID,
		"active":        stream.active,
		"message_count": stream.msgCount,
	}
}
