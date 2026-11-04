package database

import "time"

type User struct {
	ID           RecordID `json:"id,omitempty"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	PasswordHash string   `json:"password_hash"`
	Status       string   `json:"status"`
	LastOrgID    string   `json:"last_org_id,omitempty"`
	// Kind is "agent" for AI agent accounts, which cannot sign in with a
	// password and act only through their tokens. Empty means a person.
	Kind string `json:"kind,omitempty"`
	// EmailVerifiedAt is set once the person proves they own the address.
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	// MFASecret is the TOTP secret, encrypted. MFAEnabled turns it on once
	// the person has confirmed a code; recovery codes are stored hashed.
	MFASecret        string     `json:"mfa_secret,omitempty"`
	MFAEnabled       bool       `json:"mfa_enabled,omitempty"`
	MFARecoveryCodes []string   `json:"mfa_recovery_codes,omitempty"`
	PasswordChanged  *time.Time `json:"password_changed_at,omitempty"`
	// LastDigestAt is when the daily workflow digest was last emailed.
	LastDigestAt *time.Time `json:"last_digest_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"

	UserKindAgent = "agent"
)
