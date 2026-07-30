package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createTagsTable struct{}

func (m createTagsTable) Name() string { return "006_create_tags_table" }

func (m createTagsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "tags", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("tenant_id")
		t.String("name")
		t.String("color").Nullable()
		t.Timestamps()
	})
}

func (m createTagsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "tags")
}
