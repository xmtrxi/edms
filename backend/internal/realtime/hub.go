// Package realtime pushes live events to browsers over WebSocket. Events are
// published to topics such as "document:<id>" or "user:<id>". With Valkey
// configured, every API instance relays events through one pub/sub channel,
// so a browser connected to any instance receives them.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"
)

const channel = "edms:realtime"

// Person identifies who caused an event or who is present.
type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Event is one message delivered to subscribers of Topic.
type Event struct {
	Type  string  `json:"type"`
	Topic string  `json:"topic"`
	Data  any     `json:"data,omitempty"`
	Actor *Person `json:"actor,omitempty"`
	At    int64   `json:"at"`
}

// Presence event types. Browsers keep a set of people per topic from these.
const (
	PresenceJoin  = "presence.join"
	PresenceLeave = "presence.leave"
	PresenceHere  = "presence.here"
)

// Client is one connected browser tab.
type Client struct {
	Person Person
	send   chan Event
	topics map[string]bool
}

// Hub fans events out to subscribed clients.
type Hub struct {
	mu     sync.RWMutex
	topics map[string]map[*Client]struct{}
	valkey valkey.Client
	log    *slog.Logger
}

// New creates a hub. client may be nil for a single-instance deployment.
func New(client valkey.Client, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{topics: map[string]map[*Client]struct{}{}, valkey: client, log: logger}
}

// Run relays events from other instances until ctx ends. It returns at once
// without Valkey.
func (h *Hub) Run(ctx context.Context) {
	if h == nil || h.valkey == nil {
		return
	}
	for ctx.Err() == nil {
		err := h.valkey.Receive(ctx, h.valkey.B().Subscribe().Channel(channel).Build(), func(msg valkey.PubSubMessage) {
			var event Event
			if json.Unmarshal([]byte(msg.Message), &event) == nil {
				h.deliver(event)
			}
		})
		if ctx.Err() != nil {
			return
		}
		h.log.Warn("realtime subscription dropped; retrying", "error", err)
		time.Sleep(2 * time.Second)
	}
}

// Publish sends an event to everyone subscribed to its topic.
func (h *Hub) Publish(ctx context.Context, event Event) {
	if h == nil || event.Topic == "" {
		return
	}
	if event.At == 0 {
		event.At = time.Now().UnixMilli()
	}
	if h.valkey != nil {
		payload, err := json.Marshal(event)
		if err == nil {
			err = h.valkey.Do(ctx, h.valkey.B().Publish().Channel(channel).Message(string(payload)).Build()).Error()
		}
		if err == nil {
			return
		}
		h.log.Warn("publish realtime event; delivering locally", "error", err)
	}
	h.deliver(event)
}

func (h *Hub) deliver(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.topics[event.Topic] {
		// A client too slow to keep up misses events rather than stalling
		// everyone; it resynchronises when it reloads.
		if event.Type == PresenceHere && event.Actor != nil && event.Actor.ID == client.Person.ID {
			continue
		}
		select {
		case client.send <- event:
		default:
		}
	}
	// Tell a newcomer who is already here.
	if event.Type == PresenceJoin {
		seen := map[string]bool{}
		if event.Actor != nil {
			seen[event.Actor.ID] = true
		}
		for client := range h.topics[event.Topic] {
			if seen[client.Person.ID] {
				continue
			}
			seen[client.Person.ID] = true
			person := client.Person
			go h.Publish(context.Background(), Event{Type: PresenceHere, Topic: event.Topic, Actor: &person})
		}
	}
}

// Connect registers a client; call Disconnect when it goes away.
func (h *Hub) Connect(person Person) *Client {
	return &Client{Person: person, send: make(chan Event, 64), topics: map[string]bool{}}
}

// Events is the stream of events for the client.
func (c *Client) Events() <-chan Event { return c.send }

// Subscribe adds the client to a topic and announces them there.
func (h *Hub) Subscribe(ctx context.Context, client *Client, topic string) {
	h.mu.Lock()
	if client.topics[topic] {
		h.mu.Unlock()
		return
	}
	client.topics[topic] = true
	if h.topics[topic] == nil {
		h.topics[topic] = map[*Client]struct{}{}
	}
	h.topics[topic][client] = struct{}{}
	h.mu.Unlock()
	person := client.Person
	h.Publish(ctx, Event{Type: PresenceJoin, Topic: topic, Actor: &person})
}

// Unsubscribe removes the client from a topic.
func (h *Hub) Unsubscribe(ctx context.Context, client *Client, topic string) {
	h.mu.Lock()
	if !client.topics[topic] {
		h.mu.Unlock()
		return
	}
	delete(client.topics, topic)
	delete(h.topics[topic], client)
	if len(h.topics[topic]) == 0 {
		delete(h.topics, topic)
	}
	// Someone with the page open in another tab is still here.
	for other := range h.topics[topic] {
		if other.Person.ID == client.Person.ID {
			h.mu.Unlock()
			return
		}
	}
	h.mu.Unlock()
	person := client.Person
	h.Publish(ctx, Event{Type: PresenceLeave, Topic: topic, Actor: &person})
}

// Disconnect removes the client from every topic.
func (h *Hub) Disconnect(ctx context.Context, client *Client) {
	h.mu.RLock()
	topics := make([]string, 0, len(client.topics))
	for topic := range client.topics {
		topics = append(topics, topic)
	}
	h.mu.RUnlock()
	for _, topic := range topics {
		h.Unsubscribe(ctx, client, topic)
	}
}
