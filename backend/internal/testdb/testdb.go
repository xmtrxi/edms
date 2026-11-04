// Package testdb gives integration tests a migrated, empty SurrealDB
// database. Tests using it are skipped unless EDMS_TEST_SURREALDB_URL is set,
// for example:
//
//	surreal start --user root --pass root memory --bind 127.0.0.1:8099 &
//	EDMS_TEST_SURREALDB_URL=ws://127.0.0.1:8099/rpc go test ./...
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/database/migrations"
)

// Open connects to a fresh database named after a random suffix, runs every
// migration and removes the database when the test ends.
func Open(t testing.TB) *surrealdb.DB {
	t.Helper()
	url := os.Getenv("EDMS_TEST_SURREALDB_URL")
	if url == "" {
		t.Skip("set EDMS_TEST_SURREALDB_URL to run SurrealDB integration tests")
	}
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "test_" + hex.EncodeToString(suffix)

	ctx := context.Background()
	db, err := database.Connect(ctx, config.DatabaseConfig{
		URL: url, Namespace: "edms_test", Database: name,
		User: envOr("EDMS_TEST_SURREALDB_USER", "root"), Password: envOr("EDMS_TEST_SURREALDB_PASS", "root"),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Client().Use(ctx, "edms_test", name); err != nil {
		t.Fatalf("use database: %v", err)
	}
	if err := database.AutoMigrate(ctx, db, migrations.Migrations()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		_, _ = surrealdb.Query[any](context.Background(), db.Client(), "REMOVE DATABASE "+name, nil)
		_ = db.Close(context.Background())
	})
	return db.Client()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
