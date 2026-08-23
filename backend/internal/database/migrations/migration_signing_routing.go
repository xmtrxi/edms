package migrations

import (
	"context"

	"github.com/surrealdb/surrealdb.go"
	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type expandSigningRouting struct{}

func (m expandSigningRouting) Name() string { return "017_expand_signing_routing" }

func (m expandSigningRouting) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	statements := []string{
		"DEFINE FIELD document_version_id ON signing_requests TYPE string;",
		"DEFINE FIELD workflow_request_id ON signing_requests TYPE string;",
		"DEFINE FIELD workflow_step_id ON signing_requests TYPE string;",
		"DEFINE FIELD signing_url ON signing_requests TYPE option<string>;",
		"DEFINE FIELD completed_version_id ON signing_requests TYPE option<string>;",
		"DEFINE FIELD signing_request_id ON workflow_steps TYPE option<string>;",
	}
	for _, statement := range statements {
		if _, err := surrealdb.Query[any](ctx, schema.Db, statement, nil); err != nil {
			return err
		}
	}
	return nil
}

func (m expandSigningRouting) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return nil
}
