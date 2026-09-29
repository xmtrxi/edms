package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addAIJobs struct{}

func (m addAIJobs) Name() string { return "024_add_ai_jobs" }
func (m addAIJobs) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "ai_jobs", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("user_id")
		t.String("kind")
		t.String("status")
		t.Array("document_ids")
		t.Object("input")
		t.Object("result").Nullable()
		t.String("error").Nullable()
		t.Datetime("created_at")
		t.Datetime("completed_at").Nullable()
	})
}
func (m addAIJobs) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "ai_jobs")
}
