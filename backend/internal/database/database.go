package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/kemboi22/surreal-go-orm/migrator"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	surrealhttp "github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/xmtrxi/edms/internal/config"
)

type DB struct {
	client *surrealdb.DB
}

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	if cfg.URL == "" {
		return nil, errors.New("surrealdb url is required")
	}

	client, err := open(ctx, cfg.URL)
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

// open mirrors surrealdb.FromEndpointURLString but wraps the request
// marshaler so datetimes are sent in the form every server version accepts.
func open(ctx context.Context, rawURL string) (*surrealdb.DB, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, err
	}
	conf := connection.NewConfig(u)
	conf.Marshaler = datetimeMarshaler{inner: conf.Marshaler}
	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid connection config: %w", err)
	}

	var con connection.Connection
	switch u.Scheme {
	case "http", "https":
		con = surrealhttp.New(conf)
	case "ws", "wss":
		con = gorillaws.New(conf)
	default:
		return nil, fmt.Errorf("unsupported surrealdb url scheme %q", u.Scheme)
	}
	return surrealdb.FromConnection(ctx, con)
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
