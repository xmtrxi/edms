package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addNotifications struct{}

func (m addNotifications) Name() string { return "035_add_notifications" }

// Up adds the in-app notification inbox and tracks the daily digest.
func (m addNotifications) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE TABLE notifications SCHEMAFULL;
DEFINE FIELD org_id ON notifications TYPE option<string>;
DEFINE FIELD user_id ON notifications TYPE string;
DEFINE FIELD kind ON notifications TYPE string;
DEFINE FIELD title ON notifications TYPE string;
DEFINE FIELD body ON notifications TYPE option<string>;
DEFINE FIELD link ON notifications TYPE option<string>;
DEFINE FIELD actor_id ON notifications TYPE option<string>;
DEFINE FIELD resource ON notifications TYPE option<string>;
DEFINE FIELD resource_id ON notifications TYPE option<string>;
DEFINE FIELD read_at ON notifications TYPE option<datetime>;
DEFINE FIELD created_at ON notifications TYPE datetime;
DEFINE INDEX notifications_user ON notifications FIELDS user_id, created_at;
DEFINE FIELD last_digest_at ON users TYPE option<datetime>;
`)
}

func (m addNotifications) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "notifications")
}
