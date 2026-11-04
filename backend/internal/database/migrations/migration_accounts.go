package migrations

import (
	"context"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
)

type addAccountSecurity struct{}

func (m addAccountSecurity) Name() string { return "034_add_account_security" }

// Up adds revocable sessions, emailed single-use tokens, verified addresses
// and two-step sign-in. People who already exist are treated as verified.
func (m addAccountSecurity) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	return execDDL(ctx, schema.Db, `
DEFINE FIELD email_verified_at ON users TYPE option<datetime>;
DEFINE FIELD mfa_secret ON users TYPE option<string>;
DEFINE FIELD mfa_enabled ON users TYPE option<bool>;
DEFINE FIELD mfa_recovery_codes ON users TYPE option<array<string>>;
DEFINE FIELD password_changed_at ON users TYPE option<datetime>;
UPDATE users SET email_verified_at = created_at WHERE email_verified_at = NONE;

DEFINE TABLE sessions SCHEMAFULL;
DEFINE FIELD user_id ON sessions TYPE string;
DEFINE FIELD method ON sessions TYPE string;
DEFINE FIELD ip ON sessions TYPE option<string>;
DEFINE FIELD user_agent ON sessions TYPE option<string>;
DEFINE FIELD created_at ON sessions TYPE datetime;
DEFINE FIELD expires_at ON sessions TYPE datetime;
DEFINE FIELD last_seen_at ON sessions TYPE datetime;
DEFINE FIELD revoked_at ON sessions TYPE option<datetime>;
DEFINE INDEX sessions_user ON sessions FIELDS user_id;

DEFINE TABLE auth_tokens SCHEMAFULL;
DEFINE FIELD user_id ON auth_tokens TYPE string;
DEFINE FIELD purpose ON auth_tokens TYPE string;
DEFINE FIELD email ON auth_tokens TYPE string;
DEFINE FIELD token_hash ON auth_tokens TYPE string;
DEFINE FIELD expires_at ON auth_tokens TYPE datetime;
DEFINE FIELD used_at ON auth_tokens TYPE option<datetime>;
DEFINE FIELD created_at ON auth_tokens TYPE datetime;
DEFINE INDEX auth_tokens_hash ON auth_tokens FIELDS token_hash UNIQUE;
DEFINE INDEX auth_tokens_user ON auth_tokens FIELDS user_id, purpose;
`)
}

func (m addAccountSecurity) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	if err := removeTable(ctx, schema, "auth_tokens"); err != nil {
		return err
	}
	return removeTable(ctx, schema, "sessions")
}
