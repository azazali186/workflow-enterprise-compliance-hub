// Package ws implements the real-time WebSocket hub using coder/websocket.
// Clients connect to GET /ws and receive JSON event envelopes for the four
// README event types (plus module lifecycle events).
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
)

// Hub manages all live WebSocket connections and fans out bus events.
type Hub struct {
	mu           sync.RWMutex
	clients      map[*client]struct{}
	topics       map[string]map[*client]struct{}
	max          int
	pingInterval time.Duration
}

// NewHub creates a hub with a connection cap and a ping interval used to keep
// connections alive and reap dead peers (defaults to 30s when <= 0).
func NewHub(maxConnections int, pingInterval time.Duration) *Hub {
	if pingInterval <= 0 {
		pingInterval = 30 * time.Second
	}
	return &Hub{
		clients:      make(map[*client]struct{}),
		topics:       make(map[string]map[*client]struct{}),
		max:          maxConnections,
		pingInterval: pingInterval,
	}
}

// client is a single connected WebSocket peer.
type client struct {
	conn   *websocket.Conn
	send   chan []byte
	topics map[string]struct{}
	hub    *Hub
}

// ServeWS serves an established WebSocket connection. The optional `topic`
// query parameter pre-subscribes the client to one or more event types (comma
// separated); by default the client receives everything.
func (h *Hub) ServeWS(ctx context.Context, conn *websocket.Conn, topics []string) error {
	h.mu.Lock()
	if len(h.clients) >= h.max {
		h.mu.Unlock()
		return errMaxConnections
	}
	c := &client{conn: conn, send: make(chan []byte, 64), topics: make(map[string]struct{}), hub: h}
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	if len(topics) == 0 {
		// No explicit topics => receive all event types.
		h.subscribe(c, "*")
	} else {
		for _, t := range topics {
			if t != "" {
				h.subscribe(c, t)
			}
		}
	}

	slog.Info("websocket client connected", "topics", topics)
	defer h.remove(c)

	readerDone := make(chan struct{})
	writerDone := make(chan struct{})
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	// Writer goroutine: sends queued messages and periodic pings. Ping and
	// Write are sequenced on this single goroutine, and Read runs on the main
	// one, so there is never more than one concurrent writer. When a ping
	// fails the connection is dead — the reader is aborted so the client gets
	// reaped instead of accumulating forever.
	go func() {
		defer close(writerDone)
		ticker := time.NewTicker(h.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case msg := <-c.send:
				_ = conn.Write(ctx, websocket.MessageText, msg)
			case <-ticker.C:
				pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
				if err := conn.Ping(pingCtx); err != nil {
					cancelPing()
					cancelRead() // abort the reader -> connection is reaped
					return
				}
				cancelPing()
			case <-readerDone:
				return
			}
		}
	}()

	// Reader loop: consumes (and mostly ignores) inbound messages. JSON
	// messages of the form {"op":"subscribe","topics":[...]} can change the
	// client's subscription at runtime.
	for {
		_, data, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var req subscribeRequest
		if json.Unmarshal(data, &req) == nil && req.Op == "subscribe" {
			h.setTopics(c, req.Topics)
		}
	}

	close(readerDone)
	<-writerDone
	return nil
}

type subscribeRequest struct {
	Op     string   `json:"op"`
	Topics []string `json:"topics"`
}

var errMaxConnections = &connLimitError{}

type connLimitError struct{}

func (e *connLimitError) Error() string { return "max websocket connections reached" }

func (h *Hub) subscribe(c *client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.topics[topic] = struct{}{}
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*client]struct{})
	}
	h.topics[topic][c] = struct{}{}
}

func (h *Hub) setTopics(c *client, topics []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for t := range c.topics {
		delete(h.topics[t], c)
	}
	c.topics = make(map[string]struct{})
	for _, t := range topics {
		if t == "" {
			continue
		}
		c.topics[t] = struct{}{}
		if h.topics[t] == nil {
			h.topics[t] = make(map[*client]struct{})
		}
		h.topics[t][c] = struct{}{}
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	for t := range c.topics {
		delete(h.topics[t], c)
	}
}

// Broadcast delivers an event to every subscribed client. A client subscribed
// to "*" receives all events; otherwise the event type must be in its topics.
func (h *Hub) Broadcast(e bus.Event) {
	msg, err := json.Marshal(e)
	if err != nil {
		return
	}

	h.mu.RLock()
	targets := make(map[*client]struct{})
	if h.topics["*"] != nil {
		for c := range h.topics["*"] {
			targets[c] = struct{}{}
		}
	}
	if h.topics[e.Type] != nil {
		for c := range h.topics[e.Type] {
			targets[c] = struct{}{}
		}
	}
	h.mu.RUnlock()

	for c := range targets {
		select {
		case c.send <- msg:
		default:
			// Slow consumer: drop the message rather than blocking the hub.
		}
	}
}
