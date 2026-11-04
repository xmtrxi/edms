package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addAgentAccounts struct{}

func (m addAgentAccounts) Name() string { return "029_add_agent_accounts" }

// Up marks users as people or AI agents. Existing users are people.
func (m addAgentAccounts) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, "DEFINE FIELD kind ON users TYPE option<string>;")
}

func (m addAgentAccounts) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	return nil
}
