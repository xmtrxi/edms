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
	Kind      string    `json:"kind,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"

	UserKindAgent = "agent"
)
