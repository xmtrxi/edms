package service

import (
	"context"
	"fmt"
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
