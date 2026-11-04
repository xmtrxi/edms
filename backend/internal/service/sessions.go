package service

import (
	"context"
	"errors"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

var ErrSessionNotFound = errors.New("session not found")

// SessionMeta describes where a sign-in came from.
type SessionMeta struct {
	Method    string
	IP        string
	UserAgent string
}

// SessionService stores sign-ins so they can be listed and revoked.
type SessionService struct {
	db  *surrealdb.DB
	ttl time.Duration
}

func NewSessionService(db *surrealdb.DB, ttl time.Duration) *SessionService {
	if ttl <= 0 {
		ttl = 72 * time.Hour
	}
	return &SessionService{db: db, ttl: ttl}
}

func (s *SessionService) Create(ctx context.Context, userID string, meta SessionMeta) (*database.Session, error) {
	now := time.Now().UTC()
	if len(meta.UserAgent) > 300 {
		meta.UserAgent = meta.UserAgent[:300]
	}
	return surrealdb.Create[database.Session](ctx, s.db, "sessions", map[string]any{
		"user_id": userID, "method": meta.Method, "ip": meta.IP, "user_agent": meta.UserAgent,
		"created_at": now, "expires_at": now.Add(s.ttl), "last_seen_at": now,
	})
}

// Active returns the session when it belongs to the user and is still valid.
// It refreshes last_seen_at at most every few minutes.
func (s *SessionService) Active(ctx context.Context, id, userID string) (*database.Session, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	session, err := surrealdb.Select[database.Session](ctx, s.db, rid)
	if err != nil || session == nil || session.UserID != userID || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionNotFound
	}
	if time.Since(session.LastSeenAt) > 5*time.Minute {
		_, _ = surrealdb.Merge[database.Session](ctx, s.db, rid, map[string]any{"last_seen_at": time.Now().UTC()})
	}
	return session, nil
}

// List returns the user's live sessions, newest first.
func (s *SessionService) List(ctx context.Context, userID string) ([]database.Session, error) {
	return queryRows[database.Session](ctx, s.db,
		"SELECT * FROM sessions WHERE user_id = $user AND revoked_at = NONE AND expires_at > time::now() ORDER BY last_seen_at DESC",
		map[string]any{"user": userID})
}

// Revoke ends one of the user's sessions.
func (s *SessionService) Revoke(ctx context.Context, userID, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return ErrSessionNotFound
	}
	session, err := surrealdb.Select[database.Session](ctx, s.db, rid)
	if err != nil || session == nil || session.UserID != userID {
		return ErrSessionNotFound
	}
	_, err = surrealdb.Merge[database.Session](ctx, s.db, rid, map[string]any{"revoked_at": time.Now().UTC()})
	return err
}

// RevokeAll ends every session of the user except keep, which may be empty.
func (s *SessionService) RevokeAll(ctx context.Context, userID, keep string) error {
	vars := map[string]any{"user": userID, "now": time.Now().UTC()}
	statement := "UPDATE sessions SET revoked_at = $now WHERE user_id = $user AND revoked_at = NONE"
	if keep != "" {
		rid, err := database.ParseRecordID(keep)
		if err == nil {
			statement += " AND id != $keep"
			vars["keep"] = rid
		}
	}
	_, err := surrealdb.Query[any](ctx, s.db, statement, vars)
	return err
}

// Purge deletes sessions and emailed tokens that can no longer be used.
func (s *SessionService) Purge(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	_, err := surrealdb.Query[any](ctx, s.db,
		"DELETE sessions WHERE expires_at < $cutoff OR revoked_at < $cutoff; DELETE auth_tokens WHERE expires_at < $cutoff;",
		map[string]any{"cutoff": cutoff})
	return err
}
