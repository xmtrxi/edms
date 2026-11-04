package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addPDFJobs struct{}

func (m addPDFJobs) Name() string { return "036_add_pdf_jobs" }

// Up adds queued PDF tool runs.
func (m addPDFJobs) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE TABLE pdf_jobs SCHEMAFULL;
DEFINE FIELD org_id ON pdf_jobs TYPE string;
DEFINE FIELD user_id ON pdf_jobs TYPE string;
DEFINE FIELD operation ON pdf_jobs TYPE string;
DEFINE FIELD document_ids ON pdf_jobs TYPE array<string>;
DEFINE FIELD options ON pdf_jobs TYPE option<object> FLEXIBLE;
DEFINE FIELD folder_id ON pdf_jobs TYPE option<string>;
DEFINE FIELD output_name ON pdf_jobs TYPE option<string>;
DEFINE FIELD status ON pdf_jobs TYPE string;
DEFINE FIELD attempts ON pdf_jobs TYPE int;
DEFINE FIELD error ON pdf_jobs TYPE option<string>;
DEFINE FIELD result_document_id ON pdf_jobs TYPE option<string>;
DEFINE FIELD result_version_id ON pdf_jobs TYPE option<string>;
DEFINE FIELD idempotency_key ON pdf_jobs TYPE option<string>;
DEFINE FIELD created_at ON pdf_jobs TYPE datetime;
DEFINE FIELD updated_at ON pdf_jobs TYPE datetime;
DEFINE FIELD completed_at ON pdf_jobs TYPE option<datetime>;
DEFINE INDEX pdf_jobs_user ON pdf_jobs FIELDS user_id, created_at;
DEFINE INDEX pdf_jobs_idempotency ON pdf_jobs FIELDS user_id, idempotency_key;
`)
}

func (m addPDFJobs) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "pdf_jobs")
}
