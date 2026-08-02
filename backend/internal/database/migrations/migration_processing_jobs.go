package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createProcessingJobsTable struct{}

func (m createProcessingJobsTable) Name() string { return "010_create_processing_jobs_table" }

func (m createProcessingJobsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "processing_jobs", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("type")
		t.String("status").Default(database.ProcessingJobStatusQueued)
		t.Int("attempts").Default(0)
		t.String("last_error").Nullable()
		t.String("external_id").Nullable()
		t.Object("result").Nullable()
		t.Datetime("completed_at").Nullable()
		t.Timestamps()
	})
}

func (m createProcessingJobsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "processing_jobs")
}
