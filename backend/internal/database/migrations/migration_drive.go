package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addDriveFeatures struct{}

func (m addDriveFeatures) Name() string { return "028_add_drive_features" }

// Up adds resumable upload sessions, file requests (upload links for people
// without an account) and personal API tokens used by WebDAV and agents.
func (m addDriveFeatures) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	tables := map[string]func(*surrealgoorm.Table){
		"upload_sessions": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("user_id")
			t.String("name")
			t.String("mime_type")
			t.Int("size")
			t.Int("received")
			t.String("folder_id").Nullable()
			t.String("category_id").Nullable()
			t.String("access").Nullable()
			t.String("file_request_id").Nullable()
			t.String("status")
			t.Datetime("expires_at")
			t.Timestamps()
		},
		"file_requests": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("folder_id")
			t.String("title")
			t.String("instructions").Nullable()
			t.String("token_hash")
			t.String("created_by")
			t.Int("max_files")
			t.Int("received_files")
			t.Datetime("expires_at")
			t.Datetime("closed_at").Nullable()
			t.Timestamps()
		},
		"api_tokens": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("user_id")
			t.String("name")
			t.String("prefix")
			t.String("token_hash")
			t.Array("scopes")
			t.Datetime("last_used_at").Nullable()
			t.Datetime("expires_at").Nullable()
			t.Datetime("revoked_at").Nullable()
			t.Timestamps()
		},
	}
	for _, name := range []string{"upload_sessions", "file_requests", "api_tokens"} {
		if err := createTable(ctx, schema, name, tables[name]); err != nil {
			return err
		}
	}
	return execDDL(ctx, schema.Db,
		"DEFINE INDEX file_requests_token ON file_requests FIELDS token_hash UNIQUE;"+
			"DEFINE INDEX file_requests_folder ON file_requests FIELDS org_id, folder_id;"+
			"DEFINE INDEX api_tokens_hash ON api_tokens FIELDS token_hash UNIQUE;"+
			"DEFINE INDEX api_tokens_user ON api_tokens FIELDS org_id, user_id;"+
			"DEFINE INDEX upload_sessions_user ON upload_sessions FIELDS user_id, status;")
}

func (m addDriveFeatures) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"api_tokens", "file_requests", "upload_sessions"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
