package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addWorkspaceOperations struct{}

func (m addWorkspaceOperations) Name() string { return "023_add_workspace_operations" }

func (m addWorkspaceOperations) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := execDDL(ctx, schema.Db, "DEFINE FIELD deleted_at ON documents TYPE option<datetime>; DEFINE FIELD deleted_from_status ON documents TYPE option<string>; DEFINE FIELD shared_with_user_id ON shares TYPE option<string>; DEFINE FIELD shared_with_email ON shares TYPE option<string>; DEFINE FIELD permission ON shares TYPE option<string>;"); err != nil {
		return err
	}
	tables := map[string]func(*surrealgoorm.Table){
		"document_favorites": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("document_id")
			t.String("user_id")
			t.Datetime("created_at")
		},
		"user_settings": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("user_id").Unique()
			t.Object("values")
			t.Datetime("updated_at")
		},
		"legal_holds": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("name")
			t.String("reason")
			t.Array("document_ids")
			t.String("created_by")
			t.Bool("active").Default(true)
			t.Datetime("created_at")
			t.Datetime("released_at").Nullable()
		},
		"privacy_requests": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("user_id")
			t.String("kind")
			t.String("status")
			t.Datetime("requested_at")
		},
	}
	for name, definition := range tables {
		if err := createTable(ctx, schema, name, definition); err != nil {
			return err
		}
	}
	return nil
}

func (m addWorkspaceOperations) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"privacy_requests", "legal_holds", "user_settings", "document_favorites"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
