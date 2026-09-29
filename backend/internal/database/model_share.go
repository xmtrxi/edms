package database

import "time"

type Share struct {
	ID               RecordID  `json:"id,omitempty"`
	DocumentID       string    `json:"document_id"`
	CreatedBy        string    `json:"created_by"`
	SharedWithUserID string    `json:"shared_with_user_id,omitempty"`
	SharedWithEmail  string    `json:"shared_with_email"`
	Permission       string    `json:"permission"`
	TokenHash        string    `json:"token_hash"`
	ExpiresAt        time.Time `json:"expires_at"`
	RevokedAt        time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
