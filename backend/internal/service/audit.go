package service

import (
	"context"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
)

func writeAudit(ctx context.Context, db *surrealdb.DB, actorID, action, resource, resourceID string, metadata map[string]any) error {
	_, err := surrealdb.Create[database.AuditEvent](ctx, db, "audit_events", &database.AuditEvent{
		OrgID: orgOf(ctx), ActorID: actorID, Action: action, Resource: resource, ResourceID: resourceID,
		Metadata: metadata, CreatedAt: time.Now().UTC(),
	})
	return err
}

// WriteAudit records an event for handlers that act outside a service.
func WriteAudit(ctx context.Context, db *surrealdb.DB, actorID, action, resource, resourceID string, metadata map[string]any) error {
	return writeAudit(ctx, db, actorID, action, resource, resourceID, metadata)
}
