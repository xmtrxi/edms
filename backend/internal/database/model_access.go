package database

import "time"

// Role is the editable metadata for a Casbin role. The effective permissions
// and user assignments live in casbin_rules so Casbin remains the source of
// truth for authorization decisions.
type Role struct {
	ID          RecordID  `json:"id,omitempty"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Protected   bool      `json:"protected"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CasbinRule struct {
	ID        RecordID  `json:"id,omitempty"`
	PType     string    `json:"ptype"`
	V0        string    `json:"v0"`
	V1        string    `json:"v1"`
	V2        string    `json:"v2"`
	V3        string    `json:"v3"`
	V4        string    `json:"v4"`
	V5        string    `json:"v5"`
	CreatedAt time.Time `json:"created_at"`
}

type Invitation struct {
	ID         RecordID  `json:"id,omitempty"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	TokenHash  string    `json:"-"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expires_at"`
	AcceptedAt time.Time `json:"accepted_at,omitempty"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}
