package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addEmailDelivery struct{}

func (m addEmailDelivery) Name() string { return "025_add_email_delivery" }
func (m addEmailDelivery) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return createTable(ctx, schema, "email_deliveries", func(t *surrealgoorm.Table) {
		t.ID()
		t.String("recipient")
		t.String("subject")
		t.String("body")
		t.String("status")
		t.Int("attempts").Default(0)
		t.String("last_error").Nullable()
		t.String("created_by")
		t.Datetime("created_at")
		t.Datetime("updated_at")
		t.Datetime("delivered_at").Nullable()
	})
}
func (m addEmailDelivery) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "email_deliveries")
}
