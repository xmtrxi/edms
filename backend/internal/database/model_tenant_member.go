package database

import "time"

type TenantMember struct {
	ID        string    `json:"id,omitempty"`
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	TenantRoleOwner  = "owner"
	TenantRoleAdmin  = "admin"
	TenantRoleMember = "member"
)
