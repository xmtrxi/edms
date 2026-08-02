package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createCommentsTable struct{}

func (m createCommentsTable) Name() string { return "007_create_comments_table" }

func (m createCommentsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "comments", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("author_id")
		t.String("body")
		t.Timestamps()
	})
}

func (m createCommentsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "comments")
}
