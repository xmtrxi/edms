package realtime

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func newHub() *Hub {
	return New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func next(t *testing.T, c *Client) Event {
	t.Helper()
	select {
	case event := <-c.Events():
		return event
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: timed out waiting for an event", c.Person.ID)
		return Event{}
	}
}

func expectNone(t *testing.T, c *Client) {
	t.Helper()
	select {
	case event := <-c.Events():
		t.Fatalf("%s: unexpected event %+v", c.Person.ID, event)
	case <-time.After(50 * time.Millisecond):
	}
}

func expect(t *testing.T, c *Client, eventType, topic, actorID string) Event {
	t.Helper()
	event := next(t, c)
	if event.Type != eventType || event.Topic != topic || event.Actor == nil || event.Actor.ID != actorID {
		actor := "<nil>"
		if event.Actor != nil {
			actor = event.Actor.ID
		}
		t.Fatalf("%s: got %s/%s by %s, want %s/%s by %s", c.Person.ID, event.Type, event.Topic, actor, eventType, topic, actorID)
	}
	return event
}

func TestRunReturnsWithoutValkey(t *testing.T) {
	done := make(chan struct{})
	go func() {
		newHub().Run(context.Background())
		var nilHub *Hub
		nilHub.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run should return at once without Valkey")
	}
}

func TestNewDefaultsLogger(t *testing.T) {
	if New(nil, nil).log == nil {
		t.Fatal("expected default logger")
	}
}

func TestPublishDeliversToTopicSubscribersOnly(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	alice := hub.Connect(Person{ID: "user:alice", Name: "Alice"})
	bob := hub.Connect(Person{ID: "user:bob", Name: "Bob"})
	hub.Subscribe(ctx, alice, "document:1")
	expect(t, alice, PresenceJoin, "document:1", "user:alice")
	hub.Subscribe(ctx, bob, "document:2")
	expect(t, bob, PresenceJoin, "document:2", "user:bob")

	before := time.Now().UnixMilli()
	hub.Publish(ctx, Event{Type: "comment.created", Topic: "document:1", Data: map[string]any{"id": "c1"}})
	event := next(t, alice)
	if event.Type != "comment.created" || event.Data.(map[string]any)["id"] != "c1" {
		t.Fatalf("unexpected event %+v", event)
	}
	if event.At < before {
		t.Fatalf("At should be stamped when zero, got %d", event.At)
	}
	expectNone(t, bob)

	hub.Publish(ctx, Event{Type: "x", Topic: "document:1", At: 42})
	if event := next(t, alice); event.At != 42 {
		t.Fatalf("explicit At should be kept, got %d", event.At)
	}
}

func TestPublishIgnoresEmptyTopicAndNilHub(t *testing.T) {
	hub := newHub()
	alice := hub.Connect(Person{ID: "user:alice"})
	// The presence.join for an empty topic is dropped like any other event.
	hub.Subscribe(context.Background(), alice, "")
	hub.Publish(context.Background(), Event{Type: "x"})
	expectNone(t, alice)

	var nilHub *Hub
	nilHub.Publish(context.Background(), Event{Type: "x", Topic: "document:1"}) // must not panic
}

func TestPresenceJoinAndHere(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	const topic = "document:1"
	alice := hub.Connect(Person{ID: "user:alice", Name: "Alice"})
	bob := hub.Connect(Person{ID: "user:bob", Name: "Bob"})

	hub.Subscribe(ctx, alice, topic)
	expect(t, alice, PresenceJoin, topic, "user:alice")
	expectNone(t, alice) // nobody else is here

	hub.Subscribe(ctx, bob, topic)
	expect(t, alice, PresenceJoin, topic, "user:bob")
	expect(t, bob, PresenceJoin, topic, "user:bob")
	// The newcomer is told who is already here, and is never told about themselves.
	expect(t, bob, PresenceHere, topic, "user:alice")
	expectNone(t, bob)
	expectNone(t, alice)

	// Subscribing twice is a no-op.
	hub.Subscribe(ctx, bob, topic)
	expectNone(t, alice)
	expectNone(t, bob)
}

func TestPresenceLeave(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	const topic = "folder:1"
	alice := hub.Connect(Person{ID: "user:alice"})
	bob := hub.Connect(Person{ID: "user:bob"})
	hub.Subscribe(ctx, alice, topic)
	hub.Subscribe(ctx, bob, topic)
	drain(alice, bob)

	hub.Unsubscribe(ctx, bob, topic)
	expect(t, alice, PresenceLeave, topic, "user:bob")
	expectNone(t, bob) // no longer subscribed

	// Unsubscribing again, or from a topic never joined, is a no-op.
	hub.Unsubscribe(ctx, bob, topic)
	hub.Unsubscribe(ctx, bob, "folder:other")
	expectNone(t, alice)

	hub.Unsubscribe(ctx, alice, topic)
	hub.mu.RLock()
	_, stillThere := hub.topics[topic]
	hub.mu.RUnlock()
	if stillThere {
		t.Fatal("empty topics should be removed")
	}
}

func TestSecondTabKeepsPersonPresent(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	const topic = "document:1"
	bob := hub.Connect(Person{ID: "user:bob"})
	tab1 := hub.Connect(Person{ID: "user:alice"})
	tab2 := hub.Connect(Person{ID: "user:alice"})
	hub.Subscribe(ctx, bob, topic)
	hub.Subscribe(ctx, tab1, topic)
	hub.Subscribe(ctx, tab2, topic)
	time.Sleep(50 * time.Millisecond)
	drain(bob, tab1, tab2)

	// Closing one tab must not announce Alice as gone.
	hub.Unsubscribe(ctx, tab1, topic)
	expectNone(t, bob)

	hub.Unsubscribe(ctx, tab2, topic)
	expect(t, bob, PresenceLeave, topic, "user:alice")
}

func TestSecondTabJoinDoesNotSendHereAboutSelf(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	const topic = "document:1"
	tab1 := hub.Connect(Person{ID: "user:alice"})
	tab2 := hub.Connect(Person{ID: "user:alice"})
	hub.Subscribe(ctx, tab1, topic)
	drain(tab1)
	hub.Subscribe(ctx, tab2, topic)
	expect(t, tab1, PresenceJoin, topic, "user:alice")
	expect(t, tab2, PresenceJoin, topic, "user:alice")
	expectNone(t, tab2)
	expectNone(t, tab1)
}

func TestDisconnectLeavesEveryTopic(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	alice := hub.Connect(Person{ID: "user:alice"})
	bob := hub.Connect(Person{ID: "user:bob"})
	for _, topic := range []string{"document:1", "document:2"} {
		hub.Subscribe(ctx, alice, topic)
		hub.Subscribe(ctx, bob, topic)
	}
	time.Sleep(50 * time.Millisecond)
	drain(alice, bob)

	hub.Disconnect(ctx, bob)
	seen := map[string]bool{}
	for range 2 {
		event := next(t, alice)
		if event.Type != PresenceLeave || event.Actor == nil || event.Actor.ID != "user:bob" {
			t.Fatalf("unexpected event %+v", event)
		}
		seen[event.Topic] = true
	}
	if !seen["document:1"] || !seen["document:2"] {
		t.Fatalf("expected leave on both topics, got %v", seen)
	}
	if len(bob.topics) != 0 {
		t.Fatalf("client should have no topics after Disconnect, got %v", bob.topics)
	}
	hub.Publish(ctx, Event{Type: "x", Topic: "document:1"})
	next(t, alice)
	expectNone(t, bob)
}

func TestSlowClientDoesNotBlockOthers(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	const topic = "document:1"
	slow := hub.Connect(Person{ID: "user:slow"})
	fast := hub.Connect(Person{ID: "user:fast"})
	hub.Subscribe(ctx, slow, topic)
	hub.Subscribe(ctx, fast, topic)
	time.Sleep(50 * time.Millisecond)
	drain(slow, fast)

	const total = 200 // well over the 64-event buffer; nobody is reading
	done := make(chan struct{})
	go func() {
		for range total {
			hub.Publish(ctx, Event{Type: "tick", Topic: topic})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a full client buffer")
	}
	if got := len(slow.Events()); got != cap(slow.send) {
		t.Fatalf("slow client buffer should be full (%d), got %d", cap(slow.send), got)
	}
	// A client that catches up receives new events again.
	drain(fast)
	hub.Publish(ctx, Event{Type: "after", Topic: topic})
	if event := next(t, fast); event.Type != "after" {
		t.Fatalf("unexpected event %+v", event)
	}
}

func TestConcurrentSubscribePublish(t *testing.T) {
	hub := newHub()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := hub.Connect(Person{ID: string(rune('a' + i))})
			go func() {
				for range client.Events() {
				}
			}()
			for _, topic := range []string{"t1", "t2", "t3"} {
				hub.Subscribe(ctx, client, topic)
				hub.Publish(ctx, Event{Type: "x", Topic: topic})
			}
			hub.Disconnect(ctx, client)
		}()
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond) // let async presence.here publishes finish
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.topics) != 0 {
		t.Fatalf("all topics should be empty after every client disconnects, got %d", len(hub.topics))
	}
}

func drain(clients ...*Client) {
	for _, c := range clients {
		for {
			select {
			case <-c.Events():
				continue
			case <-time.After(20 * time.Millisecond):
			}
			break
		}
	}
}
