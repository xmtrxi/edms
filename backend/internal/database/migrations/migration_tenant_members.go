package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createTenantMembersTable struct{}

func (m createTenantMembersTable) Name() string { return "003_create_tenant_members_table" }

func (m createTenantMembersTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "tenant_members", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("tenant_id")
		t.String("user_id")
		t.String("role").Default(database.TenantRoleMember)
		t.Timestamps()
	})
}

func (m createTenantMembersTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "tenant_members")
}
