package database

import "time"

type User struct {
	ID           RecordID  `json:"id,omitempty"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"password_hash"`
	Status       string    `json:"status"`
	LastOrgID    string    `json:"last_org_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)
