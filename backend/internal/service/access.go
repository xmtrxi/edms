package service

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

var ErrAccessDenied = errors.New("forbidden")

type AccessService struct{ DB *surrealdb.DB }

func NewAccessService(db *surrealdb.DB) *AccessService { return &AccessService{DB: db} }

var roleCapabilities = map[string][]string{
	"manager":     {"view", "download", "upload", "edit", "comment", "route", "sign", "share", "move", "delete", "manage_folder", "create_version"},
	"editor":      {"view", "download", "upload", "edit", "comment", "create_version"},
	"contributor": {"view", "download", "upload", "comment"},
	"commenter":   {"view", "download", "comment"},
	"viewer":      {"view", "download"},
	"uploader":    {"upload"},
}

// IsGrantRole reports whether role can be used in a folder or document grant.
func IsGrantRole(role string) bool {
	_, ok := roleCapabilities[role]
	return ok
}

// Principals is everything a grant can name that applies to one user inside
// one organization: themselves, their departments (heads also cover every
// sub-department), their groups, their Casbin roles and "everyone".
type Principals struct {
	UserID string
	OrgID  string
	Admin  bool
	Guest  bool
	// Member is false when the user does not belong to OrgID at all.
	Member bool
	keys   map[string]bool
}

func principalKey(kind, id string) string { return kind + "|" + id }

// Has reports whether the grant principal applies to these principals.
func (p *Principals) Has(kind, id string) bool {
	if p == nil {
		return false
	}
	return p.keys[principalKey(kind, id)]
}

// Keys lists the principal keys, for filtering grants in queries.
func (p *Principals) Keys() []string {
	keys := make([]string, 0, len(p.keys))
	for key := range p.keys {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// named reports whether a grant names specific people (a user or a group),
// the only kind allowed to open restricted documents.
func named(kind string) bool {
	return kind == database.PrincipalUser || kind == database.PrincipalGroup
}

// PrincipalsFor resolves userID's principals in orgID. The result is memoized
// on the request context when it belongs to the signed-in actor.
func (s *AccessService) PrincipalsFor(ctx context.Context, userID, orgID string) (*Principals, error) {
	if state, ok := ctx.Value(actorKey{}).(*actorState); ok && state != nil && state.actor.UserID == userID && state.actor.OrgID == orgID {
		state.once.Do(func() {
			state.principals, state.err = s.resolvePrincipals(ctx, userID, orgID, state.actor.OrgRole)
		})
		return state.principals, state.err
	}
	return s.resolvePrincipals(ctx, userID, orgID, "")
}

func (s *AccessService) resolvePrincipals(ctx context.Context, userID, orgID, role string) (*Principals, error) {
	p := &Principals{UserID: userID, OrgID: orgID, keys: map[string]bool{principalKey(database.PrincipalUser, userID): true}}
	if orgID == "" || userID == "" {
		return p, nil
	}
	if role == "" {
		memberships, err := queryRows[database.Membership](ctx, s.DB,
			"SELECT * FROM memberships WHERE org_id = $org AND user_id = $user AND status = $status LIMIT 1",
			map[string]any{"org": orgID, "user": userID, "status": database.MembershipStatusActive})
		if err != nil {
			return nil, err
		}
		if len(memberships) == 0 {
			return p, nil
		}
		role = memberships[0].Role
	}
	p.Member = true
	p.Admin = role == database.MembershipRoleOwner || role == database.MembershipRoleAdmin
	p.Guest = role == database.MembershipRoleGuest
	if !p.Guest {
		p.keys[principalKey(database.PrincipalEveryone, database.EveryonePrincipalID)] = true
	}

	vars := map[string]any{"org": orgID, "user": userID}
	memberships, err := queryRows[database.DepartmentMember](ctx, s.DB, "SELECT * FROM department_members WHERE org_id = $org AND user_id = $user", vars)
	if err != nil {
		return nil, err
	}
	heads := []string{}
	for _, membership := range memberships {
		p.keys[principalKey(database.PrincipalDepartment, membership.DepartmentID)] = true
		if membership.Role == "head" {
			heads = append(heads, membership.DepartmentID)
		}
	}
	if len(heads) > 0 {
		departments, err := queryRows[database.Department](ctx, s.DB, "SELECT * FROM departments WHERE org_id = $org", vars)
		if err != nil {
			return nil, err
		}
		for _, id := range descendantDepartments(departments, heads) {
			p.keys[principalKey(database.PrincipalDepartment, id)] = true
		}
	}

	groups, err := queryRows[database.GroupMember](ctx, s.DB, "SELECT * FROM group_members WHERE org_id = $org AND user_id = $user", vars)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		p.keys[principalKey(database.PrincipalGroup, group.GroupID)] = true
	}

	if authz, err := NewAuthorizationService(s.DB); err == nil {
		for _, role := range authz.RolesFor(userID, orgID) {
			p.keys[principalKey(database.PrincipalRole, role)] = true
		}
	}
	return p, nil
}

// descendantDepartments returns every department below the given roots.
func descendantDepartments(departments []database.Department, roots []string) []string {
	children := map[string][]string{}
	for _, department := range departments {
		if department.ParentID != "" {
			children[department.ParentID] = append(children[department.ParentID], string(department.ID))
		}
	}
	seen := map[string]bool{}
	queue := append([]string{}, roots...)
	result := []string{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range children[current] {
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
				queue = append(queue, child)
			}
		}
	}
	return result
}

// actorPrincipals resolves principals for userID in orgID, preferring the
// request's actor when it matches.
func (s *AccessService) actorPrincipals(ctx context.Context, userID, orgID string) (*Principals, error) {
	if orgID == "" {
		orgID = orgOf(ctx)
	}
	return s.PrincipalsFor(ctx, userID, orgID)
}

func (s *AccessService) RequireFolder(ctx context.Context, userID, capability, folderID string) error {
	ok, err := s.CanFolder(ctx, userID, capability, folderID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// CanDocument decides whether userID may perform capability on document.
//
// Order: organization membership, restricted label, admin, creator, the
// document's own grants (deny wins), the folder chain when the document
// inherits, then direct shares, which also work across organizations.
func (s *AccessService) CanDocument(ctx context.Context, userID, capability string, document *database.Document) (bool, error) {
	if document == nil {
		return false, ErrDocumentNotFound
	}
	orgID := document.OrgID
	if orgID == "" {
		orgID = orgOf(ctx)
	}
	p, err := s.actorPrincipals(ctx, userID, orgID)
	if err != nil {
		return false, err
	}
	if p.Member {
		decided, allowed, err := s.memberDecision(ctx, p, capability, document)
		if err != nil || decided {
			return allowed, err
		}
	}
	return s.shareAllows(ctx, userID, capability, string(document.ID))
}

func (s *AccessService) memberDecision(ctx context.Context, p *Principals, capability string, document *database.Document) (decided, allowed bool, err error) {
	restricted := document.Sensitivity == database.SensitivityRestricted
	if p.Admin && !restricted {
		return true, true, nil
	}
	if document.OwnerID == p.UserID {
		return true, true, nil
	}
	grants, err := queryRows[database.DocumentGrant](ctx, s.DB, "SELECT * FROM document_grants WHERE document_id = $document", map[string]any{"document": string(document.ID)})
	if err != nil {
		return false, false, err
	}
	applicable := make([]grantView, 0, len(grants))
	for _, grant := range grants {
		if p.Has(grant.PrincipalType, grant.PrincipalID) && (!restricted || named(grant.PrincipalType)) {
			applicable = append(applicable, grantView{grant.Role, grant.Allow, grant.Deny})
		}
	}
	if decision, found := decide(applicable, capability); found {
		return true, decision, nil
	}
	if restricted || !document.InheritPermissions || document.FolderID == "" {
		return false, false, nil
	}
	folderAllowed, err := s.canFolderWith(ctx, p, capability, document.FolderID)
	if err != nil || folderAllowed {
		return true, folderAllowed, err
	}
	return false, false, nil
}

// documentCapabilities is every capability the UI can offer on a document.
var documentCapabilities = []string{"view", "download", "comment", "edit", "create_version", "share", "move", "delete", "route", "sign"}

// Capabilities lists what userID may do with document, for the UI.
func (s *AccessService) Capabilities(ctx context.Context, userID string, document *database.Document) ([]string, error) {
	result := []string{}
	for _, capability := range documentCapabilities {
		allowed, err := s.CanDocument(ctx, userID, capability, document)
		if err != nil {
			return nil, err
		}
		if allowed {
			result = append(result, capability)
		}
	}
	return result, nil
}

// shareCapabilities maps a share role to what it allows.
func shareCapabilities(share database.Share) []string {
	var capabilities []string
	switch share.Permission {
	case database.ShareRoleEditor:
		capabilities = []string{"view", "comment", "edit", "create_version"}
	case database.ShareRoleCommenter:
		capabilities = []string{"view", "comment"}
	default:
		capabilities = []string{"view"}
	}
	if share.AllowDownload || share.Permission == "download" || share.Permission == database.ShareRoleEditor {
		capabilities = append(capabilities, "download")
	}
	return capabilities
}

// shareAllows checks direct person shares and organization-wide links.
func (s *AccessService) shareAllows(ctx context.Context, userID, capability, documentID string) (bool, error) {
	shares, err := queryRows[database.Share](ctx, s.DB,
		"SELECT * FROM shares WHERE document_id = $document AND "+activeShareClause,
		map[string]any{"document": documentID, "now": time.Now().UTC()})
	if err != nil {
		return false, err
	}
	for _, share := range shares {
		applies := share.SharedWithUserID == userID
		if !applies && share.Kind == database.ShareKindLink && share.LinkScope == database.LinkScopeOrganization {
			// Organization links open for any member of the document's organization.
			p, err := s.PrincipalsFor(ctx, userID, share.OrgID)
			applies = err == nil && p.Member && !p.Guest
		}
		if applies && slices.Contains(shareCapabilities(share), capability) {
			return true, nil
		}
	}
	return false, nil
}

func (s *AccessService) CanFolder(ctx context.Context, userID, capability, folderID string) (bool, error) {
	folder, err := s.folder(ctx, folderID)
	if err != nil || folder == nil {
		return false, err
	}
	p, err := s.actorPrincipals(ctx, userID, folder.OrgID)
	if err != nil {
		return false, err
	}
	return s.canFolderWith(ctx, p, capability, folderID)
}

func (s *AccessService) canFolderWith(ctx context.Context, p *Principals, capability, folderID string) (bool, error) {
	current, err := s.folder(ctx, folderID)
	if err != nil || current == nil {
		return false, err
	}
	if !p.Member || (current.OrgID != "" && current.OrgID != p.OrgID) {
		return false, nil
	}
	if p.Admin || current.OwnerID == p.UserID {
		return true, nil
	}
	seen := map[string]bool{}
	for !seen[string(current.ID)] {
		seen[string(current.ID)] = true
		grants, err := queryRows[database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE folder_id = $folder", map[string]any{"folder": string(current.ID)})
		if err != nil {
			return false, err
		}
		if decision, found := decide(applicableFolderGrants(grants, p), capability); found {
			return decision, nil
		}
		if current.ParentID == "" || !current.InheritPermissions {
			return false, nil
		}
		if current, err = s.folder(ctx, current.ParentID); err != nil || current == nil {
			return false, err
		}
	}
	return false, nil
}

type grantView struct {
	role        string
	allow, deny []string
}

func applicableFolderGrants(grants []database.FolderGrant, p *Principals) []grantView {
	result := []grantView{}
	for _, grant := range grants {
		if p.Has(grant.PrincipalType, grant.PrincipalID) {
			result = append(result, grantView{grant.Role, grant.Allow, grant.Deny})
		}
	}
	return result
}

// decide applies deny, then explicit allow, then role capabilities. found is
// false when no grant applies at this level, so the caller keeps looking.
func decide(grants []grantView, capability string) (allowed, found bool) {
	if len(grants) == 0 {
		return false, false
	}
	for _, grant := range grants {
		if slices.Contains(grant.deny, capability) {
			return false, true
		}
	}
	for _, grant := range grants {
		if slices.Contains(grant.allow, capability) || slices.Contains(roleCapabilities[grant.role], capability) {
			return true, true
		}
	}
	return false, true
}

// VisibleScope describes, for one user, which documents of the organization
// they can view, in a form a single query can filter on.
type VisibleScope struct {
	OrgID  string
	UserID string
	// Admin sees every document that is not restricted.
	Admin bool
	// Folders whose documents are visible when the document inherits.
	FolderIDs []string
	// NamedDocs are granted through a user or group; BroadDocs through a
	// department, role or everyone, which never opens restricted documents.
	NamedDocs, BroadDocs []string
	DeniedDocs           []string
}

// Scope computes the visible scope for userID in orgID.
func (s *AccessService) Scope(ctx context.Context, userID, orgID string) (*VisibleScope, error) {
	p, err := s.actorPrincipals(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}
	scope := &VisibleScope{OrgID: p.OrgID, UserID: userID, Admin: p.Admin}
	if !p.Member {
		return scope, nil
	}
	if scope.FolderIDs, err = s.viewableFolders(ctx, p, "view"); err != nil {
		return nil, err
	}
	grants, err := queryRows[database.DocumentGrant](ctx, s.DB, "SELECT * FROM document_grants WHERE org_id = $org", map[string]any{"org": p.OrgID})
	if err != nil {
		return nil, err
	}
	byDocument := map[string][]database.DocumentGrant{}
	for _, grant := range grants {
		if p.Has(grant.PrincipalType, grant.PrincipalID) {
			byDocument[grant.DocumentID] = append(byDocument[grant.DocumentID], grant)
		}
	}
	for documentID, applicable := range byDocument {
		var all, namedOnly []grantView
		for _, grant := range applicable {
			view := grantView{grant.Role, grant.Allow, grant.Deny}
			all = append(all, view)
			if named(grant.PrincipalType) {
				namedOnly = append(namedOnly, view)
			}
		}
		if allowed, found := decide(all, "view"); found && !allowed {
			scope.DeniedDocs = append(scope.DeniedDocs, documentID)
			continue
		}
		if allowed, _ := decide(namedOnly, "view"); allowed {
			scope.NamedDocs = append(scope.NamedDocs, documentID)
		} else if allowed, _ := decide(all, "view"); allowed {
			scope.BroadDocs = append(scope.BroadDocs, documentID)
		}
	}
	return scope, nil
}

// Clause returns a SurrealQL condition (and its variables) selecting the
// documents in scope. Variables are prefixed with "scope_".
func (v *VisibleScope) Clause() (string, map[string]any) {
	vars := map[string]any{"scope_org": v.OrgID, "scope_user": v.UserID}
	visible := "owner_id = $scope_user OR (folder_id INSIDE $scope_folders AND inherit_permissions != false AND sensitivity != 'restricted')" +
		" OR <string> id INSIDE $scope_named OR (<string> id INSIDE $scope_broad AND sensitivity != 'restricted')"
	if v.Admin {
		visible += " OR sensitivity != 'restricted'"
	}
	vars["scope_folders"] = nonNil(v.FolderIDs)
	vars["scope_named"] = nonNil(v.NamedDocs)
	vars["scope_broad"] = nonNil(v.BroadDocs)
	vars["scope_denied"] = nonNil(v.DeniedDocs)
	return "org_id = $scope_org AND (" + visible + ") AND (owner_id = $scope_user OR <string> id NOTINSIDE $scope_denied)", vars
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// ViewableFolderIDs lists the folders of the actor's organization where the
// user holds capability, for folder trees and search filters.
func (s *AccessService) ViewableFolderIDs(ctx context.Context, userID, capability string) ([]string, error) {
	p, err := s.actorPrincipals(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	return s.viewableFolders(ctx, p, capability)
}

func (s *AccessService) viewableFolders(ctx context.Context, p *Principals, capability string) ([]string, error) {
	if !p.Member {
		return []string{}, nil
	}
	vars := map[string]any{"org": p.OrgID}
	folderRows, err := queryRows[database.Folder](ctx, s.DB, "SELECT * FROM folders WHERE org_id = $org", vars)
	if err != nil {
		return nil, err
	}
	folders := make(map[string]database.Folder, len(folderRows))
	for _, folder := range folderRows {
		folders[string(folder.ID)] = folder
	}
	result := make([]string, 0, len(folders))
	if p.Admin {
		for id := range folders {
			result = append(result, id)
		}
		slices.Sort(result)
		return result, nil
	}
	grantRows, err := queryRows[database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE org_id = $org", vars)
	if err != nil {
		return nil, err
	}
	grants := map[string][]grantView{}
	for _, grant := range grantRows {
		if p.Has(grant.PrincipalType, grant.PrincipalID) {
			grants[grant.FolderID] = append(grants[grant.FolderID], grantView{grant.Role, grant.Allow, grant.Deny})
		}
	}
	for id, folder := range folders {
		if folder.OwnerID == p.UserID || canFolderFromSnapshot(id, capability, folders, grants) {
			result = append(result, id)
		}
	}
	slices.Sort(result)
	return result, nil
}

func canFolderFromSnapshot(folderID, capability string, folders map[string]database.Folder, grants map[string][]grantView) bool {
	current, ok := folders[folderID]
	seen := map[string]bool{}
	for ok && !seen[string(current.ID)] {
		seen[string(current.ID)] = true
		if decision, found := decide(grants[string(current.ID)], capability); found {
			return decision
		}
		if current.ParentID == "" || !current.InheritPermissions {
			return false
		}
		current, ok = folders[current.ParentID]
	}
	return false
}

func (s *AccessService) folder(ctx context.Context, id string) (*database.Folder, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrFolderNotFound
	}
	return surrealdb.Select[database.Folder](ctx, s.DB, rid)
}
