package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/deployment"
)

var (
	ErrNoOrganization        = errors.New("you are not a member of any organization")
	ErrNotMember             = errors.New("you are not a member of this organization")
	ErrOrganizationLimit     = errors.New("this installation does not allow creating more organizations")
	ErrInvitationInvalid     = errors.New("invitation is invalid or expired")
	ErrInvitationEmail       = errors.New("this invitation was sent to a different email address")
	ErrLastOwner             = errors.New("an organization must keep at least one owner")
	ErrPersonalWorkspaceTeam = errors.New("personal workspaces cannot have other members")
)

type OrganizationService struct {
	db    *surrealdb.DB
	caps  deployment.Capabilities
	smtp  config.SMTPConfig
	authz *AuthorizationService
	log   *slog.Logger
}

func NewOrganizationService(cfg config.Config, db *surrealdb.DB, logger *slog.Logger) *OrganizationService {
	authz, _ := NewAuthorizationService(db)
	if logger == nil {
		logger = slog.Default()
	}
	return &OrganizationService{db: db, caps: deployment.New(cfg.Deployment), smtp: cfg.SMTP, authz: authz, log: logger}
}

func (s *OrganizationService) Capabilities() deployment.Capabilities { return s.caps }

// UserOrganization is an organization as seen by one of its members.
type UserOrganization struct {
	database.Organization
	Role string `json:"role"`
}

// MemberView joins a membership with the member's public profile.
type MemberView struct {
	database.Membership
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *OrganizationService) Get(ctx context.Context, orgID string) (*database.Organization, error) {
	rid, err := database.ParseRecordID(orgID)
	if err != nil {
		return nil, ErrNoOrganization
	}
	org, err := surrealdb.Select[database.Organization](ctx, s.db, rid)
	if err != nil || org == nil || org.ID == "" {
		return nil, ErrNoOrganization
	}
	return org, nil
}

// Membership returns the user's active membership in orgID.
func (s *OrganizationService) Membership(ctx context.Context, orgID, userID string) (*database.Membership, error) {
	items, err := queryRows[database.Membership](ctx, s.db,
		"SELECT * FROM memberships WHERE org_id = $org AND user_id = $user AND status = $status LIMIT 1",
		map[string]any{"org": orgID, "user": userID, "status": database.MembershipStatusActive})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNotMember
	}
	return &items[0], nil
}

// ForUser lists the organizations the user is an active member of.
func (s *OrganizationService) ForUser(ctx context.Context, userID string) ([]UserOrganization, error) {
	memberships, err := queryRows[database.Membership](ctx, s.db,
		"SELECT * FROM memberships WHERE user_id = $user AND status = $status ORDER BY created_at",
		map[string]any{"user": userID, "status": database.MembershipStatusActive})
	if err != nil {
		return nil, err
	}
	result := make([]UserOrganization, 0, len(memberships))
	for _, membership := range memberships {
		org, err := s.Get(ctx, membership.OrgID)
		if err != nil {
			continue
		}
		result = append(result, UserOrganization{Organization: *org, Role: membership.Role})
	}
	// Personal workspace first, then organizations alphabetically.
	sort.SliceStable(result, func(i, j int) bool {
		pi, pj := result[i].Kind == database.OrganizationKindPersonal, result[j].Kind == database.OrganizationKindPersonal
		if pi != pj {
			return pi
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

// Resolve picks the organization a request acts in: the requested one when the
// user belongs to it, otherwise the last one used, otherwise the first one.
// When the user belongs to none and personal workspaces are enabled, one is
// created so a new account always has somewhere to put files.
func (s *OrganizationService) Resolve(ctx context.Context, user *database.User, requested string) (*database.Organization, *database.Membership, error) {
	userID := string(user.ID)
	for _, candidate := range []string{requested, user.LastOrgID} {
		if candidate == "" {
			continue
		}
		if membership, err := s.Membership(ctx, candidate, userID); err == nil {
			org, err := s.Get(ctx, candidate)
			if err == nil {
				return org, membership, nil
			}
		}
	}
	orgs, err := s.ForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(orgs) == 0 {
		if !s.caps.PersonalWorkspaces() {
			return nil, nil, ErrNoOrganization
		}
		org, err := s.EnsurePersonalWorkspace(ctx, user)
		if err != nil {
			return nil, nil, err
		}
		membership, err := s.Membership(ctx, string(org.ID), userID)
		return org, membership, err
	}
	org := orgs[0].Organization
	membership, err := s.Membership(ctx, string(org.ID), userID)
	return &org, membership, err
}

// SetLastOrganization remembers the workspace the user switched to.
func (s *OrganizationService) SetLastOrganization(ctx context.Context, userID, orgID string) error {
	rid, err := database.ParseRecordID(userID)
	if err != nil {
		return err
	}
	_, err = surrealdb.Merge[database.User](ctx, s.db, rid, map[string]any{"last_org_id": orgID})
	return err
}

// Create makes a new organization owned by user. It enforces the deployment's
// limit on the number of organizations.
func (s *OrganizationService) Create(ctx context.Context, user *database.User, name string) (*database.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("organization name is required")
	}
	count, err := s.organizationCount(ctx)
	if err != nil {
		return nil, err
	}
	if !s.caps.CanCreateOrganization(count) {
		return nil, ErrOrganizationLimit
	}
	return s.create(ctx, user, name, database.OrganizationKindOrganization)
}

// EnsurePersonalWorkspace returns the user's private workspace, creating it once.
func (s *OrganizationService) EnsurePersonalWorkspace(ctx context.Context, user *database.User) (*database.Organization, error) {
	existing, err := queryRows[database.Organization](ctx, s.db,
		"SELECT * FROM organizations WHERE kind = $kind AND owner_id = $owner LIMIT 1",
		map[string]any{"kind": database.OrganizationKindPersonal, "owner": string(user.ID)})
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return &existing[0], nil
	}
	return s.create(ctx, user, "Personal", database.OrganizationKindPersonal)
}

func (s *OrganizationService) create(ctx context.Context, user *database.User, name, kind string) (*database.Organization, error) {
	slug, err := s.uniqueSlug(ctx, name)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	org, err := surrealdb.Create[database.Organization](ctx, s.db, "organizations", &database.Organization{
		Slug: slug, Name: name, Kind: kind, OwnerID: string(user.ID), Settings: map[string]any{}, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	if _, err := s.addMember(ctx, string(org.ID), string(user.ID), database.MembershipRoleOwner, ""); err != nil {
		return nil, err
	}
	return org, nil
}

// Bootstrap prepares a self-hosted install on startup: it creates the single
// organization with the configured name and, when an admin email is set and
// nobody has an account yet, issues the owner invitation and logs its link.
func (s *OrganizationService) Bootstrap(ctx context.Context) error {
	if s.caps.IsSaaS() {
		return nil
	}
	orgs, err := queryRows[database.Organization](ctx, s.db,
		"SELECT * FROM organizations WHERE kind = $kind ORDER BY created_at LIMIT 1",
		map[string]any{"kind": database.OrganizationKindOrganization})
	if err != nil {
		return err
	}
	name := s.caps.DefaultOrganizationName()
	var org *database.Organization
	if len(orgs) == 0 {
		now := time.Now().UTC()
		slug, err := s.uniqueSlug(ctx, name)
		if err != nil {
			return err
		}
		org, err = surrealdb.Create[database.Organization](ctx, s.db, "organizations", &database.Organization{
			Slug: slug, Name: name, Kind: database.OrganizationKindOrganization, Settings: map[string]any{}, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}
		s.log.Info("created organization", "name", name, "id", org.ID)
	} else {
		org = &orgs[0]
		if org.Name == "Default organization" && name != org.Name {
			rid, _ := database.ParseRecordID(string(org.ID))
			if _, err := surrealdb.Merge[database.Organization](ctx, s.db, rid, map[string]any{"name": name, "updated_at": time.Now().UTC()}); err != nil {
				return err
			}
		}
	}

	email := strings.ToLower(strings.TrimSpace(s.caps.AdminEmail()))
	if email == "" {
		return nil
	}
	users, err := queryRows[database.User](ctx, s.db, "SELECT * FROM users LIMIT 1", nil)
	if err != nil || len(users) > 0 {
		return err
	}
	pending, err := queryRows[database.Invitation](ctx, s.db,
		"SELECT * FROM invitations WHERE org_id = $org AND email = $email AND status = 'pending' AND expires_at > time::now() LIMIT 1",
		map[string]any{"org": string(org.ID), "email": email})
	if err != nil || len(pending) > 0 {
		return err
	}
	link, err := s.Invite(ctx, string(org.ID), "", InviteInput{Email: email, Name: "Administrator", Role: database.MembershipRoleOwner})
	if err != nil {
		return err
	}
	s.log.Warn("first administrator invitation created; open this link to finish setup", "email", email, "link", link)
	return nil
}

// FirstRegistration handles the very first account on an install. On a
// self-hosted install that account becomes owner of the single organization.
func (s *OrganizationService) FirstRegistration(ctx context.Context, user *database.User, orgName string) (*database.Organization, error) {
	if s.caps.IsSaaS() {
		return s.AfterSignup(ctx, user, orgName)
	}
	orgs, err := queryRows[database.Organization](ctx, s.db,
		"SELECT * FROM organizations WHERE kind = $kind ORDER BY created_at LIMIT 1",
		map[string]any{"kind": database.OrganizationKindOrganization})
	if err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		name := strings.TrimSpace(orgName)
		if name == "" {
			name = s.caps.DefaultOrganizationName()
		}
		return s.create(ctx, user, name, database.OrganizationKindOrganization)
	}
	org := orgs[0]
	rid, _ := database.ParseRecordID(string(org.ID))
	if _, err := surrealdb.Merge[database.Organization](ctx, s.db, rid, map[string]any{"owner_id": string(user.ID), "updated_at": time.Now().UTC()}); err != nil {
		return nil, err
	}
	if _, err := s.addMember(ctx, string(org.ID), string(user.ID), database.MembershipRoleOwner, ""); err != nil {
		return nil, err
	}
	return &org, nil
}

// AfterSignup provisions workspaces for a new SaaS account: a personal
// workspace when enabled and a team organization when a name was given.
func (s *OrganizationService) AfterSignup(ctx context.Context, user *database.User, orgName string) (*database.Organization, error) {
	var active *database.Organization
	if s.caps.PersonalWorkspaces() {
		personal, err := s.EnsurePersonalWorkspace(ctx, user)
		if err != nil {
			return nil, err
		}
		active = personal
	}
	if strings.TrimSpace(orgName) != "" {
		org, err := s.Create(ctx, user, orgName)
		if err != nil {
			return nil, err
		}
		active = org
	}
	if active == nil {
		return nil, ErrNoOrganization
	}
	return active, s.SetLastOrganization(ctx, string(user.ID), string(active.ID))
}

func (s *OrganizationService) Rename(ctx context.Context, orgID, name string) (*database.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("organization name is required")
	}
	rid, err := database.ParseRecordID(orgID)
	if err != nil {
		return nil, ErrNoOrganization
	}
	return surrealdb.Merge[database.Organization](ctx, s.db, rid, map[string]any{"name": name, "updated_at": time.Now().UTC()})
}

// Members lists everyone in the organization with their profile.
func (s *OrganizationService) Members(ctx context.Context, orgID string) ([]MemberView, error) {
	memberships, err := queryRows[database.Membership](ctx, s.db,
		"SELECT * FROM memberships WHERE org_id = $org ORDER BY created_at", map[string]any{"org": orgID})
	if err != nil {
		return nil, err
	}
	result := make([]MemberView, 0, len(memberships))
	for _, membership := range memberships {
		view := MemberView{Membership: membership}
		if rid, err := database.ParseRecordID(membership.UserID); err == nil {
			if user, err := surrealdb.Select[database.User](ctx, s.db, rid); err == nil && user != nil {
				view.Email, view.Name = user.Email, user.Name
			}
		}
		result = append(result, view)
	}
	return result, nil
}

// MemberUsers lists the active users of an organization.
func (s *OrganizationService) MemberUsers(ctx context.Context, orgID string) ([]database.User, error) {
	members, err := s.Members(ctx, orgID)
	if err != nil {
		return nil, err
	}
	users := make([]database.User, 0, len(members))
	for _, member := range members {
		if member.Status != database.MembershipStatusActive {
			continue
		}
		users = append(users, database.User{ID: database.RecordID(member.UserID), Email: member.Email, Name: member.Name, Status: database.UserStatusActive})
	}
	return users, nil
}

type MemberUpdate struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

// UpdateMember changes a member's role or suspends them. Only owners can make
// or unmake owners, and the last owner can never be removed or demoted.
func (s *OrganizationService) UpdateMember(ctx context.Context, orgID string, actor *database.Membership, membershipID string, in MemberUpdate) (*database.Membership, error) {
	target, err := s.membershipByID(ctx, orgID, membershipID)
	if err != nil {
		return nil, err
	}
	changes := map[string]any{"updated_at": time.Now().UTC()}
	role := target.Role
	if in.Role != "" && in.Role != target.Role {
		if !database.IsMembershipRole(in.Role) {
			return nil, fmt.Errorf("unknown role %q", in.Role)
		}
		if (in.Role == database.MembershipRoleOwner || target.Role == database.MembershipRoleOwner) && actor.Role != database.MembershipRoleOwner {
			return nil, ErrAccessDenied
		}
		role = in.Role
		changes["role"] = role
	}
	status := target.Status
	if in.Status != "" && in.Status != target.Status {
		if in.Status != database.MembershipStatusActive && in.Status != database.MembershipStatusSuspended {
			return nil, fmt.Errorf("unknown status %q", in.Status)
		}
		if target.Role == database.MembershipRoleOwner && actor.Role != database.MembershipRoleOwner {
			return nil, ErrAccessDenied
		}
		status = in.Status
		changes["status"] = status
	}
	if target.Role == database.MembershipRoleOwner && (role != database.MembershipRoleOwner || status != database.MembershipStatusActive) {
		if err := s.requireAnotherOwner(ctx, orgID, target.UserID); err != nil {
			return nil, err
		}
	}
	rid, _ := database.ParseRecordID(string(target.ID))
	updated, err := surrealdb.Merge[database.Membership](ctx, s.db, rid, changes)
	if err != nil {
		return nil, err
	}
	if err := s.syncAdminRole(ctx, orgID, target.UserID, role, status); err != nil {
		return nil, err
	}
	return updated, nil
}

// RemoveMember takes a member out of the organization. Their files stay with
// the organization; transferring them is a separate step.
func (s *OrganizationService) RemoveMember(ctx context.Context, orgID string, actor *database.Membership, membershipID string) error {
	target, err := s.membershipByID(ctx, orgID, membershipID)
	if err != nil {
		return err
	}
	if target.Role == database.MembershipRoleOwner {
		if actor.Role != database.MembershipRoleOwner {
			return ErrAccessDenied
		}
		if err := s.requireAnotherOwner(ctx, orgID, target.UserID); err != nil {
			return err
		}
	}
	rid, _ := database.ParseRecordID(string(target.ID))
	if _, err := surrealdb.Delete[database.Membership](ctx, s.db, rid); err != nil {
		return err
	}
	vars := map[string]any{"org": orgID, "user": target.UserID}
	for _, table := range []string{"department_members", "group_members"} {
		if _, err := surrealdb.Query[any](ctx, s.db, "DELETE "+table+" WHERE org_id = $org AND user_id = $user", vars); err != nil {
			return err
		}
	}
	if s.authz != nil {
		return s.authz.RevokeRoles(ctx, target.UserID, orgID)
	}
	return nil
}

type InviteInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Invite creates an invitation into the organization and emails the link when
// SMTP is configured. The link is always returned so an admin can share it.
func (s *OrganizationService) Invite(ctx context.Context, orgID, actor string, in InviteInput) (string, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return "", fmt.Errorf("a valid email is required")
	}
	role := in.Role
	if role == "" {
		role = database.MembershipRoleMember
	}
	if !database.IsMembershipRole(role) {
		return "", fmt.Errorf("unknown role %q", role)
	}
	org, err := s.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	if org.Kind == database.OrganizationKindPersonal {
		return "", ErrPersonalWorkspaceTeam
	}
	if existing, err := s.userByEmail(ctx, email); err == nil && existing != nil {
		if _, err := s.Membership(ctx, orgID, string(existing.ID)); err == nil {
			return "", fmt.Errorf("%s is already a member", email)
		}
	}
	vars := map[string]any{"org": orgID, "email": email}
	if _, err := surrealdb.Query[any](ctx, s.db, "UPDATE invitations SET status = 'revoked' WHERE org_id = $org AND email = $email AND status = 'pending'", vars); err != nil {
		return "", err
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := time.Now().UTC()
	// A map, not the struct: a zero accepted_at would be sent as NULL, which the
	// optional datetime column rejects.
	if _, err := surrealdb.Create[database.Invitation](ctx, s.db, "invitations", map[string]any{
		"org_id": orgID, "role": role, "email": email, "name": strings.TrimSpace(in.Name), "token_hash": hashToken(token),
		"status": "pending", "expires_at": now.Add(7 * 24 * time.Hour), "created_by": actor, "created_at": now,
	}); err != nil {
		return "", err
	}
	link := strings.TrimRight(s.smtp.AppURL, "/") + "/auth/invite?token=" + token
	if s.smtp.Host != "" && s.smtp.From != "" {
		body := fmt.Sprintf("To: %s\r\nSubject: You're invited to %s\r\n\r\nYou have been invited to join %s.\r\n\r\nAccept the invitation: %s\r\n", email, org.Name, org.Name, link)
		auth := smtp.PlainAuth("", s.smtp.Username, s.smtp.Password, s.smtp.Host)
		if err := smtp.SendMail(s.smtp.Host+":"+s.smtp.Port, auth, s.smtp.From, []string{email}, []byte(body)); err != nil {
			s.log.Warn("send invitation email", "email", email, "error", err)
		}
	}
	return link, nil
}

func (s *OrganizationService) Invitations(ctx context.Context, orgID string) ([]database.Invitation, error) {
	return queryRows[database.Invitation](ctx, s.db,
		"SELECT * FROM invitations WHERE org_id = $org AND status = 'pending' ORDER BY created_at DESC", map[string]any{"org": orgID})
}

func (s *OrganizationService) RevokeInvitation(ctx context.Context, orgID, invitationID string) error {
	rid, err := database.ParseRecordID(invitationID)
	if err != nil {
		return ErrInvitationInvalid
	}
	invite, err := surrealdb.Select[database.Invitation](ctx, s.db, rid)
	if err != nil || invite == nil || invite.OrgID != orgID {
		return ErrInvitationInvalid
	}
	_, err = surrealdb.Merge[database.Invitation](ctx, s.db, rid, map[string]any{"status": "revoked"})
	return err
}

// InvitationPreview is what an invitee sees before accepting.
type InvitationPreview struct {
	Email            string `json:"email"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	OrganizationName string `json:"organization_name"`
	AccountExists    bool   `json:"account_exists"`
}

func (s *OrganizationService) PreviewInvitation(ctx context.Context, token string) (*InvitationPreview, error) {
	invite, err := s.pendingInvitation(ctx, token)
	if err != nil {
		return nil, err
	}
	org, err := s.Get(ctx, invite.OrgID)
	if err != nil {
		return nil, ErrInvitationInvalid
	}
	existing, _ := s.userByEmail(ctx, invite.Email)
	return &InvitationPreview{Email: invite.Email, Name: invite.Name, Role: invite.Role, OrganizationName: org.Name, AccountExists: existing != nil}, nil
}

// AcceptInvitation adds user to the invitation's organization. The signed-in
// user's email must match the address the invitation was sent to.
func (s *OrganizationService) AcceptInvitation(ctx context.Context, token string, user *database.User) (*database.Organization, error) {
	invite, err := s.pendingInvitation(ctx, token)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), invite.Email) {
		return nil, ErrInvitationEmail
	}
	return s.acceptFor(ctx, invite, user)
}

func (s *OrganizationService) acceptFor(ctx context.Context, invite *database.Invitation, user *database.User) (*database.Organization, error) {
	org, err := s.Get(ctx, invite.OrgID)
	if err != nil {
		return nil, ErrInvitationInvalid
	}
	role := invite.Role
	if !database.IsMembershipRole(role) {
		role = database.MembershipRoleMember
	}
	if _, err := s.Membership(ctx, invite.OrgID, string(user.ID)); errors.Is(err, ErrNotMember) {
		if _, err := s.addMember(ctx, invite.OrgID, string(user.ID), role, invite.CreatedBy); err != nil {
			return nil, err
		}
	}
	if role == database.MembershipRoleOwner && org.OwnerID == "" {
		rid, _ := database.ParseRecordID(invite.OrgID)
		if _, err := surrealdb.Merge[database.Organization](ctx, s.db, rid, map[string]any{"owner_id": string(user.ID)}); err != nil {
			return nil, err
		}
	}
	rid, _ := database.ParseRecordID(string(invite.ID))
	if _, err := surrealdb.Merge[database.Invitation](ctx, s.db, rid, map[string]any{"status": "accepted", "accepted_at": time.Now().UTC()}); err != nil {
		return nil, err
	}
	if err := s.SetLastOrganization(ctx, string(user.ID), invite.OrgID); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrganizationService) pendingInvitation(ctx context.Context, token string) (*database.Invitation, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvitationInvalid
	}
	items, err := queryRows[database.Invitation](ctx, s.db,
		"SELECT * FROM invitations WHERE token_hash = $hash AND status = 'pending' LIMIT 1",
		map[string]any{"hash": hashToken(token)})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || time.Now().UTC().After(items[0].ExpiresAt) {
		return nil, ErrInvitationInvalid
	}
	invite := items[0]
	invite.Email = strings.ToLower(strings.TrimSpace(invite.Email))
	return &invite, nil
}

func (s *OrganizationService) addMember(ctx context.Context, orgID, userID, role, invitedBy string) (*database.Membership, error) {
	now := time.Now().UTC()
	membership, err := surrealdb.Create[database.Membership](ctx, s.db, "memberships", &database.Membership{
		OrgID: orgID, UserID: userID, Role: role, Status: database.MembershipStatusActive,
		InvitedBy: invitedBy, JoinedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	if err := s.syncAdminRole(ctx, orgID, userID, role, database.MembershipStatusActive); err != nil {
		return nil, err
	}
	return membership, nil
}

// syncAdminRole keeps Casbin in step with the coarse org role: owners and
// admins hold the protected admin role in the organization's domain.
func (s *OrganizationService) syncAdminRole(ctx context.Context, orgID, userID, role, status string) error {
	if s.authz == nil {
		return nil
	}
	isAdmin := status == database.MembershipStatusActive && (role == database.MembershipRoleOwner || role == database.MembershipRoleAdmin)
	if isAdmin {
		return s.authz.BootstrapAdmin(ctx, userID, orgID)
	}
	if s.authz.Can(userID, orgID, "*", "*") {
		return s.authz.RevokeRoles(ctx, userID, orgID)
	}
	return nil
}

func (s *OrganizationService) requireAnotherOwner(ctx context.Context, orgID, userID string) error {
	owners, err := queryRows[database.Membership](ctx, s.db,
		"SELECT * FROM memberships WHERE org_id = $org AND role = $role AND status = $status AND user_id != $user LIMIT 1",
		map[string]any{"org": orgID, "role": database.MembershipRoleOwner, "status": database.MembershipStatusActive, "user": userID})
	if err != nil {
		return err
	}
	if len(owners) == 0 {
		return ErrLastOwner
	}
	return nil
}

func (s *OrganizationService) membershipByID(ctx context.Context, orgID, id string) (*database.Membership, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrNotMember
	}
	membership, err := surrealdb.Select[database.Membership](ctx, s.db, rid)
	if err != nil || membership == nil || membership.OrgID != orgID {
		return nil, ErrNotMember
	}
	return membership, nil
}

// UsersByID loads users for display, skipping ids that no longer exist.
func (s *OrganizationService) UsersByID(ctx context.Context, ids []string) map[string]*database.User {
	users := map[string]*database.User{}
	for _, id := range ids {
		if _, done := users[id]; done || id == "" {
			continue
		}
		if user, err := s.userByID(ctx, id); err == nil && user != nil && user.ID != "" {
			users[id] = user
		}
	}
	return users
}

func (s *OrganizationService) userByID(ctx context.Context, id string) (*database.User, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	return surrealdb.Select[database.User](ctx, s.db, rid)
}

func (s *OrganizationService) userByEmail(ctx context.Context, email string) (*database.User, error) {
	items, err := queryRows[database.User](ctx, s.db, "SELECT * FROM users WHERE string::lowercase(email) = $email LIMIT 1", map[string]any{"email": strings.ToLower(strings.TrimSpace(email))})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (s *OrganizationService) organizationCount(ctx context.Context) (int, error) {
	return queryCount(ctx, s.db, "SELECT count() AS count FROM organizations WHERE kind = $kind GROUP ALL", map[string]any{"kind": database.OrganizationKindOrganization})
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	slug := strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		slug = "workspace"
	}
	return slug
}

func (s *OrganizationService) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := slugify(name)
	for attempt := 0; attempt < 5; attempt++ {
		candidate := base
		if attempt > 0 {
			suffix := make([]byte, 3)
			if _, err := rand.Read(suffix); err != nil {
				return "", err
			}
			candidate = base + "-" + hex.EncodeToString(suffix)
		}
		taken, err := queryRows[database.Organization](ctx, s.db, "SELECT * FROM organizations WHERE slug = $slug LIMIT 1", map[string]any{"slug": candidate})
		if err != nil {
			return "", err
		}
		if len(taken) == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate a unique organization slug")
}

func hashToken(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}
