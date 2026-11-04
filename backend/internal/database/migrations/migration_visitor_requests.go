package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addVisitorRequests struct{}

func (m addVisitorRequests) Name() string { return "021_add_visitor_requests" }

func (m addVisitorRequests) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "visitor_requests", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("template_id")
		t.String("department_id")
		t.String("visitor_name")
		t.String("visitor_email")
		t.String("access_code_hash")
		t.String("otp_hash").Nullable()
		t.Datetime("otp_expires_at").Nullable()
		t.Int("otp_attempts").Default(0)
		t.String("session_token_hash").Nullable()
		t.Datetime("session_expires_at").Nullable()
		t.String("submission_id").Nullable()
		t.String("status")
		t.Datetime("expires_at")
		t.String("created_by")
		t.Timestamps()
	})
}

func (m addVisitorRequests) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "visitor_requests")
}
