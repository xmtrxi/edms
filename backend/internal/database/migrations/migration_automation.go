package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
)

type addAutomation struct{}

func (m addAutomation) Name() string { return "032_add_automation" }

// Up adds automation rules and their run history.
func (m addAutomation) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := execDDL(ctx, schema.Db, `
DEFINE TABLE automation_rules SCHEMAFULL;
DEFINE FIELD org_id ON automation_rules TYPE string;
DEFINE FIELD name ON automation_rules TYPE string;
DEFINE FIELD enabled ON automation_rules TYPE bool;
DEFINE FIELD trigger ON automation_rules TYPE string;
DEFINE FIELD folder_id ON automation_rules TYPE option<string>;
DEFINE FIELD conditions ON automation_rules TYPE object FLEXIBLE;
DEFINE FIELD actions ON automation_rules TYPE array<object>;
DEFINE FIELD created_by ON automation_rules TYPE string;
DEFINE FIELD run_count ON automation_rules TYPE int;
DEFINE FIELD last_run_at ON automation_rules TYPE option<datetime>;
DEFINE FIELD created_at ON automation_rules TYPE datetime;
DEFINE FIELD updated_at ON automation_rules TYPE datetime;
DEFINE INDEX automation_rules_org ON automation_rules FIELDS org_id, trigger;

DEFINE TABLE automation_runs SCHEMAFULL;
DEFINE FIELD org_id ON automation_runs TYPE string;
DEFINE FIELD rule_id ON automation_runs TYPE string;
DEFINE FIELD document_id ON automation_runs TYPE string;
DEFINE FIELD trigger ON automation_runs TYPE string;
DEFINE FIELD status ON automation_runs TYPE string;
DEFINE FIELD detail ON automation_runs TYPE option<string>;
DEFINE FIELD created_at ON automation_runs TYPE datetime;
DEFINE INDEX automation_runs_rule ON automation_runs FIELDS rule_id, created_at;
`); err != nil {
		return err
	}
	// array<object> implicitly defines actions.* as a strict object, so the
	// flexible element type has to overwrite it.
	_, err := surrealdb.Query[any](ctx, schema.Db, "DEFINE FIELD OVERWRITE actions[*] ON automation_rules TYPE object FLEXIBLE;", nil)
	return err
}

func (m addAutomation) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"automation_runs", "automation_rules"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
