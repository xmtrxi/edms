package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createDocumentTagsTable struct{}

func (m createDocumentTagsTable) Name() string { return "005_create_document_tags_table" }

func (m createDocumentTagsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "document_tags", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("tag_id")
		t.String("created_by")
		t.Datetime("created_at")
	})
}

func (m createDocumentTagsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "document_tags")
}
