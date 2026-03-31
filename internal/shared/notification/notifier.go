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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

// Hub manages all active WebSocket connections keyed by entity (practitioner/accountant) ID.
type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID][]*client
	db      *sqlx.DB
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

// Push sends a live notification event to all connections belonging to entityID.
// Returns true if at least one client received the message.
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

	log.Printf("notifier: Push to entityID=%s — %d active connection(s)", entityID, len(conns))

	if len(conns) == 0 {
		log.Printf("notifier: no active WS clients for entityID=%s, notification saved to DB only", entityID)
		return false
	}

	delivered := false
	for _, c := range conns {
		select {
		case c.send <- data:
			log.Printf("notifier: queued message to entityID=%s", entityID)
			delivered = true
		default:
			log.Printf("notifier: dropped message for entityID=%s (slow client)", entityID)
		}
	}
	return delivered
}

// ServeWS upgrades the HTTP connection to WebSocket, authenticates via ?token=,
// sends stored notifications, then streams live pushes.
func (h *Hub) ServeWS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		entityID, ok := h.authenticate(c, cfg)
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
		defer h.unregister(cl)

		// send stored notifications immediately
		if err := h.sendStored(c.Request.Context(), cl); err != nil {
			log.Printf("notifier: sendStored error: %v", err)
		}

		go cl.writePump()
		cl.readPump() // blocks until disconnect
	}
}

// ── auth ─────────────────────────────────────────────────────────────────────

func (h *Hub) authenticate(c *gin.Context, cfg *config.Config) (uuid.UUID, bool) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return uuid.Nil, false
	}

	claims := &util.CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid || claims.ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return uuid.Nil, false
	}

	entityID, err := uuid.Parse(claims.ID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid entity id"})
		return uuid.Nil, false
	}
	return entityID, true
}

// ── registry ─────────────────────────────────────────────────────────────────

func (h *Hub) register(cl *client) {
	h.mu.Lock()
	h.clients[cl.entityID] = append(h.clients[cl.entityID], cl)
	count := len(h.clients[cl.entityID])
	h.mu.Unlock()
	log.Printf("notifier: client registered entityID=%s, total=%d", cl.entityID, count)
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
	log.Printf("notifier: client unregistered entityID=%s, remaining=%d", cl.entityID, len(h.clients[cl.entityID]))
	close(cl.send)
	_ = cl.conn.Close()
}

// ── stored notifications ──────────────────────────────────────────────────────

type storedNotification struct {
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

func (h *Hub) sendStored(ctx context.Context, cl *client) error {
	// Only fetch notifications where the in_app delivery was DELIVERED
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

	var notifications []storedNotification
	for rows.Next() {
		var n storedNotification
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
	cl.send <- data
	return nil
}

// ── pumps ─────────────────────────────────────────────────────────────────────

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
