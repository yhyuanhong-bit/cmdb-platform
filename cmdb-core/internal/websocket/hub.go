// Package websocket provides a tenant-aware WebSocket hub for real-time event
// push to connected browser clients.
package websocket

import (
	"context"
	"encoding/json"
	"sync"

	gorilla "github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection scoped to a tenant.
type Client struct {
	TenantID string
	UserID   string
	Conn     *gorilla.Conn
	Send     chan []byte
}

// BroadcastMessage is a tenant-scoped message pushed to all matching clients.
type BroadcastMessage struct {
	TenantID string          `json:"tenant_id"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}

// Hub maintains the set of active clients and broadcasts messages to them,
// filtering by tenant ID.
type Hub struct {
	clients    map[*Client]struct{}
	broadcast  chan BroadcastMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates and returns a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub event loop. It blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.Send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				close(client.Send)
				delete(h.clients, client)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			h.mu.RLock()
			for c := range h.clients {
				if c.TenantID != msg.TenantID {
					continue
				}
				select {
				case c.Send <- data:
				default:
					// Slow client — schedule removal.
					go h.Unregister(c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all clients of the specified tenant.
func (h *Hub) Broadcast(msg BroadcastMessage) {
	h.broadcast <- msg
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// ClientCount returns the current number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
