package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

const casbinModel = `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub, r.dom) && (p.dom == "*" || r.dom == p.dom) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)`

const OrganizationDomain = "org"
const OrganizationAdminRole = "organization_admin"

// AuthorizationService is the sole policy decision point. Resource services
// use stable object/action pairs instead of hard-coded role switches.
type AuthorizationService struct {
	db       *surrealdb.DB
	enforcer *casbin.Enforcer
	mu       sync.RWMutex
}

func NewAuthorizationService(db *surrealdb.DB) (*AuthorizationService, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, err
	}
	s := &AuthorizationService{db: db}
	e, err := casbin.NewEnforcer(m, &surrealAdapter{db: db})
	if err != nil {
		return nil, err
	}
	s.enforcer = e
	return s, nil
}

func (s *AuthorizationService) Can(userID, domain, object, action string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ok, err := s.enforcer.Enforce(userID, domain, object, action)
	return err == nil && ok
}

func (s *AuthorizationService) Require(userID, domain, object, action string) error {
	if !s.Can(userID, domain, object, action) {
		return ErrAccessDenied
	}
	return nil
}

func (s *AuthorizationService) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enforcer.LoadPolicy()
}

func (s *AuthorizationService) CreateRole(ctx context.Context, key, name, description string, permissions []string) (*database.Role, error) {
	key = normalizeRoleKey(key)
	if key == "" || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("role key and name are required")
	}
	now := time.Now().UTC()
	role, err := surrealdb.Create[database.Role](ctx, s.db, "roles", &database.Role{Key: key, Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, err
	}
	if err = s.ReplaceRolePermissions(ctx, key, permissions); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *AuthorizationService) ReplaceRolePermissions(ctx context.Context, role string, permissions []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = surrealdb.Query[any](ctx, s.db, "DELETE casbin_rules WHERE ptype = 'p' AND v0 = $role", map[string]any{"role": role})
	for _, permission := range permissions {
		object, action, ok := strings.Cut(strings.TrimSpace(permission), ".")
		if !ok || object == "" || action == "" {
			return fmt.Errorf("invalid permission %q", permission)
		}
		if _, err := surrealdb.Create[database.CasbinRule](ctx, s.db, "casbin_rules", &database.CasbinRule{PType: "p", V0: role, V1: "*", V2: object, V3: action, V4: "allow", CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
	}
	return s.enforcer.LoadPolicy()
}

func (s *AuthorizationService) AssignRole(ctx context.Context, userID, role, domain string) error {
	if domain == "" {
		domain = OrganizationDomain
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = surrealdb.Query[any](ctx, s.db, "DELETE casbin_rules WHERE ptype = 'g' AND v0 = $user AND v2 = $domain", map[string]any{"user": userID, "domain": domain})
	if _, err := surrealdb.Create[database.CasbinRule](ctx, s.db, "casbin_rules", &database.CasbinRule{PType: "g", V0: userID, V1: role, V2: domain, CreatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	return s.enforcer.LoadPolicy()
}

func (s *AuthorizationService) BootstrapAdmin(ctx context.Context, userID string) error {
	roles, err := surrealdb.Query[[]database.Role](ctx, s.db, "SELECT * FROM roles WHERE key = $key LIMIT 1", map[string]any{"key": OrganizationAdminRole})
	if err != nil {
		return err
	}
	if roles == nil || len(*roles) == 0 || len((*roles)[0].Result) == 0 {
		now := time.Now().UTC()
		if _, err := surrealdb.Create[database.Role](ctx, s.db, "roles", &database.Role{Key: OrganizationAdminRole, Name: "Organization Admin", Description: "Full system access", Protected: true, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		if err := s.ReplaceRolePermissions(ctx, OrganizationAdminRole, []string{"*.*"}); err != nil {
			return err
		}
	}
	return s.AssignRole(ctx, userID, OrganizationAdminRole, OrganizationDomain)
}

func normalizeRoleKey(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", "_"))
}

type surrealAdapter struct{ db *surrealdb.DB }

func (a *surrealAdapter) LoadPolicy(m model.Model) error {
	if a.db == nil {
		return nil
	}
	ctx := context.Background()
	result, err := surrealdb.Query[[]database.CasbinRule](ctx, a.db, "SELECT * FROM casbin_rules ORDER BY created_at", nil)
	if err != nil || result == nil || len(*result) == 0 {
		return err
	}
	for _, rule := range (*result)[0].Result {
		values := []string{rule.V0, rule.V1, rule.V2, rule.V3, rule.V4, rule.V5}
		for len(values) > 0 && values[len(values)-1] == "" {
			values = values[:len(values)-1]
		}
		line, err := persist.PolicyLineToCsv(rule.PType, values)
		if err != nil {
			return err
		}
		if err := persist.LoadPolicyLine(line, m); err != nil {
			return err
		}
	}
	return nil
}
func (a *surrealAdapter) SavePolicy(m model.Model) error {
	return fmt.Errorf("full policy save is not supported")
}
func (a *surrealAdapter) AddPolicy(string, string, []string) error {
	return fmt.Errorf("use AuthorizationService to mutate policy")
}
func (a *surrealAdapter) RemovePolicy(string, string, []string) error {
	return fmt.Errorf("use AuthorizationService to mutate policy")
}
func (a *surrealAdapter) RemoveFilteredPolicy(string, string, int, ...string) error {
	return fmt.Errorf("use AuthorizationService to mutate policy")
}
