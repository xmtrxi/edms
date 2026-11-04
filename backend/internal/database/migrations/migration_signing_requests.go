package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createSigningRequestsTable struct{}

func (m createSigningRequestsTable) Name() string { return "009_create_signing_requests_table" }

func (m createSigningRequestsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "signing_requests", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("requested_by")
		t.String("provider")
		t.String("provider_document_id").Nullable()
		t.String("status").Default(database.SigningStatusDraft)
		t.Timestamps()
	})
}

func (m createSigningRequestsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "signing_requests")
}
