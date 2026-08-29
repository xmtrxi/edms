package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addAccessAndForms struct{}

func (m addAccessAndForms) Name() string { return "019_add_casbin_access_and_forms" }

func (m addAccessAndForms) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	tables := map[string]func(*surrealgoorm.Table){
		"roles": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("key").Unique()
			t.String("name")
			t.String("description").Nullable()
			t.Bool("protected").Default(false)
			t.Timestamps()
		},
		"casbin_rules": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("ptype")
			t.String("v0").Nullable()
			t.String("v1").Nullable()
			t.String("v2").Nullable()
			t.String("v3").Nullable()
			t.String("v4").Nullable()
			t.String("v5").Nullable()
			t.Datetime("created_at")
		},
		"invitations": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("email")
			t.String("name")
			t.String("token_hash")
			t.String("status")
			t.Datetime("expires_at")
			t.Datetime("accepted_at").Nullable()
			t.String("created_by")
			t.Datetime("created_at")
		},
		"form_templates": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("department_id")
			t.String("name")
			t.String("description").Nullable()
			t.String("status")
			t.Int("version")
			t.Object("definition")
			t.String("created_by")
			t.Timestamps()
		},
		"form_submissions": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("template_id")
			t.Int("template_version")
			t.String("department_id")
			t.String("submitted_by")
			t.String("status")
			t.Object("values")
			t.Object("definition")
			t.Timestamps()
		},
		"form_comments": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("submission_id")
			t.String("author_id")
			t.String("body")
			t.Datetime("created_at")
		},
		"form_submission_documents": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("submission_id")
			t.String("document_id")
			t.Datetime("created_at")
		},
		"form_approval_steps": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("submission_id")
			t.Int("sequence")
			t.String("name")
			t.String("completion_rule")
			t.String("status")
			t.Datetime("created_at")
			t.Datetime("completed_at").Nullable()
		},
		"form_approval_assignees": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("step_id")
			t.String("user_id").Nullable()
			t.String("department_id").Nullable()
			t.String("status")
			t.Datetime("decided_at").Nullable()
		},
	}
	for name, definition := range tables {
		if err := createTable(ctx, schema, name, definition); err != nil {
			return err
		}
	}
	return nil
}

func (m addAccessAndForms) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"form_approval_assignees", "form_approval_steps", "form_submission_documents", "form_comments", "form_submissions", "form_templates", "invitations", "casbin_rules", "roles"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
