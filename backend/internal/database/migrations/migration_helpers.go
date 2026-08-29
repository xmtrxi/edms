package migrations

import (
	"context"
	"fmt"
	"strings"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists")
}

func execDDL(ctx context.Context, db *surrealdb.DB, statement string) error {
	// Make DDL idempotent: add IF NOT EXISTS where supported and swallow
	// "already exists" errors from older SurrealDB releases.
	idempotent := statement
	// Only patch DEFINE statements - don't touch REMOVE, etc.
	idempotent = strings.ReplaceAll(idempotent, "DEFINE TABLE ", "DEFINE TABLE IF NOT EXISTS ")
	idempotent = strings.ReplaceAll(idempotent, "DEFINE FIELD ", "DEFINE FIELD IF NOT EXISTS ")
	idempotent = strings.ReplaceAll(idempotent, "DEFINE INDEX ", "DEFINE INDEX IF NOT EXISTS ")
	if _, err := surrealdb.Query[any](ctx, db, idempotent, nil); err != nil {
		if isAlreadyExistsErr(err) {
			return nil
		}
		// Fallback: if IF NOT EXISTS syntax isn't supported by this SurrealDB
		// version, retry with original statement and still ignore already-exists.
		if strings.Contains(idempotent, "IF NOT EXISTS") {
			if _, err2 := surrealdb.Query[any](ctx, db, statement, nil); err2 != nil {
				if isAlreadyExistsErr(err2) {
					return nil
				}
				return err2
			}
			return nil
		}
		return err
	}
	return nil
}

func createTable(ctx context.Context, schema surrealgoorm.Schema, name string, fn func(*surrealgoorm.Table)) error {
	table := &surrealgoorm.Table{Name: name}
	fn(table)
	normalizeTableTypes(table)

	sql := table.Build()
	// Rewrite ORM-generated DDL to be idempotent.
	sql = strings.ReplaceAll(sql, "DEFINE TABLE ", "DEFINE TABLE IF NOT EXISTS ")
	sql = strings.ReplaceAll(sql, "DEFINE FIELD ", "DEFINE FIELD IF NOT EXISTS ")
	sql = strings.ReplaceAll(sql, "DEFINE INDEX ", "DEFINE INDEX IF NOT EXISTS ")

	if _, err := surrealdb.Query[any](ctx, schema.Db, sql, nil); err != nil {
		if isAlreadyExistsErr(err) {
			return nil
		}
		// Retry with original if IF NOT EXISTS not supported.
		if _, err2 := surrealdb.Query[any](ctx, schema.Db, table.Build(), nil); err2 != nil {
			if isAlreadyExistsErr(err2) {
				return nil
			}
			return fmt.Errorf("create table %s: %w", name, err2)
		}
		return nil
	}

	return nil
}

func normalizeTableTypes(table *surrealgoorm.Table) {
	// SurrealDB names its boolean kind `bool`. The ORM currently emits the
	// legacy/non-SurrealQL name `boolean`, which makes the entire DEFINE TABLE
	// batch fail on current SurrealDB releases.
	for _, column := range table.Columns {
		if column.Type == "boolean" {
			column.Type = "bool"
		}
	}
}

func removeTable(ctx context.Context, schema surrealgoorm.Schema, name string) error {
	if _, err := surrealdb.Query[any](ctx, schema.Db, "REMOVE TABLE "+name+";", nil); err != nil {
		return fmt.Errorf("remove table %s: %w", name, err)
	}

	return nil
}
