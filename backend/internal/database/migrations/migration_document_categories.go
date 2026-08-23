package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createDocumentCategoriesTable struct{}

func (m createDocumentCategoriesTable) Name() string {
	return "015_create_document_categories_table"
}

func (m createDocumentCategoriesTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "document_categories", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("category_id")
		t.Datetime("created_at")
	})
}

func (m createDocumentCategoriesTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "document_categories")
}
