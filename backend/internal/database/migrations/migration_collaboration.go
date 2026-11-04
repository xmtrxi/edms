package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addCollaboration struct{}

func (m addCollaboration) Name() string { return "031_add_collaboration" }

// Up turns comments into threads that can be resolved, anchored to a page
// and mention people.
func (m addCollaboration) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db,
		"DEFINE FIELD IF NOT EXISTS org_id ON comments TYPE option<string>;"+
			"DEFINE FIELD IF NOT EXISTS parent_id ON comments TYPE option<string>;"+
			"DEFINE FIELD IF NOT EXISTS page ON comments TYPE option<int>;"+
			"DEFINE FIELD IF NOT EXISTS quote ON comments TYPE option<string>;"+
			"DEFINE FIELD IF NOT EXISTS mentions ON comments TYPE option<array<string>>;"+
			"DEFINE FIELD IF NOT EXISTS edited_at ON comments TYPE option<datetime>;"+
			"DEFINE FIELD IF NOT EXISTS resolved_at ON comments TYPE option<datetime>;"+
			"DEFINE FIELD IF NOT EXISTS resolved_by ON comments TYPE option<string>;"+
			"DEFINE INDEX IF NOT EXISTS comments_document ON comments FIELDS document_id, created_at;")
}

func (m addCollaboration) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return nil
}
