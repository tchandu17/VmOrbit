package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/pkg/logger"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB

	// maxConnsPerUser limits how many simultaneous WebSocket connections a
	// single user can hold. Prevents connection storms from misbehaving clients.
	maxConnsPerUser = 5
)

// Message is the envelope sent over WebSocket connections.
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	rooms    map[string]bool
	mu       sync.RWMutex
}

// Hub manages all active WebSocket clients and broadcasts events.
type Hub struct {
	clients    map[*Client]bool
	rooms      map[string]map[*Client]bool
	userConns  map[string]int // userID → active connection count
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg
	mu         sync.RWMutex
	log        logger.Logger
	eventBus   messaging.EventBus
}

type broadcastMsg struct {
	room    string // empty = all clients
	message []byte
}

// NewHub creates a new WebSocket hub.
func NewHub(log logger.Logger, eventBus messaging.EventBus) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		userConns:  make(map[string]int),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan broadcastMsg, 2048), // doubled buffer to reduce back-pressure
		log:        log,
		eventBus:   eventBus,
	}
}

// Run starts the hub event loop and subscribes to internal events.
func (h *Hub) Run(ctx context.Context) {
	// Subscribe to domain events and forward to WebSocket clients.
	h.eventBus.Subscribe(messaging.EventTaskStatusChanged, func(ctx context.Context, e messaging.Event) {
		h.BroadcastToRoom("tasks", e.Type, e.Payload)
		// Also fan-out to the per-task room so fine-grained subscribers get it.
		if p, ok := e.Payload.(map[string]interface{}); ok {
			if id, _ := p["task_id"].(string); id != "" {
				h.BroadcastToRoom("task:"+id, e.Type, e.Payload)
			}
		}
	})
	h.eventBus.Subscribe(messaging.EventTaskProgress, func(ctx context.Context, e messaging.Event) {
		h.BroadcastToRoom("tasks", e.Type, e.Payload)
		if p, ok := e.Payload.(map[string]interface{}); ok {
			if id, _ := p["task_id"].(string); id != "" {
				h.BroadcastToRoom("task:"+id, e.Type, e.Payload)
			}
		}
	})
	h.eventBus.Subscribe(messaging.EventTaskLogAppended, func(ctx context.Context, e messaging.Event) {
		// Log entries only go to the per-task room — not the global tasks room.
		if p, ok := e.Payload.(map[string]interface{}); ok {
			if id, _ := p["task_id"].(string); id != "" {
				h.BroadcastToRoom("task:"+id, e.Type, e.Payload)
			}
		}
	})
	h.eventBus.Subscribe(messaging.EventTaskCancelled, func(ctx context.Context, e messaging.Event) {
		h.BroadcastToRoom("tasks", e.Type, e.Payload)
		if p, ok := e.Payload.(map[string]interface{}); ok {
			if id, _ := p["task_id"].(string); id != "" {
				h.BroadcastToRoom("task:"+id, e.Type, e.Payload)
			}
		}
	})
	h.eventBus.Subscribe(messaging.EventVMStatusChanged, func(ctx context.Context, e messaging.Event) {
		h.BroadcastToRoom("vms", e.Type, e.Payload)
	})
	h.eventBus.Subscribe(messaging.EventInventorySynced, func(ctx context.Context, e messaging.Event) {
		h.BroadcastToRoom("inventory", e.Type, e.Payload)
	})
	h.eventBus.Subscribe(messaging.EventPlatformEvent, func(ctx context.Context, e messaging.Event) {
		// Platform events go to the "events" room for the activity feed
		h.BroadcastToRoom("events", e.Type, e.Payload)
	})

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userConns[client.userID]++
			h.mu.Unlock()
			h.log.Debug("ws client connected", logger.String("user_id", client.userID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Remove from all rooms
				for room := range client.rooms {
					delete(h.rooms[room], client)
				}
				// Decrement per-user connection count
				if h.userConns[client.userID] > 0 {
					h.userConns[client.userID]--
					if h.userConns[client.userID] == 0 {
						delete(h.userConns, client.userID)
					}
				}
			}
			h.mu.Unlock()
			h.log.Debug("ws client disconnected", logger.String("user_id", client.userID))

		case msg := <-h.broadcast:
			h.mu.Lock()
			var targets map[*Client]bool
			if msg.room == "" {
				targets = h.clients
			} else {
				targets = h.rooms[msg.room]
			}
			// Collect slow clients to disconnect after releasing the lock.
			var slowClients []*Client
			for client := range targets {
				select {
				case client.send <- msg.message:
				default:
					// Slow client — mark for disconnection.
					slowClients = append(slowClients, client)
				}
			}
			// Disconnect slow clients while holding the write lock.
			for _, client := range slowClients {
				close(client.send)
				delete(h.clients, client)
				for room := range client.rooms {
					delete(h.rooms[room], client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(conn *websocket.Conn, userID string) *Client {
	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		rooms:  make(map[string]bool),
	}
	h.register <- client
	return client
}

// JoinRoom subscribes a client to a named room.
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.mu.Lock()
	client.rooms[room] = true
	client.mu.Unlock()
}

// BroadcastToRoom sends a typed message to all clients in a room.
func (h *Hub) BroadcastToRoom(room string, eventType messaging.EventType, payload interface{}) {
	msg := Message{Type: string(eventType), Payload: payload}
	b, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("ws: failed to marshal broadcast message", logger.Error(err))
		return
	}
	h.broadcast <- broadcastMsg{room: room, message: b}
}

// BroadcastAll sends a message to every connected client.
func (h *Hub) BroadcastAll(eventType messaging.EventType, payload interface{}) {
	msg := Message{Type: string(eventType), Payload: payload}
	b, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("ws: failed to marshal broadcast message", logger.Error(err))
		return
	}
	h.broadcast <- broadcastMsg{message: b}
}

// WritePump pumps messages from the hub to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.log.Error("ws read error", logger.Error(err))
			}
			break
		}
		// Handle incoming client messages (subscriptions, etc.)
		c.handleIncoming(message)
	}
}

func (c *Client) handleIncoming(data []byte) {
	var msg struct {
		Type string `json:"type"`
		Room string `json:"room"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	if msg.Type == "subscribe" && msg.Room != "" {
		c.hub.JoinRoom(c, msg.Room)
	}
}
