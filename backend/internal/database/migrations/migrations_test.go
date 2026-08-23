package migrations

import (
	"strings"
	"testing"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

func TestMigrationsAreRegisteredInOrder(t *testing.T) {
	migrations := Migrations()
	expected := []string{
		"001_create_users_table",
		"002_create_documents_table",
		"003_create_document_versions_table",
		"004_create_tags_table",
		"005_create_document_tags_table",
		"006_create_document_extractions_table",
		"007_create_comments_table",
		"008_create_shares_table",
		"009_create_signing_requests_table",
		"010_create_processing_jobs_table",
		"011_create_audit_events_table",
		"012_create_workflow_requests_table",
		"013_create_workflow_actions_table",
		"014_create_categories_table",
		"015_create_document_categories_table",
		"016_expand_versioning_and_routing",
		"017_expand_signing_routing",
	}
	if len(migrations) != len(expected) {
		t.Fatalf("expected %d migrations, got %d", len(expected), len(migrations))
	}
	for i, migration := range migrations {
		if migration.Name() != expected[i] {
			t.Fatalf("migration %d: expected %q, got %q", i, expected[i], migration.Name())
		}
	}
}

func TestUserTableDefinitionUsesSurrealORMBuilder(t *testing.T) {
	table := &surrealgoorm.Table{Name: "users"}
	table.ID()
	table.String("email").Unique()
	table.String("name")
	table.String("password_hash")
	table.String("status").Default(database.UserStatusActive)
	table.Timestamps()

	sql := table.Build()
	for _, part := range []string{
		"DEFINE TABLE users SCHEMAFULL",
		"DEFINE FIELD email ON users type string",
		"DEFINE INDEX email_unique",
		"DEFINE FIELD password_hash ON users type string",
		"DEFINE FIELD status ON users type string DEFAULT 'active'",
		"DEFINE FIELD created_at ON users type datetime",
	} {
		if !strings.Contains(sql, part) {
			t.Fatalf("expected SQL to contain %q, got:\n%s", part, sql)
		}
	}
}
