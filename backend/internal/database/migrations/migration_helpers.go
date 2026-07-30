package migrations

import (
	"context"
	"fmt"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

func createTable(ctx context.Context, schema surrealgoorm.Schema, name string, fn func(*surrealgoorm.Table)) error {
	table := &surrealgoorm.Table{Name: name}
	fn(table)

	if _, err := surrealdb.Query[any](ctx, schema.Db, table.Build(), nil); err != nil {
		return fmt.Errorf("create table %s: %w", name, err)
	}

	return nil
}

func removeTable(ctx context.Context, schema surrealgoorm.Schema, name string) error {
	if _, err := surrealdb.Query[any](ctx, schema.Db, "REMOVE TABLE "+name+";", nil); err != nil {
		return fmt.Errorf("remove table %s: %w", name, err)
	}

	return nil
}
