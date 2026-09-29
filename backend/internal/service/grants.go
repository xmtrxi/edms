package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

var ErrInvalidGrant = errors.New("invalid grant")

// GrantInput names who gets access and at which role.
type GrantInput struct {
	PrincipalType string   `json:"principal_type"`
	PrincipalID   string   `json:"principal_id"`
	Role          string   `json:"role"`
	Allow         []string `json:"allow"`
	Deny          []string `json:"deny"`
}

// normalizeGrant validates a grant against the organization: people must be
// members, and departments, groups and roles must belong to it. Restricted
// documents only accept named people and groups.
func normalizeGrant(ctx context.Context, db *surrealdb.DB, orgID string, in GrantInput, restricted bool) (GrantInput, error) {
	in.PrincipalType = strings.TrimSpace(in.PrincipalType)
	in.PrincipalID = strings.TrimSpace(in.PrincipalID)
	in.Role = strings.TrimSpace(in.Role)
	if !database.IsPrincipalType(in.PrincipalType) {
		return in, fmt.Errorf("%w: choose a person, department, group, role or everyone", ErrInvalidGrant)
	}
	if !IsGrantRole(in.Role) {
		return in, fmt.Errorf("%w: choose a valid role", ErrInvalidGrant)
	}
	if restricted && !named(in.PrincipalType) {
		return in, fmt.Errorf("%w: restricted documents can only be shared with named people or groups", ErrInvalidGrant)
	}
	vars := map[string]any{"org": orgID, "id": in.PrincipalID}
	var query string
	switch in.PrincipalType {
	case database.PrincipalEveryone:
		in.PrincipalID = database.EveryonePrincipalID
		return in, nil
	case database.PrincipalUser:
		query = "SELECT * FROM memberships WHERE org_id = $org AND user_id = $id AND status = 'active' LIMIT 1"
	case database.PrincipalDepartment:
		query = "SELECT * FROM departments WHERE org_id = $org AND <string> id = $id LIMIT 1"
	case database.PrincipalGroup:
		query = "SELECT * FROM groups WHERE org_id = $org AND <string> id = $id LIMIT 1"
	case database.PrincipalRole:
		query = "SELECT * FROM roles WHERE (org_id = $org OR protected = true) AND key = $id LIMIT 1"
	}
	if in.PrincipalID == "" {
		return in, fmt.Errorf("%w: choose who gets access", ErrInvalidGrant)
	}
	rows, err := queryRows[map[string]any](ctx, db, query, vars)
	if err != nil {
		return in, err
	}
	if len(rows) == 0 {
		return in, fmt.Errorf("%w: %s is not part of this organization", ErrInvalidGrant, in.PrincipalType)
	}
	return in, nil
}

func upsertDocumentGrant(ctx context.Context, db *surrealdb.DB, document *database.Document, actor string, in GrantInput) (*database.DocumentGrant, error) {
	in, err := normalizeGrant(ctx, db, document.OrgID, in, document.Sensitivity == database.SensitivityRestricted)
	if err != nil {
		return nil, err
	}
	existing, err := queryRows[database.DocumentGrant](ctx, db,
		"SELECT * FROM document_grants WHERE document_id = $document AND principal_type = $type AND principal_id = $principal LIMIT 1",
		map[string]any{"document": string(document.ID), "type": in.PrincipalType, "principal": in.PrincipalID})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if len(existing) > 0 {
		rid, _ := database.ParseRecordID(string(existing[0].ID))
		return surrealdb.Merge[database.DocumentGrant](ctx, db, rid, map[string]any{"role": in.Role, "allow": nonNil(in.Allow), "deny": nonNil(in.Deny), "updated_at": now})
	}
	return surrealdb.Create[database.DocumentGrant](ctx, db, "document_grants", &database.DocumentGrant{
		OrgID: document.OrgID, DocumentID: string(document.ID), PrincipalType: in.PrincipalType, PrincipalID: in.PrincipalID,
		Role: in.Role, Allow: nonNil(in.Allow), Deny: nonNil(in.Deny), CreatedBy: actor, CreatedAt: now, UpdatedAt: now,
	})
}

// PrincipalOption is one choice in the "share with" picker.
type PrincipalOption struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PrincipalService powers the people-and-teams picker and explains grants.
type PrincipalService struct {
	db   *surrealdb.DB
	orgs *OrganizationService
}

func NewPrincipalService(db *surrealdb.DB, orgs *OrganizationService) *PrincipalService {
	return &PrincipalService{db: db, orgs: orgs}
}

// Search finds members, departments, groups and roles of orgID matching query.
// Guests can only pick individual people.
func (s *PrincipalService) Search(ctx context.Context, orgID, query string, guest bool) ([]PrincipalOption, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	matches := func(values ...string) bool {
		if needle == "" {
			return true
		}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), needle) {
				return true
			}
		}
		return false
	}
	options := []PrincipalOption{}
	if !guest && matches("everyone", "all staff", "organization") {
		options = append(options, PrincipalOption{Type: database.PrincipalEveryone, ID: database.EveryonePrincipalID, Name: "Everyone in the organization"})
	}
	members, err := s.orgs.Members(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member.Status == database.MembershipStatusActive && matches(member.Name, member.Email) {
			options = append(options, PrincipalOption{Type: database.PrincipalUser, ID: member.UserID, Name: firstNonEmpty(member.Name, member.Email), Description: member.Email})
		}
	}
	if guest {
		return limitOptions(options), nil
	}
	vars := map[string]any{"org": orgID}
	departments, err := queryRows[database.Department](ctx, s.db, "SELECT * FROM departments WHERE org_id = $org ORDER BY name", vars)
	if err != nil {
		return nil, err
	}
	for _, department := range departments {
		if matches(department.Name) {
			options = append(options, PrincipalOption{Type: database.PrincipalDepartment, ID: string(department.ID), Name: department.Name, Description: "Department"})
		}
	}
	groups, err := queryRows[database.Group](ctx, s.db, "SELECT * FROM groups WHERE org_id = $org ORDER BY name", vars)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if matches(group.Name) {
			options = append(options, PrincipalOption{Type: database.PrincipalGroup, ID: string(group.ID), Name: group.Name, Description: "Group"})
		}
	}
	roles, err := queryRows[database.Role](ctx, s.db, "SELECT * FROM roles WHERE org_id = $org OR protected = true ORDER BY name", vars)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if matches(role.Name, role.Key) {
			options = append(options, PrincipalOption{Type: database.PrincipalRole, ID: role.Key, Name: role.Name, Description: "Role"})
		}
	}
	return limitOptions(options), nil
}

func limitOptions(options []PrincipalOption) []PrincipalOption {
	if len(options) > 50 {
		return options[:50]
	}
	return options
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// GrantView is a grant with a readable principal name for the share dialog.
type GrantView struct {
	ID            string   `json:"id"`
	PrincipalType string   `json:"principal_type"`
	PrincipalID   string   `json:"principal_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Role          string   `json:"role"`
	Allow         []string `json:"allow,omitempty"`
	Deny          []string `json:"deny,omitempty"`
	Source        string   `json:"source"`
	SourceName    string   `json:"source_name,omitempty"`
}

// Describe fills in principal names for grants.
func (s *PrincipalService) Describe(ctx context.Context, orgID string, grants []GrantView) []GrantView {
	options, err := s.Search(ctx, orgID, "", false)
	if err != nil {
		return grants
	}
	byKey := map[string]PrincipalOption{}
	for _, option := range options {
		byKey[principalKey(option.Type, option.ID)] = option
	}
	for i := range grants {
		if option, ok := byKey[principalKey(grants[i].PrincipalType, grants[i].PrincipalID)]; ok {
			grants[i].Name, grants[i].Description = option.Name, option.Description
		} else if grants[i].Name == "" {
			grants[i].Name = "Removed " + grants[i].PrincipalType
		}
	}
	sort.SliceStable(grants, func(i, j int) bool { return grants[i].Source < grants[j].Source })
	return grants
}
