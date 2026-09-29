package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

// APITokenPrefix marks personal access tokens so they are recognisable in
// logs and secret scanners, and distinguishable from session tokens.
const APITokenPrefix = "edms_"

// Token scopes. Read tokens may only use safe HTTP methods.
const (
	ScopeRead   = "read"
	ScopeWrite  = "write"
	ScopeWebDAV = "webdav"
	ScopeMCP    = "mcp"
)

var validScopes = []string{ScopeRead, ScopeWrite, ScopeWebDAV, ScopeMCP}

var ErrInvalidToken = errors.New("invalid or expired token")

type TokenService struct {
	db   *surrealdb.DB
	orgs *OrganizationService
}

func NewTokenService(db *surrealdb.DB, orgs *OrganizationService) *TokenService {
	return &TokenService{db: db, orgs: orgs}
}

type TokenInput struct {
	Name      string    `json:"name"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Create issues a token for userID in orgID. The secret is returned once.
func (s *TokenService) Create(ctx context.Context, userID, orgID string, in TokenInput) (*database.APIToken, string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, "", fmt.Errorf("give the token a name so you can recognise it later")
	}
	scopes := []string{}
	for _, scope := range in.Scopes {
		if slices.Contains(validScopes, scope) && !slices.Contains(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		scopes = []string{ScopeRead}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", err
	}
	secret := APITokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": orgID, "user_id": userID, "name": name, "prefix": secret[:len(APITokenPrefix)+6],
		"token_hash": secretHash(secret), "scopes": scopes, "created_at": now, "updated_at": now,
	}
	if !in.ExpiresAt.IsZero() {
		record["expires_at"] = in.ExpiresAt.UTC()
	}
	token, err := surrealdb.Create[database.APIToken](ctx, s.db, "api_tokens", record)
	if err != nil {
		return nil, "", err
	}
	_ = writeAudit(ctx, s.db, userID, "api_token.created", "api_token", string(token.ID), map[string]any{"name": name, "scopes": scopes})
	return token, secret, nil
}

func (s *TokenService) List(ctx context.Context, userID, orgID string) ([]database.APIToken, error) {
	return queryRows[database.APIToken](ctx, s.db,
		"SELECT * FROM api_tokens WHERE org_id = $org AND user_id = $user AND revoked_at = NONE ORDER BY created_at DESC",
		map[string]any{"org": orgID, "user": userID})
}

func (s *TokenService) Revoke(ctx context.Context, userID, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return ErrInvalidToken
	}
	token, err := surrealdb.Select[database.APIToken](ctx, s.db, rid)
	if err != nil || token == nil || token.UserID != userID {
		return ErrInvalidToken
	}
	_, err = surrealdb.Query[any](ctx, s.db, "UPDATE $token SET revoked_at = time::now()", map[string]any{"token": rid})
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "api_token.revoked", "api_token", id, nil)
	}
	return err
}

// Authenticate resolves a secret to its token and user. The user must still
// be active and a member of the token's organization.
func (s *TokenService) Authenticate(ctx context.Context, secret string) (*database.APIToken, *database.User, error) {
	if !strings.HasPrefix(secret, APITokenPrefix) {
		return nil, nil, ErrInvalidToken
	}
	tokens, err := queryRows[database.APIToken](ctx, s.db,
		"SELECT * FROM api_tokens WHERE token_hash = $hash AND revoked_at = NONE LIMIT 1", map[string]any{"hash": secretHash(secret)})
	if err != nil {
		return nil, nil, err
	}
	if len(tokens) == 0 {
		return nil, nil, ErrInvalidToken
	}
	token := tokens[0]
	if !token.ExpiresAt.IsZero() && time.Now().UTC().After(token.ExpiresAt) {
		return nil, nil, ErrInvalidToken
	}
	user, err := s.orgs.userByID(ctx, token.UserID)
	if err != nil || user == nil || user.Status != database.UserStatusActive {
		return nil, nil, ErrInvalidToken
	}
	if _, err := s.orgs.Membership(ctx, token.OrgID, token.UserID); err != nil {
		return nil, nil, ErrInvalidToken
	}
	// Recording every use would write on each request; once a minute is enough.
	if time.Since(token.LastUsedAt) > time.Minute {
		rid, _ := database.ParseRecordID(string(token.ID))
		_, _ = surrealdb.Query[any](ctx, s.db, "UPDATE $token SET last_used_at = time::now()", map[string]any{"token": rid})
	}
	return &token, user, nil
}

// HasScope reports whether token grants scope. Write implies read.
func HasScope(token *database.APIToken, scope string) bool {
	if token == nil {
		return true
	}
	if slices.Contains(token.Scopes, scope) {
		return true
	}
	return scope == ScopeRead && slices.Contains(token.Scopes, ScopeWrite)
}
