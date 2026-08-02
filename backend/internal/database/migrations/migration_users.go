package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/xmtrxi/edms/internal/database"
)

type createUsersTable struct{}

func (m createUsersTable) Name() string { return "001_create_users_table" }

func (m createUsersTable) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "users", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("email").Unique()
		t.String("name")
		t.String("password_hash")
		t.String("status").Default(database.UserStatusActive)
		t.Timestamps()
	})
}

func (m createUsersTable) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "users")
}
