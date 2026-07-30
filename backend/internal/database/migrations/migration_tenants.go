package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createTenantsTable struct{}

func (m createTenantsTable) Name() string { return "001_create_tenants_table" }

func (m createTenantsTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "tenants", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("name")
		t.String("slug").Unique()
		t.String("status").Default(database.TenantStatusActive)
		t.Timestamps()
	})
}

func (m createTenantsTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "tenants")
}
