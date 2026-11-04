package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addEmailEvents struct{}

func (m addEmailEvents) Name() string { return "038_add_email_events" }

// Up records the Message-ID of each email and when the server accepted it,
// so provider callbacks can report delivery and bounces.
func (m addEmailEvents) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE FIELD message_id ON email_deliveries TYPE option<string>;
DEFINE FIELD sent_at ON email_deliveries TYPE option<datetime>;
DEFINE INDEX email_deliveries_message ON email_deliveries FIELDS message_id;
`)
}

func (m addEmailEvents) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return nil
}
