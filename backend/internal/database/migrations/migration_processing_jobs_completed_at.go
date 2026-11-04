package migrations

import (
	"context"
	"fmt"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

// makeProcessingJobCompletionOptional fixes the original processing_jobs
// definition. A queued or running job has no completion timestamp, which
// SurrealDB represents as NONE. The original strict datetime field rejected
// that value and prevented the entire job record from being inserted.
type makeProcessingJobCompletionOptional struct{}

func (m makeProcessingJobCompletionOptional) Name() string {
	return "020_make_processing_job_completion_optional"
}

func (m makeProcessingJobCompletionOptional) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	_, err := surrealdb.Query[any](ctx, schema.Db,
		"DEFINE FIELD OVERWRITE completed_at ON processing_jobs TYPE option<datetime>;", nil)
	if err != nil {
		return fmt.Errorf("make processing_jobs.completed_at optional: %w", err)
	}
	return nil
}

func (m makeProcessingJobCompletionOptional) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	_, err := surrealdb.Query[any](ctx, schema.Db,
		"DEFINE FIELD OVERWRITE completed_at ON processing_jobs TYPE datetime;", nil)
	if err != nil {
		return fmt.Errorf("restore processing_jobs.completed_at: %w", err)
	}
	return nil
}
