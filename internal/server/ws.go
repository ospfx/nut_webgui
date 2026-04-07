package server

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/ospfx/nut_webgui/internal/poller"
	"net/http"
)

// wsHub manages WebSocket client connections and broadcasts events.
type wsHub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	ch      chan poller.Event
}

func newWSHub() *wsHub {
	return &wsHub{
		clients: make(map[*websocket.Conn]struct{}),
		ch:      make(chan poller.Event, 256),
	}
}

func (h *wsHub) run() {
	for ev := range h.ch {
		h.mu.Lock()
		msg, _ := json.Marshal(ev)
		for conn := range h.clients {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				conn.Close()
				delete(h.clients, conn)
			}
		}
		h.mu.Unlock()
	}
}

func (h *wsHub) broadcast(ev poller.Event) {
	select {
	case h.ch <- ev:
	default:
	}
}

func (h *wsHub) register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *wsHub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

// handleWS upgrades HTTP to WebSocket and registers the client with the hub.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}
	s.hub.register(conn)
	defer func() {
		s.hub.unregister(conn)
		conn.Close()
	}()

	// Read loop – discard client messages, keep connection alive
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
