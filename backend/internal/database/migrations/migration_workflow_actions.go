package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type createWorkflowActionsTable struct{}

func (m createWorkflowActionsTable) Name() string { return "015_create_workflow_actions_table" }

func (m createWorkflowActionsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "workflow_actions", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("workflow_request_id")
		t.String("actor_id")
		t.String("action")
		t.String("note").Nullable()
		t.Datetime("created_at")
	})
}

func (m createWorkflowActionsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "workflow_actions")
}
