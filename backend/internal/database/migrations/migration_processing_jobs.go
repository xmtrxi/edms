package migrations

import (
	"context"
	"fmt"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
)

type createProcessingJobsTable struct{}

func (m createProcessingJobsTable) Name() string { return "010_create_processing_jobs_table" }

func (m createProcessingJobsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := createTable(ctx, schema, "processing_jobs", func(t *surrealgoorm.Table) {
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
	}); err != nil {
		return err
	}
	// ORM's Nullable() emits `TYPE object` (+FLEXIBLE) not `option<object>`.
	// `option<object>` is `object | NONE` so a stored `NULL` still fails with
	// `Expected none | object but found NULL`. Allow both legacy NULL and NONE.
	if _, err := surrealdb.Query[any](ctx, schema.Db, "DEFINE FIELD OVERWRITE result ON processing_jobs TYPE option<object> | NULL FLEXIBLE;", nil); err != nil {
		return fmt.Errorf("overwrite processing_jobs.result: %w", err)
	}
	if _, err := surrealdb.Query[any](ctx, schema.Db, "DEFINE FIELD OVERWRITE last_error ON processing_jobs TYPE option<string> | NULL;", nil); err != nil {
		return fmt.Errorf("overwrite processing_jobs.last_error: %w", err)
	}
	if _, err := surrealdb.Query[any](ctx, schema.Db, "DEFINE FIELD OVERWRITE external_id ON processing_jobs TYPE option<string> | NULL;", nil); err != nil {
		return fmt.Errorf("overwrite processing_jobs.external_id: %w", err)
	}
	if _, err := surrealdb.Query[any](ctx, schema.Db, "DEFINE FIELD OVERWRITE completed_at ON processing_jobs TYPE option<datetime> | NULL;", nil); err != nil {
		return fmt.Errorf("overwrite processing_jobs.completed_at: %w", err)
	}
	return nil
}

func (m createProcessingJobsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "processing_jobs")
}
