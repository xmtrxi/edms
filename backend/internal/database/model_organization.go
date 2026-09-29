package database

import "time"

// Organization is the unit of ownership: folders, files and settings belong to
// an organization, never to a single person. A personal workspace is an
// organization of kind "personal" with exactly one member.
type Organization struct {
	ID        RecordID       `json:"id,omitempty"`
	Slug      string         `json:"slug"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	OwnerID   string         `json:"owner_id"`
	Settings  map[string]any `json:"settings"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

const (
	OrganizationKindPersonal     = "personal"
	OrganizationKindOrganization = "organization"
)

// Membership links a user to an organization. Role is the coarse org role;
// fine-grained permissions come from Casbin roles in the organization domain.
type Membership struct {
	ID        RecordID  `json:"id,omitempty"`
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	InvitedBy string    `json:"invited_by,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	MembershipRoleOwner  = "owner"
	MembershipRoleAdmin  = "admin"
	MembershipRoleMember = "member"
	MembershipRoleGuest  = "guest"

	MembershipStatusActive    = "active"
	MembershipStatusSuspended = "suspended"
)

// IsMembershipRole reports whether role is one of the coarse org roles.
func IsMembershipRole(role string) bool {
	switch role {
	case MembershipRoleOwner, MembershipRoleAdmin, MembershipRoleMember, MembershipRoleGuest:
		return true
	}
	return false
}

// Group is a cross-department team (a committee, a project) that can receive
// access like a department can.
type Group struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GroupMember struct {
	ID        RecordID  `json:"id,omitempty"`
	OrgID     string    `json:"org_id"`
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
