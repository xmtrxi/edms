package database

import "time"

// Share gives one document to someone outside the normal grant model: a
// person (possibly in another organization), an email address without an
// account, or anyone holding a link.
type Share struct {
	ID               RecordID `json:"id,omitempty"`
	OrgID            string   `json:"org_id"`
	DocumentID       string   `json:"document_id"`
	CreatedBy        string   `json:"created_by"`
	Kind             string   `json:"kind"`
	SharedWithUserID string   `json:"shared_with_user_id,omitempty"`
	SharedWithEmail  string   `json:"shared_with_email"`
	// Permission is the share role: viewer, commenter or editor. Older rows
	// hold "view" or "download", which both read as viewer.
	Permission string `json:"permission"`
	LinkScope  string `json:"link_scope,omitempty"`
	// PasswordHash must keep a real tag: the database driver decodes by JSON
	// tag, and "-" would silently drop the hash. Shares are only returned to
	// clients through service.ShareView, which never includes it.
	PasswordHash  string    `json:"password_hash,omitempty"`
	AllowDownload bool      `json:"allow_download"`
	TokenHash     string    `json:"token_hash"`
	ExpiresAt     time.Time `json:"expires_at"`
	RevokedAt     time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

const (
	ShareKindUser  = "user"
	ShareKindEmail = "email"
	ShareKindLink  = "link"

	ShareRoleViewer    = "viewer"
	ShareRoleCommenter = "commenter"
	ShareRoleEditor    = "editor"

	LinkScopeOrganization = "organization"
	LinkScopeAnyone       = "anyone"
)

// AccessRequest is raised by someone who opened a document they cannot see.
type AccessRequest struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	DocumentID  string    `json:"document_id"`
	RequesterID string    `json:"requester_id"`
	Role        string    `json:"role"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
	DecidedBy   string    `json:"decided_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const (
	AccessRequestPending  = "pending"
	AccessRequestApproved = "approved"
	AccessRequestDenied   = "denied"
)
