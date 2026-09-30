package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addRetention struct{}

func (m addRetention) Name() string { return "033_add_retention" }

// Up adds retention policies.
func (m addRetention) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE TABLE retention_policies SCHEMAFULL;
DEFINE FIELD org_id ON retention_policies TYPE string;
DEFINE FIELD name ON retention_policies TYPE string;
DEFINE FIELD enabled ON retention_policies TYPE bool;
DEFINE FIELD folder_id ON retention_policies TYPE option<string>;
DEFINE FIELD sensitivity ON retention_policies TYPE option<string>;
DEFINE FIELD age_basis ON retention_policies TYPE string;
DEFINE FIELD keep_days ON retention_policies TYPE int;
DEFINE FIELD action ON retention_policies TYPE string;
DEFINE FIELD created_by ON retention_policies TYPE string;
DEFINE FIELD last_run_at ON retention_policies TYPE option<datetime>;
DEFINE FIELD last_count ON retention_policies TYPE int;
DEFINE FIELD created_at ON retention_policies TYPE datetime;
DEFINE FIELD updated_at ON retention_policies TYPE datetime;
DEFINE INDEX retention_policies_org ON retention_policies FIELDS org_id;
`)
}

func (m addRetention) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return removeTable(ctx, schema, "retention_policies")
}
