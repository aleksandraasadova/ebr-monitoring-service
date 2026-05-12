package wsserver

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Hub maintains active WebSocket connections and broadcasts messages to all.
type Hub struct {
	mu        sync.RWMutex
	clients   map[*wsClient]bool
	broadcast chan []byte
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*wsClient]bool),
		broadcast: make(chan []byte, 256),
	}
}

// Run starts the hub event loop. Call in a goroutine.
func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		for c := range h.clients {
			select {
			case c.send <- msg:
			default:
				// slow client — drop message
			}
		}
		h.mu.RUnlock()
	}
}

// Broadcast sends data to all connected clients.
func (h *Hub) Broadcast(data []byte) {
	select {
	case h.broadcast <- data:
	default:
		slog.Warn("ws hub broadcast channel full, dropping message")
	}
}

// ServeWS upgrades HTTP to WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}

	c := &wsClient{conn: conn, send: make(chan []byte, 64)}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	// Writer goroutine
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			conn.Close()
		}()
		for msg := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Reader goroutine (consumes pings/control frames, detects close)
	go func() {
		defer close(c.send)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
