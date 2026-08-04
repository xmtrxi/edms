package database

import (
	"context"
	"errors"
	"fmt"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/kemboi22/surreal-go-orm/migrator"
	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/config"
)

type DB struct {
	client *surrealdb.DB
}

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	if cfg.URL == "" {
		return nil, errors.New("surrealdb url is required")
	}

	client, err := surrealdb.FromEndpointURLString(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect to surrealdb: %w", err)
	}

	if err := client.Use(ctx, cfg.Namespace, cfg.Database); err != nil {
		_ = client.Close(ctx)
		return nil, fmt.Errorf("select surrealdb namespace/database: %w", err)
	}

	if cfg.User != "" || cfg.Password != "" {
		if _, err := client.SignIn(ctx, surrealdb.Auth{
			Username: cfg.User,
			Password: cfg.Password,
		}); err != nil {
			_ = client.Close(ctx)
			return nil, fmt.Errorf("sign in to surrealdb: %w", err)
		}
	}

	return &DB{client: client}, nil
}

func (db *DB) Close(ctx context.Context) error {
	if db == nil || db.client == nil {
		return nil
	}

	return db.client.Close(ctx)
}

func (db *DB) Client() *surrealdb.DB {
	if db == nil {
		return nil
	}
	return db.client
}

func (db *DB) Health(ctx context.Context) error {
	_, err := surrealdb.Query[any](ctx, db.client, "RETURN 1;", nil)
	return err
}

func AutoMigrate(ctx context.Context, db *DB, migrations []surrealgoorm.Migration) error {
	return migrator.NewAutoMigrator(db.client).AutoMigrate(ctx, migrations)
}
