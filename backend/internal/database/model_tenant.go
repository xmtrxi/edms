package database

import "time"

type Tenant struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	TenantStatusActive   = "active"
	TenantStatusDisabled = "disabled"
)
