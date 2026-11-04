package service

import (
	"context"

	"github.com/xmtrxi/edms/internal/realtime"
)

// hub delivers live events; nil when realtime is off, which makes publish a
// no-op. It is set once at startup.
var hub *realtime.Hub

// UseRealtime sets the hub services publish live events to.
func UseRealtime(h *realtime.Hub) { hub = h }

// publish sends an event about a document to everyone who has it open.
func publishDocument(ctx context.Context, documentID, eventType string, data any) {
	if hub == nil || documentID == "" {
		return
	}
	event := realtime.Event{Type: eventType, Topic: "document:" + documentID, Data: data}
	if actor, ok := ActorFrom(ctx); ok && actor.UserID != "" {
		event.Actor = &realtime.Person{ID: actor.UserID, Email: actor.Email}
	}
	hub.Publish(context.WithoutCancel(ctx), event)
}

// publishUser sends an event to one person's open tabs.
func publishUser(ctx context.Context, userID, eventType string, data any) {
	if hub == nil || userID == "" {
		return
	}
	hub.Publish(context.WithoutCancel(ctx), realtime.Event{Type: eventType, Topic: "user:" + userID, Data: data})
}
