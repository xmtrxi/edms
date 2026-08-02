package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createDocumentsTable struct{}

func (m createDocumentsTable) Name() string { return "002_create_documents_table" }

func (m createDocumentsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "documents", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("owner_id")
		t.String("name")
		t.String("original_name")
		t.String("mime_type")
		t.Int("size")
		t.String("storage_provider")
		t.String("storage_path")
		t.String("checksum")
		t.String("status").Default(database.DocumentStatusUploaded)
		t.Object("metadata").Nullable()
		t.Timestamps()
	})
}

func (m createDocumentsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "documents")
}
