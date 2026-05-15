package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

type Hub struct {
	mu            sync.RWMutex
	clients       map[uuid.UUID][]*client
	db            *sqlx.DB
	streamMu      sync.RWMutex // Separate mutex for stream manager
	streamManager interface {
		AttachUserStream(userID uuid.UUID, handler func(interface{})) error
		DetachUserStream(userID uuid.UUID)
	}
}

type client struct {
	conn     *websocket.Conn
	entityID uuid.UUID
	send     chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func NewNotifier(db *sqlx.DB) *Hub {
	return &Hub{
		clients: make(map[uuid.UUID][]*client),
		db:      db,
	}
}

// SetStreamManager sets the stream manager for real-time NATS integration
func (h *Hub) SetStreamManager(sm interface {
	AttachUserStream(userID uuid.UUID, handler func(interface{})) error
	DetachUserStream(userID uuid.UUID)
}) {
	h.streamMu.Lock()
	defer h.streamMu.Unlock()
	h.streamManager = sm
}

// Push sends a live notification event to all connections belonging to entityID.
func (h *Hub) Push(entityID uuid.UUID, payload any) bool {
	msg := map[string]any{
		"type": "notification",
		"data": payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("notifier: marshal error: %v", err)
		return false
	}

	h.mu.RLock()
	conns := h.clients[entityID]
	h.mu.RUnlock()

	if len(conns) == 0 {
		return false
	}

	delivered := false
	for _, c := range conns {
		select {
		case c.send <- data:
			delivered = true
		default:
			log.Printf("notifier: dropped message for entityID=%s (slow or closed client)", entityID)
		}
	}
	return delivered
}

func (h *Hub) ServeWS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		entityID, ok := util.GetEntityID(c)
		if !ok {
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("notifier: upgrade error: %v", err)
			return
		}

		cl := &client{conn: conn, entityID: entityID, send: make(chan []byte, 64)}
		h.register(cl)

		done := make(chan struct{})
		go func() {
			defer close(done)
			if err := h.StoredNotifications(c.Request.Context(), cl); err != nil {
				log.Printf("notifier: StoredNotifications error: %v", err)
			}
			cl.writePump()
		}()

		cl.readPump() // blocks until disconnect

		h.unregister(cl)
		<-done // wait for writePump to finish
	}
}

func (h *Hub) register(cl *client) {
	h.mu.Lock()
	h.clients[cl.entityID] = append(h.clients[cl.entityID], cl)
	h.mu.Unlock()

	// Attach NATS stream for real-time notifications with proper synchronization
	h.streamMu.RLock()
	sm := h.streamManager
	h.streamMu.RUnlock()

	if sm != nil {
		handler := func(event interface{}) {
			data, err := json.Marshal(map[string]any{
				"type": "notification",
				"data": event,
			})
			if err != nil {
				log.Printf("notifier: marshal error for stream event: %v", err)
				return
			}

			select {
			case cl.send <- data:
			default:
				log.Printf("notifier: dropped stream message for entityID=%s (buffer full)", cl.entityID)
			}
		}

		if err := sm.AttachUserStream(cl.entityID, handler); err != nil {
			log.Printf("notifier: failed to attach stream for user %s: %v", cl.entityID, err)
		} else {
			log.Printf("notifier: attached NATS stream for user %s", cl.entityID)
		}
	}
}

func (h *Hub) unregister(cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	list := h.clients[cl.entityID]
	for i, c := range list {
		if c == cl {
			h.clients[cl.entityID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	
	// Clean up map entry to prevent memory leak.
	if len(h.clients[cl.entityID]) == 0 {
		delete(h.clients, cl.entityID)
		
		// Detach NATS stream when last client disconnects with proper synchronization
		h.streamMu.RLock()
		sm := h.streamManager
		h.streamMu.RUnlock()

		if sm != nil {
			sm.DetachUserStream(cl.entityID)
			log.Printf("notifier: detached NATS stream for user %s", cl.entityID)
		}
	}
	
	// Closing the channel signals writePump to exit cleanly.
	close(cl.send)
}

type Notification struct {
	ID          uuid.UUID       `db:"id"`
	RecipientID uuid.UUID       `db:"recipient_id"`
	SenderID    *uuid.UUID      `db:"sender_id"`
	EventType   string          `db:"event_type"`
	EntityType  string          `db:"entity_type"`
	EntityID    uuid.UUID       `db:"entity_id"`
	Status      string          `db:"status"`
	Payload     json.RawMessage `db:"payload"`
	CreatedAt   time.Time       `db:"created_at"`
	ReadedAt    *time.Time      `db:"readed_at"`
}

func (h *Hub) StoredNotifications(ctx context.Context, cl *client) error {
	const q = `
		SELECT n.id, n.recipient_id, n.sender_id, n.event_type, n.entity_type, n.entity_id,
		       n.status, n.payload, n.created_at, n.read_at AS readed_at
		FROM tbl_notification n
		JOIN tbl_notification_delivery d
		  ON d.notification_id = n.id AND d.channel = 'in_app'
		WHERE n.recipient_id = $1
		  AND n.status != 'DISMISSED'
		  AND d.status = 'DELIVERED'
		ORDER BY n.created_at DESC
		LIMIT 50
	`
	rows, err := h.db.QueryxContext(ctx, q, cl.entityID)
	if err != nil {
		return fmt.Errorf("query stored notifications: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.StructScan(&n); err != nil {
			return fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate notifications: %w", err)
	}

	msg := map[string]any{
		"type": "initial",
		"data": notifications,
	}
	data, _ := json.Marshal(msg)

	select {
	case cl.send <- data:
	default:
		log.Printf("notifier: StoredNotifications dropped for entityID=%s (buffer full)", cl.entityID)
	}
	return nil
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

func (cl *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-cl.send:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = cl.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := cl.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (cl *client) readPump() {
	defer cl.conn.Close()
	_ = cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := cl.conn.ReadMessage(); err != nil {
			break
		}
	}
}
