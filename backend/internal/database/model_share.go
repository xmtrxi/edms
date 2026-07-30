package database

import "time"

type Share struct {
	ID         string    `json:"id,omitempty"`
	DocumentID string    `json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	TokenHash  string    `json:"token_hash"`
	ExpiresAt  time.Time `json:"expires_at"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
