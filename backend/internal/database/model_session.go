package database

import "time"

// Session is one sign-in. Every bearer token names its session, so signing
// out or revoking a session takes effect at once.
type Session struct {
	ID         RecordID   `json:"id,omitempty"`
	UserID     string     `json:"user_id"`
	Method     string     `json:"method"`
	IP         string     `json:"ip,omitempty"`
	UserAgent  string     `json:"user_agent,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

const (
	SessionMethodPassword = "password"
	SessionMethodSSO      = "sso"
	SessionMethodSignup   = "signup"
	SessionMethodInvite   = "invite"
)

// AuthToken is a single-use secret sent by email to reset a password or
// confirm an address. Only its hash is stored.
type AuthToken struct {
	ID        RecordID   `json:"id,omitempty"`
	UserID    string     `json:"user_id"`
	Purpose   string     `json:"purpose"`
	Email     string     `json:"email"`
	TokenHash string     `json:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

const (
	AuthTokenPasswordReset = "password_reset"
	AuthTokenVerifyEmail   = "verify_email"
)
