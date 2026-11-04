package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createDocumentVersionsTable struct{}

func (m createDocumentVersionsTable) Name() string { return "003_create_document_versions_table" }

func (m createDocumentVersionsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "document_versions", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.Int("version")
		t.String("storage_provider")
		t.String("storage_path")
		t.String("checksum")
		t.Int("size")
		t.String("created_by")
		t.Datetime("created_at")
	})
}

func (m createDocumentVersionsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "document_versions")
}
