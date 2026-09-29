package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"slices"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"golang.org/x/crypto/bcrypt"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var (
	ErrSharingDisabled   = errors.New("your organization does not allow this kind of sharing")
	ErrShareNotFound     = errors.New("share not found")
	ErrLinkPassword      = errors.New("this link needs a password")
	ErrRestrictedSharing = errors.New("restricted documents can only be shared with named people inside the organization")
)

// SharePolicy is an organization's rules for sharing outside its grants.
type SharePolicy struct {
	AllowExternal    bool     `json:"allow_external"`
	AllowPublicLinks bool     `json:"allow_public_links"`
	AllowedDomains   []string `json:"allowed_domains"`
	MaxLinkDays      int      `json:"max_link_days"`
}

// DefaultSharePolicy is permissive enough to be useful and safe by default:
// people outside can be invited, but public links must be turned on.
func DefaultSharePolicy() SharePolicy {
	return SharePolicy{AllowExternal: true, AllowPublicLinks: false, AllowedDomains: []string{}, MaxLinkDays: 90}
}

type SharingService struct {
	db     *surrealdb.DB
	access *AccessService
	orgs   *OrganizationService
	smtp   config.SMTPConfig
	log    *slog.Logger
}

func NewSharingService(cfg config.Config, db *surrealdb.DB, logger *slog.Logger) *SharingService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SharingService{db: db, access: NewAccessService(db), orgs: NewOrganizationService(cfg, db, logger), smtp: cfg.SMTP, log: logger}
}

// Policy reads the organization's sharing policy, falling back to defaults.
func (s *SharingService) Policy(ctx context.Context, orgID string) (SharePolicy, error) {
	policy := DefaultSharePolicy()
	org, err := s.orgs.Get(ctx, orgID)
	if err != nil {
		return policy, err
	}
	raw, ok := org.Settings["sharing"].(map[string]any)
	if !ok {
		return policy, nil
	}
	if value, ok := raw["allow_external"].(bool); ok {
		policy.AllowExternal = value
	}
	if value, ok := raw["allow_public_links"].(bool); ok {
		policy.AllowPublicLinks = value
	}
	if values, ok := raw["allowed_domains"].([]any); ok {
		for _, value := range values {
			if domain, ok := value.(string); ok && domain != "" {
				policy.AllowedDomains = append(policy.AllowedDomains, domain)
			}
		}
	}
	switch value := raw["max_link_days"].(type) {
	case float64:
		policy.MaxLinkDays = int(value)
	case int64:
		policy.MaxLinkDays = int(value)
	case uint64:
		policy.MaxLinkDays = int(value)
	case int:
		policy.MaxLinkDays = value
	}
	return policy, nil
}

func (s *SharingService) UpdatePolicy(ctx context.Context, orgID string, policy SharePolicy) (SharePolicy, error) {
	domains := []string{}
	for _, domain := range policy.AllowedDomains {
		domain = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "@"))
		if domain != "" && !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
	}
	policy.AllowedDomains = domains
	if policy.MaxLinkDays <= 0 || policy.MaxLinkDays > 3650 {
		policy.MaxLinkDays = DefaultSharePolicy().MaxLinkDays
	}
	rid, err := database.ParseRecordID(orgID)
	if err != nil {
		return policy, ErrNoOrganization
	}
	_, err = surrealdb.Query[any](ctx, s.db, "UPDATE $org SET settings.sharing = $policy, updated_at = time::now()", map[string]any{
		"org": rid,
		"policy": map[string]any{
			"allow_external": policy.AllowExternal, "allow_public_links": policy.AllowPublicLinks,
			"allowed_domains": policy.AllowedDomains, "max_link_days": policy.MaxLinkDays,
		},
	})
	return policy, err
}

func (p SharePolicy) allowsEmail(email string) bool {
	if !p.AllowExternal {
		return false
	}
	if len(p.AllowedDomains) == 0 {
		return true
	}
	_, domain, _ := strings.Cut(strings.ToLower(email), "@")
	return slices.Contains(p.AllowedDomains, domain)
}

func (p SharePolicy) clampExpiry(expiresAt time.Time) time.Time {
	limit := time.Now().UTC().Add(time.Duration(p.MaxLinkDays) * 24 * time.Hour)
	if expiresAt.IsZero() || expiresAt.After(limit) {
		return limit
	}
	return expiresAt
}

// document loads a document the actor holds capability on.
func (s *SharingService) document(ctx context.Context, userID, capability, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrDocumentNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" {
		return nil, ErrDocumentNotFound
	}
	allowed, err := s.access.CanDocument(ctx, userID, capability, doc)
	if err != nil {
		return nil, err
	}
	if !allowed {
		if visible, _ := s.access.CanDocument(ctx, userID, "view", doc); visible {
			return nil, ErrForbidden
		}
		return nil, ErrDocumentNotFound
	}
	return doc, nil
}

// ShareView is a share as shown in the share dialog.
type ShareView struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	UserID        string    `json:"user_id,omitempty"`
	Email         string    `json:"email,omitempty"`
	Name          string    `json:"name,omitempty"`
	Role          string    `json:"role"`
	LinkScope     string    `json:"link_scope,omitempty"`
	HasPassword   bool      `json:"has_password"`
	AllowDownload bool      `json:"allow_download"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// DocumentAccess is everything the share dialog shows for one document.
type DocumentAccess struct {
	DocumentID         string      `json:"document_id"`
	OwnerID            string      `json:"owner_id"`
	OwnerName          string      `json:"owner_name"`
	FolderID           string      `json:"folder_id,omitempty"`
	FolderName         string      `json:"folder_name,omitempty"`
	InheritPermissions bool        `json:"inherit_permissions"`
	Sensitivity        string      `json:"sensitivity"`
	Inherited          []GrantView `json:"inherited"`
	Grants             []GrantView `json:"grants"`
	Shares             []ShareView `json:"shares"`
	Links              []ShareView `json:"links"`
	Policy             SharePolicy `json:"policy"`
	CanShare           bool        `json:"can_share"`
	PendingRequests    int         `json:"pending_requests"`
}

// Access describes who can reach a document. Anyone who can view it can see
// the list; changing it needs the share capability.
func (s *SharingService) Access(ctx context.Context, userID, documentID string, principals *PrincipalService) (*DocumentAccess, error) {
	doc, err := s.document(ctx, userID, "view", documentID)
	if err != nil {
		return nil, err
	}
	canShare, err := s.access.CanDocument(ctx, userID, "share", doc)
	if err != nil {
		return nil, err
	}
	policy, _ := s.Policy(ctx, doc.OrgID)
	result := &DocumentAccess{
		DocumentID: string(doc.ID), OwnerID: doc.OwnerID, FolderID: doc.FolderID,
		InheritPermissions: doc.InheritPermissions, Sensitivity: firstNonEmpty(doc.Sensitivity, database.SensitivityInternal),
		Inherited: []GrantView{}, Grants: []GrantView{}, Shares: []ShareView{}, Links: []ShareView{},
		Policy: policy, CanShare: canShare,
	}
	if owner, err := s.orgs.userByID(ctx, doc.OwnerID); err == nil && owner != nil {
		result.OwnerName = firstNonEmpty(owner.Name, owner.Email)
	}
	if doc.FolderID != "" {
		if folder, err := s.access.folder(ctx, doc.FolderID); err == nil && folder != nil {
			result.FolderName = folder.Name
			if doc.InheritPermissions && doc.Sensitivity != database.SensitivityRestricted {
				inherited, err := NewFolderService(s.db).ListGrants(ctx, userID, doc.FolderID)
				if err == nil {
					for i := range inherited {
						inherited[i].Source = "inherited"
					}
					result.Inherited = principals.Describe(ctx, doc.OrgID, inherited)
				}
			}
		}
	}
	grants, err := queryRows[database.DocumentGrant](ctx, s.db, "SELECT * FROM document_grants WHERE document_id = $document ORDER BY created_at", map[string]any{"document": documentID})
	if err != nil {
		return nil, err
	}
	for _, grant := range grants {
		result.Grants = append(result.Grants, GrantView{ID: string(grant.ID), PrincipalType: grant.PrincipalType, PrincipalID: grant.PrincipalID, Role: grant.Role, Allow: grant.Allow, Deny: grant.Deny, Source: "document"})
	}
	result.Grants = principals.Describe(ctx, doc.OrgID, result.Grants)

	shares, err := s.activeShares(ctx, documentID)
	if err != nil {
		return nil, err
	}
	for _, share := range shares {
		view := ShareView{ID: string(share.ID), Kind: share.Kind, UserID: share.SharedWithUserID, Email: share.SharedWithEmail,
			Role: shareRole(share), LinkScope: share.LinkScope, HasPassword: share.PasswordHash != "", AllowDownload: share.AllowDownload,
			ExpiresAt: share.ExpiresAt, CreatedAt: share.CreatedAt}
		if share.Kind == database.ShareKindLink {
			result.Links = append(result.Links, view)
			continue
		}
		if user, err := s.orgs.userByID(ctx, share.SharedWithUserID); err == nil && user != nil {
			view.Name = user.Name
		}
		result.Shares = append(result.Shares, view)
	}
	if canShare {
		result.PendingRequests, _ = queryCount(ctx, s.db, "SELECT count() AS count FROM access_requests WHERE document_id = $document AND status = 'pending' GROUP ALL", map[string]any{"document": documentID})
	}
	return result, nil
}

func shareRole(share database.Share) string {
	switch share.Permission {
	case database.ShareRoleEditor, database.ShareRoleCommenter:
		return share.Permission
	}
	return database.ShareRoleViewer
}

func (s *SharingService) activeShares(ctx context.Context, documentID string) ([]database.Share, error) {
	return queryRows[database.Share](ctx, s.db,
		"SELECT * FROM shares WHERE document_id = $document AND "+activeShareClause+" ORDER BY created_at",
		map[string]any{"document": documentID, "now": time.Now().UTC()})
}

// activeShareClause matches shares that are neither revoked nor expired.
// Older rows stored an empty revoked_at as a zero time instead of NONE.
const activeShareClause = "(revoked_at = NONE OR revoked_at = NULL OR revoked_at < created_at) AND expires_at > $now"

// UpdateSettings changes whether the document follows its folder and its label.
func (s *SharingService) UpdateSettings(ctx context.Context, userID, documentID string, inherit *bool, sensitivity string) (*database.Document, error) {
	doc, err := s.document(ctx, userID, "share", documentID)
	if err != nil {
		return nil, err
	}
	changes := map[string]any{"updated_at": time.Now().UTC()}
	if sensitivity != "" {
		if !database.IsSensitivity(sensitivity) {
			return nil, fmt.Errorf("unknown sensitivity %q", sensitivity)
		}
		changes["sensitivity"] = sensitivity
		if sensitivity == database.SensitivityRestricted {
			changes["inherit_permissions"] = false
			// Admins cannot bypass restricted documents, so whoever restricts one
			// keeps access through a named grant.
			if doc.OwnerID != userID {
				if _, err := upsertDocumentGrant(ctx, s.db, doc, userID, GrantInput{PrincipalType: database.PrincipalUser, PrincipalID: userID, Role: "manager"}); err != nil {
					return nil, err
				}
			}
			// Broad grants and links stop working on restricted documents.
			if _, err := surrealdb.Query[any](ctx, s.db, "DELETE document_grants WHERE document_id = $document AND principal_type NOT IN ['user', 'group']", map[string]any{"document": documentID}); err != nil {
				return nil, err
			}
			if _, err := surrealdb.Query[any](ctx, s.db, "UPDATE shares SET revoked_at = time::now() WHERE document_id = $document AND kind = 'link'", map[string]any{"document": documentID}); err != nil {
				return nil, err
			}
		}
	}
	if inherit != nil && (sensitivity != database.SensitivityRestricted || !*inherit) {
		if *inherit && firstNonEmpty(sensitivity, doc.Sensitivity) == database.SensitivityRestricted {
			return nil, ErrRestrictedSharing
		}
		changes["inherit_permissions"] = *inherit
	}
	rid, _ := database.ParseRecordID(documentID)
	updated, err := surrealdb.Merge[database.Document](ctx, s.db, rid, changes)
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "document.access_settings_changed", "document", documentID, changes)
	}
	return updated, err
}

func (s *SharingService) UpsertGrant(ctx context.Context, userID, documentID string, in GrantInput) (*database.DocumentGrant, error) {
	doc, err := s.document(ctx, userID, "share", documentID)
	if err != nil {
		return nil, err
	}
	grant, err := upsertDocumentGrant(ctx, s.db, doc, userID, in)
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "document.shared", "document", documentID, map[string]any{"principal_type": grant.PrincipalType, "principal_id": grant.PrincipalID, "role": grant.Role})
	}
	return grant, err
}

func (s *SharingService) RemoveGrant(ctx context.Context, userID, documentID, grantID string) error {
	if _, err := s.document(ctx, userID, "share", documentID); err != nil {
		return err
	}
	rid, err := database.ParseRecordID(grantID)
	if err != nil {
		return ErrShareNotFound
	}
	grant, err := surrealdb.Select[database.DocumentGrant](ctx, s.db, rid)
	if err != nil || grant == nil || grant.DocumentID != documentID {
		return ErrShareNotFound
	}
	if _, err := surrealdb.Delete[database.DocumentGrant](ctx, s.db, rid); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.db, userID, "document.unshared", "document", documentID, map[string]any{"principal_type": grant.PrincipalType, "principal_id": grant.PrincipalID})
	return nil
}

// PersonShareInput invites someone by email.
type PersonShareInput struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at"`
	Notify    bool      `json:"notify"`
}

// ShareResult tells the caller what sharing with an email address did.
type ShareResult struct {
	// Kind is "grant" for members of the organization, "user" for people with
	// an account elsewhere, and "email" for people without an account.
	Kind  string                  `json:"kind"`
	Grant *database.DocumentGrant `json:"grant,omitempty"`
	Share *ShareView              `json:"share,omitempty"`
	// Link is only returned for email shares, so it can be sent another way.
	Link string `json:"link,omitempty"`
}

// ShareWithPerson gives one person access. Organization members get a
// document grant; anyone else gets a share, subject to the sharing policy.
func (s *SharingService) ShareWithPerson(ctx context.Context, userID, documentID string, in PersonShareInput) (*ShareResult, error) {
	doc, err := s.document(ctx, userID, "share", documentID)
	if err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(email, "@") {
		return nil, fmt.Errorf("a valid email address is required")
	}
	role := in.Role
	if role != database.ShareRoleCommenter && role != database.ShareRoleEditor {
		role = database.ShareRoleViewer
	}
	recipient, err := s.orgs.userByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if recipient != nil {
		if _, err := s.orgs.Membership(ctx, doc.OrgID, string(recipient.ID)); err == nil {
			grant, err := upsertDocumentGrant(ctx, s.db, doc, userID, GrantInput{PrincipalType: database.PrincipalUser, PrincipalID: string(recipient.ID), Role: grantRoleForShare(role)})
			if err != nil {
				return nil, err
			}
			_ = writeAudit(ctx, s.db, userID, "document.shared", "document", documentID, map[string]any{"recipient": email, "role": role})
			if in.Notify {
				s.notify(ctx, email, doc, userID, in.Message, s.documentURL(doc))
			}
			return &ShareResult{Kind: "grant", Grant: grant}, nil
		}
	}

	if doc.Sensitivity == database.SensitivityRestricted {
		return nil, ErrRestrictedSharing
	}
	policy, err := s.Policy(ctx, doc.OrgID)
	if err != nil {
		return nil, err
	}
	if !policy.allowsEmail(email) {
		return nil, ErrSharingDisabled
	}
	kind := database.ShareKindEmail
	recipientID := ""
	if recipient != nil {
		kind, recipientID = database.ShareKindUser, string(recipient.ID)
	}
	if _, err := surrealdb.Query[any](ctx, s.db,
		"UPDATE shares SET revoked_at = time::now() WHERE document_id = $document AND kind != 'link' AND shared_with_email = $email AND (revoked_at = NONE OR revoked_at = NULL OR revoked_at < created_at)",
		map[string]any{"document": documentID, "email": email}); err != nil {
		return nil, err
	}
	token, err := newShareToken()
	if err != nil {
		return nil, err
	}
	expiresAt := policy.clampExpiry(in.ExpiresAt)
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": doc.OrgID, "document_id": documentID, "created_by": userID, "kind": kind,
		"shared_with_email": email, "permission": role, "allow_download": true,
		"token_hash": secretHash(token), "expires_at": expiresAt, "created_at": now,
	}
	if recipientID != "" {
		record["shared_with_user_id"] = recipientID
	}
	share, err := surrealdb.Create[database.Share](ctx, s.db, "shares", record)
	if err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, userID, "document.shared_externally", "document", documentID, map[string]any{"recipient": email, "role": role, "kind": kind})
	link := ""
	openURL := s.documentURL(doc)
	if kind == database.ShareKindEmail {
		link = s.linkURL(token)
		openURL = link
	}
	if in.Notify {
		s.notify(ctx, email, doc, userID, in.Message, openURL)
	}
	view := ShareView{ID: string(share.ID), Kind: kind, UserID: recipientID, Email: email, Role: role, ExpiresAt: expiresAt, CreatedAt: now, AllowDownload: true}
	return &ShareResult{Kind: kind, Share: &view, Link: link}, nil
}

func grantRoleForShare(role string) string {
	switch role {
	case database.ShareRoleEditor:
		return "editor"
	case database.ShareRoleCommenter:
		return "commenter"
	}
	return "viewer"
}

// LinkInput configures a share link.
type LinkInput struct {
	Scope         string    `json:"scope"`
	Role          string    `json:"role"`
	AllowDownload bool      `json:"allow_download"`
	Password      string    `json:"password"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// CreateLink replaces the document's link of the same scope with a new one.
func (s *SharingService) CreateLink(ctx context.Context, userID, documentID string, in LinkInput) (*ShareView, string, error) {
	doc, err := s.document(ctx, userID, "share", documentID)
	if err != nil {
		return nil, "", err
	}
	if doc.Sensitivity == database.SensitivityRestricted {
		return nil, "", ErrRestrictedSharing
	}
	if in.Scope != database.LinkScopeAnyone {
		in.Scope = database.LinkScopeOrganization
	}
	policy, err := s.Policy(ctx, doc.OrgID)
	if err != nil {
		return nil, "", err
	}
	if in.Scope == database.LinkScopeAnyone && (!policy.AllowPublicLinks || doc.Sensitivity == database.SensitivityConfidential) {
		return nil, "", ErrSharingDisabled
	}
	role := in.Role
	if role != database.ShareRoleCommenter && role != database.ShareRoleEditor {
		role = database.ShareRoleViewer
	}
	if in.Scope == database.LinkScopeAnyone && role != database.ShareRoleViewer {
		return nil, "", fmt.Errorf("public links can only give view access")
	}
	if _, err := surrealdb.Query[any](ctx, s.db,
		"UPDATE shares SET revoked_at = time::now() WHERE document_id = $document AND kind = 'link' AND link_scope = $scope AND (revoked_at = NONE OR revoked_at = NULL OR revoked_at < created_at)",
		map[string]any{"document": documentID, "scope": in.Scope}); err != nil {
		return nil, "", err
	}
	token, err := newShareToken()
	if err != nil {
		return nil, "", err
	}
	expiresAt := policy.clampExpiry(in.ExpiresAt)
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": doc.OrgID, "document_id": documentID, "created_by": userID, "kind": database.ShareKindLink,
		"shared_with_email": "", "permission": role, "link_scope": in.Scope, "allow_download": in.AllowDownload,
		"token_hash": secretHash(token), "expires_at": expiresAt, "created_at": now,
	}
	if in.Password != "" {
		if len(in.Password) < 6 {
			return nil, "", fmt.Errorf("link passwords need at least 6 characters")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", err
		}
		record["password_hash"] = string(hash)
	}
	share, err := surrealdb.Create[database.Share](ctx, s.db, "shares", record)
	if err != nil {
		return nil, "", err
	}
	_ = writeAudit(ctx, s.db, userID, "document.link_created", "document", documentID, map[string]any{"scope": in.Scope, "role": role})
	view := &ShareView{ID: string(share.ID), Kind: database.ShareKindLink, Role: role, LinkScope: in.Scope, HasPassword: in.Password != "", AllowDownload: in.AllowDownload, ExpiresAt: expiresAt, CreatedAt: now}
	return view, s.linkURL(token), nil
}

// UpdateShareRole changes what a person or link share allows without issuing
// a new token, so links already sent keep working.
func (s *SharingService) UpdateShareRole(ctx context.Context, userID, documentID, shareID, role string) error {
	if _, err := s.document(ctx, userID, "share", documentID); err != nil {
		return err
	}
	if role != database.ShareRoleViewer && role != database.ShareRoleCommenter && role != database.ShareRoleEditor {
		return fmt.Errorf("choose viewer, commenter or editor")
	}
	rid, err := database.ParseRecordID(shareID)
	if err != nil {
		return ErrShareNotFound
	}
	share, err := surrealdb.Select[database.Share](ctx, s.db, rid)
	if err != nil || share == nil || share.DocumentID != documentID {
		return ErrShareNotFound
	}
	if share.Kind == database.ShareKindLink && share.LinkScope == database.LinkScopeAnyone && role != database.ShareRoleViewer {
		return fmt.Errorf("public links can only give view access")
	}
	if _, err := surrealdb.Query[any](ctx, s.db, "UPDATE $share SET permission = $role", map[string]any{"share": rid, "role": role}); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.db, userID, "document.share_role_changed", "document", documentID, map[string]any{"share_id": shareID, "role": role})
	return nil
}

func (s *SharingService) Revoke(ctx context.Context, userID, documentID, shareID string) error {
	if _, err := s.document(ctx, userID, "share", documentID); err != nil {
		return err
	}
	rid, err := database.ParseRecordID(shareID)
	if err != nil {
		return ErrShareNotFound
	}
	share, err := surrealdb.Select[database.Share](ctx, s.db, rid)
	if err != nil || share == nil || share.DocumentID != documentID {
		return ErrShareNotFound
	}
	if _, err := surrealdb.Query[any](ctx, s.db, "UPDATE $share SET revoked_at = time::now()", map[string]any{"share": rid}); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.db, userID, "document.share_revoked", "document", documentID, map[string]any{"share_id": shareID})
	return nil
}

// LinkDocument is what a link opens.
type LinkDocument struct {
	Document *database.Document
	Share    *database.Share
}

// OpenLink resolves a share token. Organization links need a signed-in member
// of the document's organization; email links are bound to the recipient
// when they have an account.
func (s *SharingService) OpenLink(ctx context.Context, token, password string, viewer *database.User) (*LinkDocument, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrShareNotFound
	}
	shares, err := queryRows[database.Share](ctx, s.db,
		"SELECT * FROM shares WHERE token_hash = $hash AND "+activeShareClause+" LIMIT 1",
		map[string]any{"hash": secretHash(token), "now": time.Now().UTC()})
	if err != nil {
		return nil, err
	}
	if len(shares) == 0 {
		return nil, ErrShareNotFound
	}
	share := shares[0]
	rid, err := database.ParseRecordID(share.DocumentID)
	if err != nil {
		return nil, ErrShareNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" || doc.Status == database.DocumentStatusTrashed || doc.Status == database.DocumentStatusArchived {
		return nil, ErrShareNotFound
	}
	if share.Kind == database.ShareKindLink && share.LinkScope == database.LinkScopeOrganization {
		if viewer == nil {
			return nil, ErrAccessDenied
		}
		p, err := s.access.PrincipalsFor(ctx, string(viewer.ID), doc.OrgID)
		if err != nil || !p.Member {
			return nil, ErrAccessDenied
		}
	}
	if share.Kind == database.ShareKindLink && doc.Sensitivity == database.SensitivityRestricted {
		return nil, ErrShareNotFound
	}
	if share.PasswordHash != "" {
		if password == "" {
			return nil, ErrLinkPassword
		}
		if bcrypt.CompareHashAndPassword([]byte(share.PasswordHash), []byte(password)) != nil {
			return nil, ErrLinkPassword
		}
	}
	return &LinkDocument{Document: doc, Share: &share}, nil
}

// LinkAllows reports whether an opened link permits capability.
func LinkAllows(share *database.Share, capability string) bool {
	return share != nil && slices.Contains(shareCapabilities(*share), capability)
}

// SharedWithMe lists documents shared directly with the user, across all
// organizations, plus documents granted to them by name in their organization.
func (s *SharingService) SharedWithMe(ctx context.Context, user *database.User) ([]database.Document, error) {
	shares, err := queryRows[database.Share](ctx, s.db,
		"SELECT * FROM shares WHERE kind != 'link' AND (shared_with_user_id = $user OR shared_with_email = $email) AND "+activeShareClause,
		map[string]any{"user": string(user.ID), "email": strings.ToLower(user.Email), "now": time.Now().UTC()})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, share := range shares {
		ids = append(ids, share.DocumentID)
	}
	if orgID := orgOf(ctx); orgID != "" {
		grants, err := queryRows[database.DocumentGrant](ctx, s.db,
			"SELECT * FROM document_grants WHERE org_id = $org AND principal_type = 'user' AND principal_id = $user",
			map[string]any{"org": orgID, "user": string(user.ID)})
		if err != nil {
			return nil, err
		}
		for _, grant := range grants {
			ids = append(ids, grant.DocumentID)
		}
	}
	items := []database.Document{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		rid, err := database.ParseRecordID(id)
		if err != nil {
			continue
		}
		doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
		if err != nil || doc == nil || doc.ID == "" || doc.OwnerID == string(user.ID) || doc.Status == database.DocumentStatusTrashed {
			continue
		}
		if allowed, err := s.access.CanDocument(ctx, string(user.ID), "view", doc); err == nil && allowed {
			items = append(items, *doc)
		}
	}
	return items, nil
}

// RequestAccess records a request from a member who cannot see the document.
func (s *SharingService) RequestAccess(ctx context.Context, userID, documentID, role, message string) (*database.AccessRequest, error) {
	rid, err := database.ParseRecordID(documentID)
	if err != nil {
		return nil, ErrDocumentNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" {
		return nil, ErrDocumentNotFound
	}
	p, err := s.access.PrincipalsFor(ctx, userID, doc.OrgID)
	if err != nil || !p.Member {
		return nil, ErrDocumentNotFound
	}
	if allowed, _ := s.access.CanDocument(ctx, userID, "view", doc); allowed && role != "editor" {
		return nil, fmt.Errorf("you already have access to this document")
	}
	if role != "editor" && role != "commenter" {
		role = "viewer"
	}
	pending, err := queryRows[database.AccessRequest](ctx, s.db,
		"SELECT * FROM access_requests WHERE document_id = $document AND requester_id = $user AND status = 'pending' LIMIT 1",
		map[string]any{"document": documentID, "user": userID})
	if err != nil {
		return nil, err
	}
	if len(pending) > 0 {
		return &pending[0], nil
	}
	now := time.Now().UTC()
	request, err := surrealdb.Create[database.AccessRequest](ctx, s.db, "access_requests", &database.AccessRequest{
		OrgID: doc.OrgID, DocumentID: documentID, RequesterID: userID, Role: role, Message: strings.TrimSpace(message),
		Status: database.AccessRequestPending, CreatedAt: now, UpdatedAt: now,
	})
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "document.access_requested", "document", documentID, map[string]any{"role": role})
	}
	return request, err
}

// AccessRequestView adds names for the requests inbox.
type AccessRequestView struct {
	database.AccessRequest
	RequesterName  string `json:"requester_name"`
	RequesterEmail string `json:"requester_email"`
	DocumentName   string `json:"document_name"`
}

// AccessRequests lists pending requests on documents the user can share.
func (s *SharingService) AccessRequests(ctx context.Context, userID string) ([]AccessRequestView, error) {
	requests, err := queryRows[database.AccessRequest](ctx, s.db,
		"SELECT * FROM access_requests WHERE org_id = $org AND status = 'pending' ORDER BY created_at DESC LIMIT 200",
		map[string]any{"org": orgOf(ctx)})
	if err != nil {
		return nil, err
	}
	views := []AccessRequestView{}
	for _, request := range requests {
		doc, err := s.document(ctx, userID, "share", request.DocumentID)
		if err != nil {
			continue
		}
		view := AccessRequestView{AccessRequest: request, DocumentName: doc.Name}
		if user, err := s.orgs.userByID(ctx, request.RequesterID); err == nil && user != nil {
			view.RequesterName, view.RequesterEmail = user.Name, user.Email
		}
		views = append(views, view)
	}
	return views, nil
}

// DecideAccessRequest approves (granting the requested or chosen role) or denies.
func (s *SharingService) DecideAccessRequest(ctx context.Context, userID, requestID string, approve bool, role string) (*database.AccessRequest, error) {
	rid, err := database.ParseRecordID(requestID)
	if err != nil {
		return nil, ErrShareNotFound
	}
	request, err := surrealdb.Select[database.AccessRequest](ctx, s.db, rid)
	if err != nil || request == nil || request.ID == "" || request.Status != database.AccessRequestPending {
		return nil, ErrShareNotFound
	}
	doc, err := s.document(ctx, userID, "share", request.DocumentID)
	if err != nil {
		return nil, err
	}
	status := database.AccessRequestDenied
	if approve {
		if role == "" {
			role = request.Role
		}
		if _, err := upsertDocumentGrant(ctx, s.db, doc, userID, GrantInput{PrincipalType: database.PrincipalUser, PrincipalID: request.RequesterID, Role: grantRoleForShare(role)}); err != nil {
			return nil, err
		}
		status = database.AccessRequestApproved
	}
	updated, err := surrealdb.Merge[database.AccessRequest](ctx, s.db, rid, map[string]any{"status": status, "decided_by": userID, "updated_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "document.access_request_"+status, "document", request.DocumentID, map[string]any{"requester": request.RequesterID})
	}
	return updated, err
}

func (s *SharingService) documentURL(doc *database.Document) string {
	return strings.TrimRight(s.smtp.AppURL, "/") + "/documents/" + string(doc.ID)
}

func (s *SharingService) linkURL(token string) string {
	return strings.TrimRight(s.smtp.AppURL, "/") + "/s/" + token
}

func (s *SharingService) notify(ctx context.Context, email string, doc *database.Document, senderID, message, url string) {
	if s.smtp.Host == "" || s.smtp.From == "" {
		return
	}
	sender := "Someone"
	if user, err := s.orgs.userByID(ctx, senderID); err == nil && user != nil {
		sender = firstNonEmpty(user.Name, user.Email)
	}
	body := fmt.Sprintf("To: %s\r\nSubject: %s shared \"%s\" with you\r\n\r\n%s shared \"%s\" with you.\r\n", email, sender, doc.Name, sender, doc.Name)
	if strings.TrimSpace(message) != "" {
		body += "\r\n" + strings.TrimSpace(message) + "\r\n"
	}
	body += "\r\nOpen it: " + url + "\r\n"
	auth := smtp.PlainAuth("", s.smtp.Username, s.smtp.Password, s.smtp.Host)
	if err := smtp.SendMail(s.smtp.Host+":"+s.smtp.Port, auth, s.smtp.From, []string{email}, []byte(body)); err != nil {
		s.log.Warn("send share notification", "email", email, "error", err)
	}
}

func newShareToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
