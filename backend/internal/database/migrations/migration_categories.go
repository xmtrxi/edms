package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createCategoriesTable struct{}

func (m createCategoriesTable) Name() string { return "014_create_categories_table" }

func (m createCategoriesTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "categories", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("owner_id")
		t.String("name")
		t.String("description").Nullable()
		t.String("parent_id").Nullable()
		t.String("color").Nullable()
		t.Timestamps()
	})
}

func (m createCategoriesTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "categories")
}
