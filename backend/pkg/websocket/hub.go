package websocket

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

type Connection struct {
	userID int64
	session string
	conn    *websocket.Conn
	send    chan interface{}
	closeOnce sync.Once
}

type Hub struct {
	db    *sql.DB
	conns map[int64]map[*Connection]struct{}
	mu    sync.RWMutex

	inboundMu sync.RWMutex
	inbound   map[string]InboundHandler
}

type InboundHandler func(senderID int64, frame []byte)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The ONLY origin defence here: browsers apply neither the same-origin policy nor a CORS preflight to a WebSocket handshake, and attach the session cookie anyway.
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == middleware.AllowedOrigin
	},
}

func NewHub(db *sql.DB) *Hub {
	return &Hub{
		db:      db,
		conns:   make(map[int64]map[*Connection]struct{}),
		inbound: make(map[string]InboundHandler),
	}
}

func (h *Hub) HandleInbound(msgType string, fn InboundHandler) {
	h.inboundMu.Lock()
	defer h.inboundMu.Unlock()
	h.inbound[msgType] = fn
}

func (h *Hub) dispatch(senderID int64, frame []byte) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(frame, &envelope); err != nil {
		return
	}

	h.inboundMu.RLock()
	fn := h.inbound[envelope.Type]
	h.inboundMu.RUnlock()

	if fn != nil {
		fn(senderID, frame)
	}
}

func RegisterRoutes(mux *http.ServeMux, hub *Hub) {
	mux.HandleFunc("GET /ws", hub.ServeWS)
}

func (h *Hub) register(c *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[c.userID]; !ok {
		h.conns[c.userID] = make(map[*Connection]struct{})
	}
	h.conns[c.userID][c] = struct{}{}
}

// Must run exactly once per connection -- closing c.send twice panics -- and readPump, CloseSession and ServeWS can each reach it concurrently.
func (h *Hub) unregister(c *Connection) {
	c.closeOnce.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if set, ok := h.conns[c.userID]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.conns, c.userID)
			}
		}
		close(c.send)
		c.conn.Close()
	})
}

const sessionEndedTimeout = time.Second

// RFC 6455 caps a close frame's payload at 125 bytes and the code takes the first two, so a longer reason makes WriteControl fail.
const maxCloseReason = 123

const closeReasonSessionInvalid = "session invalid"

func (h *Hub) CloseSession(token string, reason string) {
	h.mu.Lock()
	var ended []*Connection
	for userID, set := range h.conns {
		for c := range set {
			if c.session == token {
				delete(set, c)
				ended = append(ended, c)
			}
		}
		if len(set) == 0 {
			delete(h.conns, userID)
		}
	}
	h.mu.Unlock()

	for _, c := range ended {
		h.endSession(c, reason)
	}
}

// The code stays 1008 whatever the reason: a client keys "do not reconnect" off the code and only then reads the text.
func (h *Hub) endSession(c *Connection, reason string) {
	if len(reason) > maxCloseReason {
		reason = reason[:maxCloseReason]
	}

	c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason),
		time.Now().Add(sessionEndedTimeout),
	)
	h.unregister(c)
}

// Deliberately does NOT unregister on a write error: readPump's defer owns teardown, and closing the socket here unblocks its read.
func (h *Hub) writePump(c *Connection) {
	for msg := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteJSON(msg); err != nil {
			c.conn.Close()
			return
		}
	}
}

// Each message is read in full: NextReader resets the length it checks against the read limit, so a large message sent in fragments would never trip it.
func (h *Hub) readPump(c *Connection) {
	defer h.unregister(c)
	c.conn.SetReadLimit(1024 * 16)
	for {
		_, r, err := c.conn.NextReader()
		if err != nil {
			return
		}
		frame, err := io.ReadAll(r)
		if err != nil {
			return
		}
		h.dispatch(c.userID, frame)
	}
}

func (h *Hub) Push(userID int64, payload interface{}) {
	h.PushToUser(userID, payload)
}

func (h *Hub) PushToUser(userID int64, msg interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if conns, ok := h.conns[userID]; ok {
		for conn := range conns {
			select {
			case conn.send <- msg:
			default:
			}
		}
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "not logged in")
		return
	}

	userID, err := middleware.ValidateSession(r.Context(), h.db, cookie.Value)
	if errors.Is(err, middleware.ErrInvalidSession) {
		response.Error(w, http.StatusUnauthorized, "session expired or invalid")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to validate session")
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	conn := &Connection{
		userID:  userID,
		session: cookie.Value,
		conn:    ws,
		send:    make(chan interface{}, 32),
	}

	h.register(conn)

	// Re-checked now the connection is registered: a handshake that validated just before a logout deleted the session would otherwise stay open on a dead one.
	if _, err := middleware.ValidateSession(r.Context(), h.db, cookie.Value); err != nil {
		if errors.Is(err, middleware.ErrInvalidSession) {
			h.endSession(conn, closeReasonSessionInvalid)
		} else {
			h.unregister(conn)
		}
		return
	}

	go h.writePump(conn)
	h.readPump(conn)
}
