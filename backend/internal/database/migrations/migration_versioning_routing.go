package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

type expandVersioningAndRouting struct{}

func (m expandVersioningAndRouting) Name() string { return "016_expand_versioning_and_routing" }

func (m expandVersioningAndRouting) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	// Existing tables are SCHEMAFULL, so newly persisted fields must be declared.
	statements := []string{
		"DEFINE FIELD current_version_id ON documents TYPE option<string>;",
		"DEFINE FIELD version_count ON documents TYPE int DEFAULT 0;",
		"DEFINE FIELD parent_version_id ON document_versions TYPE option<string>;",
		"DEFINE FIELD file_name ON document_versions TYPE string;",
		"DEFINE FIELD mime_type ON document_versions TYPE string;",
		"DEFINE FIELD source ON document_versions TYPE string;",
		"DEFINE FIELD change_summary ON document_versions TYPE string;",
		"DEFINE FIELD change_manifest ON document_versions TYPE option<object> FLEXIBLE;",
		"DEFINE FIELD workflow_id ON document_versions TYPE option<string>;",
		"DEFINE FIELD workflow_step_id ON document_versions TYPE option<string>;",
		"DEFINE FIELD signing_request_id ON document_versions TYPE option<string>;",
		"DEFINE FIELD title ON workflow_requests TYPE string;",
		"DEFINE FIELD started_version_id ON workflow_requests TYPE string;",
		"DEFINE FIELD current_version_id ON workflow_requests TYPE string;",
		"DEFINE FIELD priority ON workflow_requests TYPE string;",
		"DEFINE FIELD current_step ON workflow_requests TYPE int DEFAULT 0;",
		"DEFINE FIELD returned_version_id ON workflow_requests TYPE option<string>;",
		"DEFINE FIELD due_at ON workflow_requests TYPE option<datetime>;",
		"DEFINE FIELD completed_at ON workflow_requests TYPE option<datetime>;",
		"DEFINE FIELD step_id ON workflow_actions TYPE option<string>;",
		"DEFINE FIELD document_version_id ON workflow_actions TYPE option<string>;",
		"DEFINE FIELD return_to_step ON workflow_actions TYPE option<int>;",
	}
	for _, statement := range statements {
		if _, err := surrealdb.Query[any](ctx, schema.Db, statement, nil); err != nil {
			return err
		}
	}
	if err := createTable(ctx, schema, "document_version_contributors", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("version_id")
		t.String("user_id")
		t.String("role")
		t.Datetime("created_at")
	}); err != nil {
		return err
	}
	if err := createTable(ctx, schema, "departments", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("name")
		t.String("description").Nullable()
		t.String("created_by")
		t.Timestamps()
	}); err != nil {
		return err
	}
	if err := createTable(ctx, schema, "department_members", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("department_id")
		t.String("user_id")
		t.String("role")
		t.Datetime("created_at")
	}); err != nil {
		return err
	}
	return createTable(ctx, schema, "workflow_steps", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("workflow_request_id")
		t.Int("sequence")
		t.String("name")
		t.String("action_type")
		t.String("assignment_type")
		t.String("assigned_user_id").Nullable()
		t.String("assigned_department_id").Nullable()
		t.String("external_email").Nullable()
		t.String("status")
		t.Bool("require_comment")
		t.Bool("require_new_version")
		t.Datetime("activated_at").Nullable()
		t.Datetime("completed_at").Nullable()
		t.Datetime("created_at")
	})
}

func (m expandVersioningAndRouting) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, table := range []string{"workflow_steps", "department_members", "departments", "document_version_contributors"} {
		if err := removeTable(ctx, schema, table); err != nil {
			return err
		}
	}
	return nil
}
