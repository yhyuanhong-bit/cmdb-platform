package websocket

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true // dev mode — allow all origins
	},
}

// HandleWS returns a gin handler that upgrades HTTP connections to WebSocket
// and registers clients with the hub. It reads tenant_id and user_id from the
// gin context (typically set by auth middleware).
func HandleWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		tenantID, _ := c.Get("tenant_id")
		userID, _ := c.Get("user_id")

		client := &Client{
			TenantID: asString(tenantID),
			UserID:   asString(userID),
			Conn:     conn,
			Send:     make(chan []byte, 256),
		}

		hub.Register(client)

		go writePump(client)
		go readPump(hub, client)
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// readPump reads messages from the WebSocket connection. It keeps the
// connection alive by handling pong messages and enforces read deadlines.
func readPump(hub *Hub, client *Client) {
	defer func() {
		hub.Unregister(client)
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(maxMessageSize)
	_ = client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	client.Conn.SetPongHandler(func(string) error {
		return client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
		// We don't process inbound messages — the WS is server-push only.
	}
}

// writePump pumps messages from the Send channel to the WebSocket connection
// and sends periodic pings to keep the connection alive.
func writePump(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			_ = client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				_ = client.Conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			if err := client.Conn.WriteMessage(gorilla.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.Conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
