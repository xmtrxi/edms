package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createAuditEventsTable struct{}

func (m createAuditEventsTable) Name() string { return "011_create_audit_events_table" }

func (m createAuditEventsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "audit_events", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("actor_id")
		t.String("action")
		t.String("resource")
		t.String("resource_id")
		t.Object("metadata").Nullable()
		t.Datetime("created_at")
	})
}

func (m createAuditEventsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "audit_events")
}
