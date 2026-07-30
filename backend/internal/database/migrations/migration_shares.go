package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createSharesTable struct{}

func (m createSharesTable) Name() string { return "010_create_shares_table" }

func (m createSharesTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "shares", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("created_by")
		t.String("token_hash").Unique()
		t.Datetime("expires_at")
		t.Datetime("revoked_at").Nullable()
		t.Datetime("created_at")
	})
}

func (m createSharesTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "shares")
}
