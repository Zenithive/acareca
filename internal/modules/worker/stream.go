package worker

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
)

type StreamManager struct {
	events    sharedEvents.IEvent
	mu        sync.RWMutex
	streams   map[uuid.UUID]*UserStream
	consumers *Consumer
}

type UserStream struct {
	userID   uuid.UUID
	ctx      context.Context
	cancel   context.CancelFunc
	handler  func(notification.NotificationEvent)
	active   bool
	msgCount int32
}

func NewStreamManager(events sharedEvents.IEvent, consumer *Consumer) *StreamManager {
	return &StreamManager{
		events:    events,
		streams:   make(map[uuid.UUID]*UserStream, 100),
		consumers: consumer,
	}
}

func (sm *StreamManager) AttachUserStream(userID uuid.UUID, handler func(interface{})) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, ok := sm.streams[userID]; ok {
		if existing.active {
			log.Printf("User %s already has an active stream, cleaning up old stream", userID)
			existing.cancel()
			existing.active = false
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	wrappedHandler := func(event notification.NotificationEvent) {
		handler(event)
	}

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

func (sm *StreamManager) DetachUserStream(userID uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stream, ok := sm.streams[userID]
	if !ok {
		return
	}

	stream.active = false
	stream.cancel()
	delete(sm.streams, userID)

	msgCount := atomic.LoadInt32(&stream.msgCount)
	log.Printf("Detached notification stream for user %s (processed %d messages)", userID, msgCount)
}

func (sm *StreamManager) DeliverToUser(userID uuid.UUID, event notification.NotificationEvent) bool {
	sm.mu.RLock()
	stream, ok := sm.streams[userID]
	sm.mu.RUnlock()

	if !ok || !stream.active {
		return false
	}

	stream.handler(event)
	atomic.AddInt32(&stream.msgCount, 1)

	return true
}

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

func (sm *StreamManager) IsUserStreamActive(userID uuid.UUID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stream, ok := sm.streams[userID]
	return ok && stream.active
}

func (sm *StreamManager) GetUserStreamInfo(userID uuid.UUID) map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stream, ok := sm.streams[userID]
	if !ok {
		return nil
	}

	msgCount := atomic.LoadInt32(&stream.msgCount)
	return map[string]interface{}{
		"user_id":       stream.userID,
		"active":        stream.active,
		"message_count": msgCount,
	}
}
