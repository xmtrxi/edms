package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createWorkflowRequestsTable struct{}

func (m createWorkflowRequestsTable) Name() string { return "014_create_workflow_requests_table" }

func (m createWorkflowRequestsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "workflow_requests", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("document_id")
		t.String("created_by")
		t.String("status").Default(database.WorkflowStatusPending)
		t.Timestamps()
	})
}

func (m createWorkflowRequestsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "workflow_requests")
}
