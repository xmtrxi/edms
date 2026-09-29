package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type OrganizationResponse struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func NewOrganizationResponse(org *database.Organization, role string) OrganizationResponse {
	if org == nil {
		return OrganizationResponse{}
	}
	return OrganizationResponse{ID: string(org.ID), Slug: org.Slug, Name: org.Name, Kind: org.Kind, Role: role, CreatedAt: org.CreatedAt}
}

// SessionResponse is everything the frontend needs after sign-in: who you are,
// which workspace you are acting in, and which others you can switch to.
type SessionResponse struct {
	User          UserResponse           `json:"user"`
	Organization  OrganizationResponse   `json:"organization"`
	Organizations []OrganizationResponse `json:"organizations"`
}

type SwitchOrganizationResponse struct {
	Token        string               `json:"token"`
	Organization OrganizationResponse `json:"organization"`
}

type MemberResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	JoinedAt  time.Time `json:"joined_at"`
	InvitedBy string    `json:"invited_by,omitempty"`
}

type InvitationResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func NewInvitationResponse(invite *database.Invitation) InvitationResponse {
	role := invite.Role
	if role == "" {
		role = database.MembershipRoleMember
	}
	return InvitationResponse{ID: string(invite.ID), Email: invite.Email, Name: invite.Name, Role: role, Status: invite.Status, ExpiresAt: invite.ExpiresAt, CreatedAt: invite.CreatedAt}
}
