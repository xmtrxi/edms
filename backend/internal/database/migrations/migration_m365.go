package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addM365Sessions struct{}

func (m addM365Sessions) Name() string { return "037_add_m365_sessions" }

// Up adds Microsoft 365 working copies.
func (m addM365Sessions) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE TABLE m365_sessions SCHEMAFULL;
DEFINE FIELD org_id ON m365_sessions TYPE string;
DEFINE FIELD document_id ON m365_sessions TYPE string;
DEFINE FIELD user_id ON m365_sessions TYPE string;
DEFINE FIELD path ON m365_sessions TYPE string;
DEFINE FIELD web_url ON m365_sessions TYPE string;
DEFINE FIELD base_version_id ON m365_sessions TYPE string;
DEFINE FIELD checksum ON m365_sessions TYPE string;
DEFINE FIELD created_at ON m365_sessions TYPE datetime;
DEFINE FIELD updated_at ON m365_sessions TYPE datetime;
DEFINE FIELD closed_at ON m365_sessions TYPE option<datetime>;
DEFINE INDEX m365_sessions_document ON m365_sessions FIELDS document_id, user_id;
`)
}

func (m addM365Sessions) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "m365_sessions")
}
