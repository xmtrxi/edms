package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createDocumentExtractionsTable struct{}

func (m createDocumentExtractionsTable) Name() string { return "008_create_document_extractions_table" }

func (m createDocumentExtractionsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "document_extractions", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id").Unique()
		t.String("text")
		t.String("language").Nullable()
		t.Int("pages").Default(0)
		t.Object("metadata").Nullable()
		t.Timestamps()
	})
}

func (m createDocumentExtractionsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "document_extractions")
}
