package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

type addAccessModel struct{}

func (m addAccessModel) Name() string { return "027_add_access_model" }

// Up adds document-level grants, sharing by link and email, and access
// requests. Existing documents keep following their folder's access.
func (m addAccessModel) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	tables := map[string]func(*surrealgoorm.Table){
		"document_grants": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("document_id")
			t.String("principal_type")
			t.String("principal_id")
			t.String("role")
			t.Array("allow").Nullable()
			t.Array("deny").Nullable()
			t.String("created_by")
			t.Timestamps()
		},
		"access_requests": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("document_id")
			t.String("requester_id")
			t.String("role")
			t.String("message").Nullable()
			t.String("status")
			t.String("decided_by").Nullable()
			t.Timestamps()
		},
	}
	for _, name := range []string{"document_grants", "access_requests"} {
		if err := createTable(ctx, schema, name, tables[name]); err != nil {
			return err
		}
	}
	ddl := "DEFINE FIELD inherit_permissions ON documents TYPE bool DEFAULT true;" +
		"DEFINE FIELD sensitivity ON documents TYPE option<string>;" +
		"DEFINE FIELD kind ON shares TYPE option<string>;" +
		"DEFINE FIELD link_scope ON shares TYPE option<string>;" +
		"DEFINE FIELD password_hash ON shares TYPE option<string>;" +
		"DEFINE FIELD allow_download ON shares TYPE option<bool>;" +
		"DEFINE FIELD org_id ON email_deliveries TYPE option<string>;" +
		// Organization settings are a strict object; declare the sharing policy.
		"DEFINE FIELD settings.sharing ON organizations TYPE option<object>;" +
		"DEFINE FIELD settings.sharing.allow_external ON organizations TYPE option<bool>;" +
		"DEFINE FIELD settings.sharing.allow_public_links ON organizations TYPE option<bool>;" +
		"DEFINE FIELD settings.sharing.allowed_domains ON organizations TYPE option<array<string>>;" +
		"DEFINE FIELD settings.sharing.max_link_days ON organizations TYPE option<int>;" +
		"DEFINE INDEX form_templates_org ON form_templates FIELDS org_id, status;" +
		"DEFINE INDEX document_grants_principal ON document_grants FIELDS document_id, principal_type, principal_id UNIQUE;" +
		"DEFINE INDEX document_grants_org ON document_grants FIELDS org_id, principal_type, principal_id;" +
		"DEFINE INDEX folder_grants_org_principal ON folder_grants FIELDS org_id, principal_type, principal_id;" +
		"DEFINE INDEX shares_document ON shares FIELDS document_id;" +
		"DEFINE INDEX shares_user ON shares FIELDS shared_with_user_id;" +
		"DEFINE INDEX access_requests_document ON access_requests FIELDS document_id, status;"
	if err := execDDL(ctx, schema.Db, ddl); err != nil {
		return err
	}
	for _, statement := range []string{
		"UPDATE documents SET inherit_permissions = true WHERE inherit_permissions = NONE;",
		"UPDATE documents SET sensitivity = 'internal' WHERE sensitivity = NONE OR sensitivity = '';",
		"UPDATE shares SET kind = IF shared_with_user_id != NONE AND shared_with_user_id != '' THEN 'user' ELSE 'email' END WHERE kind = NONE;",
		"UPDATE shares SET allow_download = (permission = 'download') WHERE allow_download = NONE;",
		"UPDATE shares SET permission = 'viewer' WHERE permission = NONE OR permission IN ['view', 'download'];",
	} {
		if _, err := surrealdb.Query[any](ctx, schema.Db, statement, nil); err != nil {
			return err
		}
	}
	return nil
}

func (m addAccessModel) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"access_requests", "document_grants"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
