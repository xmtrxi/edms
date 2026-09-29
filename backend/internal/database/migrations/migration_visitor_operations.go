package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addVisitorOperations struct{}

func (m addVisitorOperations) Name() string { return "022_add_visitor_operations" }

func (m addVisitorOperations) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := execDDL(ctx, schema.Db, "DEFINE FIELD otp_request_count ON visitor_requests TYPE int DEFAULT 0; DEFINE FIELD otp_last_sent_at ON visitor_requests TYPE option<datetime>;"); err != nil {
		return err
	}
	return createTable(ctx, schema, "visitor_attachments", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("visitor_request_id")
		t.String("submission_id").Nullable()
		t.String("name")
		t.String("mime_type")
		t.Int("size")
		t.String("storage_path")
		t.Datetime("created_at")
	})
}

func (m addVisitorOperations) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "visitor_attachments")
}
