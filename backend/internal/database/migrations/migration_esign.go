package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addNativeSigning struct{}

func (m addNativeSigning) Name() string { return "030_add_native_signing" }

// Up adds signature envelopes, their recipients and fields, and the audit
// trail printed on each certificate of completion.
func (m addNativeSigning) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	tables := map[string]func(*surrealgoorm.Table){
		"signature_envelopes": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("document_id")
			t.String("source_version_id").Nullable()
			t.String("source_path")
			t.String("source_hash")
			t.Int("pages")
			t.String("title")
			t.String("message").Nullable()
			t.String("status")
			t.Bool("sequential")
			t.Bool("require_otp")
			t.String("created_by")
			t.String("completed_version_id").Nullable()
			t.String("completed_hash").Nullable()
			t.Datetime("completed_at").Nullable()
			t.Datetime("voided_at").Nullable()
			t.String("void_reason").Nullable()
			t.Datetime("expires_at")
			t.Timestamps()
		},
		"signature_recipients": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("envelope_id")
			t.String("name")
			t.String("email")
			t.String("user_id").Nullable()
			t.String("role")
			t.Int("position")
			t.String("status")
			t.String("nonce")
			t.String("token_hash")
			t.String("otp_hash").Nullable()
			t.Datetime("otp_expires_at").Nullable()
			t.Int("otp_attempts")
			t.Datetime("verified_at").Nullable()
			t.Datetime("viewed_at").Nullable()
			t.Datetime("signed_at").Nullable()
			t.Datetime("declined_at").Nullable()
			t.String("decline_reason").Nullable()
			t.String("ip").Nullable()
			t.String("user_agent").Nullable()
			t.Timestamps()
		},
		"signature_fields": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("envelope_id")
			t.String("recipient_id")
			t.String("kind")
			t.Int("page")
			t.Float("x")
			t.Float("y")
			t.Float("w")
			t.Float("h")
			t.Bool("required")
			t.String("label").Nullable()
			t.String("value").Nullable()
			t.String("image_path").Nullable()
			t.Datetime("filled_at").Nullable()
			t.Datetime("created_at")
		},
		"signature_events": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("envelope_id")
			t.String("recipient_id").Nullable()
			t.String("action")
			t.String("detail").Nullable()
			t.String("ip").Nullable()
			t.String("user_agent").Nullable()
			t.Datetime("created_at")
		},
	}
	for _, name := range []string{"signature_envelopes", "signature_recipients", "signature_fields", "signature_events"} {
		if err := createTable(ctx, schema, name, tables[name]); err != nil {
			return err
		}
	}
	return execDDL(ctx, schema.Db,
		"DEFINE INDEX signature_envelopes_document ON signature_envelopes FIELDS org_id, document_id;"+
			"DEFINE INDEX signature_recipients_envelope ON signature_recipients FIELDS envelope_id;"+
			"DEFINE INDEX signature_recipients_token ON signature_recipients FIELDS token_hash UNIQUE;"+
			"DEFINE INDEX signature_recipients_user ON signature_recipients FIELDS user_id, status;"+
			"DEFINE INDEX signature_fields_envelope ON signature_fields FIELDS envelope_id;"+
			"DEFINE INDEX signature_events_envelope ON signature_events FIELDS envelope_id;")
}

func (m addNativeSigning) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"signature_events", "signature_fields", "signature_recipients", "signature_envelopes"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
