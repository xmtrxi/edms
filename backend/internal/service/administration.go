package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
)

type DepartmentInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id"`
}
type AdministrationService struct {
	db    *surrealdb.DB
	authz *AuthorizationService
	orgs  *OrganizationService
}

func NewAdministrationService(db *surrealdb.DB, authz *AuthorizationService, orgs *OrganizationService) *AdministrationService {
	return &AdministrationService{db: db, authz: authz, orgs: orgs}
}
func (s *AdministrationService) Departments(ctx context.Context, orgID string) ([]database.Department, error) {
	return queryRows[database.Department](ctx, s.db, "SELECT * FROM departments WHERE org_id = $org ORDER BY name", map[string]any{"org": orgID})
}

func (s *AdministrationService) CreateDepartment(ctx context.Context, orgID, actor string, in DepartmentInput) (*database.Department, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("department name is required")
	}
	if in.ParentID != "" {
		if _, err := s.department(ctx, orgID, in.ParentID); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	return surrealdb.Create[database.Department](ctx, s.db, "departments", &database.Department{OrgID: orgID, ParentID: in.ParentID, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), CreatedBy: actor, CreatedAt: now, UpdatedAt: now})
}

func (s *AdministrationService) Members(ctx context.Context, orgID, departmentID string) ([]database.DepartmentMember, error) {
	if _, err := s.department(ctx, orgID, departmentID); err != nil {
		return nil, err
	}
	return queryRows[database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE org_id = $org AND department_id = $department", map[string]any{"org": orgID, "department": departmentID})
}

// AddMember puts an organization member into a department. People outside the
// organization cannot be added.
func (s *AdministrationService) AddMember(ctx context.Context, orgID, departmentID, userID, role string) (*database.DepartmentMember, error) {
	if _, err := s.department(ctx, orgID, departmentID); err != nil {
		return nil, err
	}
	if s.orgs != nil {
		if _, err := s.orgs.Membership(ctx, orgID, userID); err != nil {
			return nil, err
		}
	}
	if role != "head" {
		role = "member"
	}
	existing, err := queryRows[database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE department_id = $department AND user_id = $user LIMIT 1", map[string]any{"department": departmentID, "user": userID})
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		rid, _ := database.ParseRecordID(string(existing[0].ID))
		return surrealdb.Merge[database.DepartmentMember](ctx, s.db, rid, map[string]any{"role": role})
	}
	return surrealdb.Create[database.DepartmentMember](ctx, s.db, "department_members", &database.DepartmentMember{OrgID: orgID, DepartmentID: departmentID, UserID: userID, Role: role, CreatedAt: time.Now().UTC()})
}

func (s *AdministrationService) department(ctx context.Context, orgID, id string) (*database.Department, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("department not found")
	}
	department, err := surrealdb.Select[database.Department](ctx, s.db, rid)
	if err != nil || department == nil || department.OrgID != orgID {
		return nil, fmt.Errorf("department not found")
	}
	return department, nil
}

// Roles lists the system roles plus the organization's custom roles.
func (s *AdministrationService) Roles(ctx context.Context, orgID string) ([]database.Role, error) {
	return queryRows[database.Role](ctx, s.db, "SELECT * FROM roles WHERE org_id = $org OR protected = true ORDER BY name", map[string]any{"org": orgID})
}

// AssignRole gives an organization member a role in that organization.
func (s *AdministrationService) AssignRole(ctx context.Context, orgID, userID, role string) error {
	if s.orgs != nil {
		if _, err := s.orgs.Membership(ctx, orgID, userID); err != nil {
			return err
		}
	}
	roles, err := s.Roles(ctx, orgID)
	if err != nil {
		return err
	}
	for _, candidate := range roles {
		if candidate.Key == role {
			return s.authz.AssignRole(ctx, userID, role, orgID)
		}
	}
	return fmt.Errorf("role %q not found", role)
}

// UpdateDepartment renames a department or moves it under another one.
func (s *AdministrationService) UpdateDepartment(ctx context.Context, orgID, id string, in DepartmentInput) (*database.Department, error) {
	if _, err := s.department(ctx, orgID, id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("department name is required")
	}
	if in.ParentID != "" {
		if in.ParentID == id {
			return nil, fmt.Errorf("a department cannot be its own parent")
		}
		departments, err := s.Departments(ctx, orgID)
		if err != nil {
			return nil, err
		}
		if slices.Contains(descendantDepartments(departments, []string{id}), in.ParentID) {
			return nil, fmt.Errorf("a department cannot move under one of its own sub-departments")
		}
		if _, err := s.department(ctx, orgID, in.ParentID); err != nil {
			return nil, err
		}
	}
	rid, _ := database.ParseRecordID(id)
	return surrealdb.Merge[database.Department](ctx, s.db, rid, map[string]any{"name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "parent_id": in.ParentID, "updated_at": time.Now().UTC()})
}

// DeleteDepartment removes an empty department. Its grants are removed too.
func (s *AdministrationService) DeleteDepartment(ctx context.Context, orgID, id string) error {
	if _, err := s.department(ctx, orgID, id); err != nil {
		return err
	}
	children, err := queryRows[database.Department](ctx, s.db, "SELECT * FROM departments WHERE parent_id = $id LIMIT 1", map[string]any{"id": id})
	if err != nil {
		return err
	}
	if len(children) > 0 {
		return fmt.Errorf("move or delete its sub-departments first")
	}
	vars := map[string]any{"id": id, "record": mustRecordID(id)}
	for _, statement := range []string{
		"DELETE department_members WHERE department_id = $id",
		"DELETE folder_grants WHERE principal_type = 'department' AND principal_id = $id",
		"DELETE document_grants WHERE principal_type = 'department' AND principal_id = $id",
		"DELETE $record",
	} {
		if _, err := surrealdb.Query[any](ctx, s.db, statement, vars); err != nil {
			return err
		}
	}
	return nil
}

func (s *AdministrationService) RemoveMember(ctx context.Context, orgID, departmentID, userID string) error {
	if _, err := s.department(ctx, orgID, departmentID); err != nil {
		return err
	}
	_, err := surrealdb.Query[any](ctx, s.db, "DELETE department_members WHERE department_id = $department AND user_id = $user", map[string]any{"department": departmentID, "user": userID})
	return err
}

type GroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GroupView is a group with its member count for the admin list.
type GroupView struct {
	database.Group
	MemberCount int `json:"member_count"`
}

func (s *AdministrationService) Groups(ctx context.Context, orgID string) ([]GroupView, error) {
	groups, err := queryRows[database.Group](ctx, s.db, "SELECT * FROM groups WHERE org_id = $org ORDER BY name", map[string]any{"org": orgID})
	if err != nil {
		return nil, err
	}
	views := make([]GroupView, 0, len(groups))
	for _, group := range groups {
		count, _ := queryCount(ctx, s.db, "SELECT count() AS count FROM group_members WHERE group_id = $group GROUP ALL", map[string]any{"group": string(group.ID)})
		views = append(views, GroupView{Group: group, MemberCount: count})
	}
	return views, nil
}

func (s *AdministrationService) CreateGroup(ctx context.Context, orgID, actor string, in GroupInput) (*database.Group, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("group name is required")
	}
	now := time.Now().UTC()
	return surrealdb.Create[database.Group](ctx, s.db, "groups", &database.Group{OrgID: orgID, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), CreatedBy: actor, CreatedAt: now, UpdatedAt: now})
}

func (s *AdministrationService) UpdateGroup(ctx context.Context, orgID, id string, in GroupInput) (*database.Group, error) {
	if _, err := s.group(ctx, orgID, id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("group name is required")
	}
	return surrealdb.Merge[database.Group](ctx, s.db, mustRecordID(id), map[string]any{"name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "updated_at": time.Now().UTC()})
}

func (s *AdministrationService) DeleteGroup(ctx context.Context, orgID, id string) error {
	if _, err := s.group(ctx, orgID, id); err != nil {
		return err
	}
	vars := map[string]any{"id": id, "record": mustRecordID(id)}
	for _, statement := range []string{
		"DELETE group_members WHERE group_id = $id",
		"DELETE folder_grants WHERE principal_type = 'group' AND principal_id = $id",
		"DELETE document_grants WHERE principal_type = 'group' AND principal_id = $id",
		"DELETE $record",
	} {
		if _, err := surrealdb.Query[any](ctx, s.db, statement, vars); err != nil {
			return err
		}
	}
	return nil
}

// GroupMemberView is a group member with profile details.
type GroupMemberView struct {
	database.GroupMember
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *AdministrationService) GroupMembers(ctx context.Context, orgID, groupID string) ([]GroupMemberView, error) {
	if _, err := s.group(ctx, orgID, groupID); err != nil {
		return nil, err
	}
	members, err := queryRows[database.GroupMember](ctx, s.db, "SELECT * FROM group_members WHERE group_id = $group ORDER BY created_at", map[string]any{"group": groupID})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}
	users := s.orgs.UsersByID(ctx, ids)
	views := make([]GroupMemberView, 0, len(members))
	for _, member := range members {
		view := GroupMemberView{GroupMember: member}
		if user := users[member.UserID]; user != nil {
			view.Name, view.Email = user.Name, user.Email
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AdministrationService) AddGroupMember(ctx context.Context, orgID, groupID, userID, role string) (*database.GroupMember, error) {
	if _, err := s.group(ctx, orgID, groupID); err != nil {
		return nil, err
	}
	if _, err := s.orgs.Membership(ctx, orgID, userID); err != nil {
		return nil, err
	}
	if role != "owner" {
		role = "member"
	}
	existing, err := queryRows[database.GroupMember](ctx, s.db, "SELECT * FROM group_members WHERE group_id = $group AND user_id = $user LIMIT 1", map[string]any{"group": groupID, "user": userID})
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return surrealdb.Merge[database.GroupMember](ctx, s.db, mustRecordID(string(existing[0].ID)), map[string]any{"role": role})
	}
	return surrealdb.Create[database.GroupMember](ctx, s.db, "group_members", &database.GroupMember{OrgID: orgID, GroupID: groupID, UserID: userID, Role: role, CreatedAt: time.Now().UTC()})
}

func (s *AdministrationService) RemoveGroupMember(ctx context.Context, orgID, groupID, userID string) error {
	if _, err := s.group(ctx, orgID, groupID); err != nil {
		return err
	}
	_, err := surrealdb.Query[any](ctx, s.db, "DELETE group_members WHERE group_id = $group AND user_id = $user", map[string]any{"group": groupID, "user": userID})
	return err
}

func (s *AdministrationService) group(ctx context.Context, orgID, id string) (*database.Group, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("group not found")
	}
	group, err := surrealdb.Select[database.Group](ctx, s.db, rid)
	if err != nil || group == nil || group.OrgID != orgID {
		return nil, fmt.Errorf("group not found")
	}
	return group, nil
}
