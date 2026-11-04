package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addFoldersAndDocumentFolderID struct{}

func (m addFoldersAndDocumentFolderID) Name() string { return "018_add_folders_and_document_folder_id" }

func (m addFoldersAndDocumentFolderID) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := createTable(ctx, schema, "folders", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("owner_id")
		t.String("parent_id").Nullable()
		t.Int("depth").Default(0)
		t.String("name")
		t.String("normalized_name")
		t.String("description").Nullable()
		t.String("color").Nullable()
		t.Bool("inherit_permissions").Default(true)
		t.Int("permission_version").Default(1)
		t.String("storage_provider").Nullable()
		t.String("external_id").Nullable()
		t.String("external_path").Nullable()
		t.String("created_by")
		t.Timestamps()
	}); err != nil {
		return err
	}
	if err := createTable(ctx, schema, "folder_grants", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("folder_id")
		t.String("principal_type")
		t.String("principal_id")
		t.String("role")
		t.String("created_by")
		t.Timestamps()
	}); err != nil {
		return err
	}
	if err := createTable(ctx, schema, "folder_categories", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("folder_id")
		t.String("category_id")
		t.Datetime("created_at")
	}); err != nil {
		return err
	}

	for _, statement := range []string{
		"DEFINE FIELD ancestor_ids ON folders TYPE option<array<string>>;",
		"DEFINE FIELD allow ON folder_grants TYPE option<array<string>>;",
		"DEFINE FIELD deny ON folder_grants TYPE option<array<string>>;",
		"DEFINE FIELD folder_id ON documents TYPE option<string>;",
		"DEFINE INDEX folders_owner_parent_name ON folders FIELDS owner_id, parent_id, normalized_name UNIQUE;",
		"DEFINE INDEX folders_parent ON folders FIELDS parent_id;",
		"DEFINE INDEX documents_folder_updated ON documents FIELDS folder_id, updated_at;",
		"DEFINE INDEX folder_grants_principal ON folder_grants FIELDS folder_id, principal_type, principal_id UNIQUE;",
		"DEFINE INDEX folder_categories_unique ON folder_categories FIELDS folder_id, category_id UNIQUE;",
	} {
		if err := execDDL(ctx, schema.Db, statement); err != nil {
			return err
		}
	}
	return nil
}

func (m addFoldersAndDocumentFolderID) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, table := range []string{"folder_categories", "folder_grants", "folders"} {
		if err := removeTable(ctx, schema, table); err != nil {
			return err
		}
	}
	return nil
}
