package migrations

import (
	"context"

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
		if err := execDDL(ctx, schema.Db, statement); err != nil {
			return err
		}
	}
	return nil
}

func (m expandSigningRouting) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return nil
}
