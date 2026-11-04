package migrations

import (
	"context"
	"fmt"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

type addOrganizations struct{}

func (m addOrganizations) Name() string { return "026_add_organizations" }

// orgScopedTables hold records that belong to one organization. Existing rows
// are moved into the default organization when this migration first runs.
var orgScopedTables = []string{
	"documents", "folders", "folder_grants", "categories", "tags", "shares",
	"departments", "department_members", "invitations", "form_templates",
	"form_submissions", "visitor_requests", "workflow_requests", "signing_requests",
	"audit_events", "legal_holds", "ai_jobs", "processing_jobs",
}

// orgIndexedTables are listed by organization on hot paths.
var orgIndexedTables = []string{"documents", "folders", "folder_grants", "categories", "departments", "department_members", "audit_events"}

func (m addOrganizations) Up(ctx context.Context, schema surrealgoorm.Schema) error {
	tables := map[string]func(*surrealgoorm.Table){
		"organizations": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("slug").Unique()
			t.String("name")
			t.String("kind")
			t.String("owner_id")
			t.Object("settings")
			t.Timestamps()
		},
		"memberships": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("user_id")
			t.String("role")
			t.String("status")
			t.String("invited_by").Nullable()
			t.Datetime("joined_at")
			t.Timestamps()
		},
		"groups": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("name")
			t.String("description").Nullable()
			t.String("created_by")
			t.Timestamps()
		},
		"group_members": func(t *surrealgoorm.Table) {
			t.ID()
			t.String("org_id")
			t.String("group_id")
			t.String("user_id")
			t.String("role")
			t.Datetime("created_at")
		},
	}
	for _, name := range []string{"organizations", "memberships", "groups", "group_members"} {
		if err := createTable(ctx, schema, name, tables[name]); err != nil {
			return err
		}
	}

	ddl := "DEFINE FIELD last_org_id ON users TYPE option<string>;" +
		"DEFINE FIELD parent_id ON departments TYPE option<string>;" +
		"DEFINE FIELD role ON invitations TYPE option<string>;" +
		"DEFINE FIELD org_id ON roles TYPE option<string>;" +
		"DEFINE INDEX memberships_org_user ON memberships FIELDS org_id, user_id UNIQUE;" +
		"DEFINE INDEX memberships_user ON memberships FIELDS user_id;" +
		"DEFINE INDEX group_members_group_user ON group_members FIELDS group_id, user_id UNIQUE;" +
		"DEFINE INDEX group_members_user ON group_members FIELDS user_id;"
	for _, table := range orgScopedTables {
		ddl += fmt.Sprintf("DEFINE FIELD org_id ON %s TYPE option<string>;", table)
	}
	for _, table := range orgIndexedTables {
		ddl += fmt.Sprintf("DEFINE INDEX %s_org ON %s FIELDS org_id;", table, table)
	}
	if err := execDDL(ctx, schema.Db, ddl); err != nil {
		return err
	}
	// Role keys were globally unique; each organization now owns its own roles.
	if err := execDDL(ctx, schema.Db, "REMOVE INDEX IF EXISTS key_unique ON roles; DEFINE INDEX roles_org_key ON roles FIELDS org_id, key UNIQUE;"); err != nil {
		return err
	}
	return backfillDefaultOrganization(ctx, schema.Db)
}

// backfillDefaultOrganization moves an existing single-organization install
// into an explicit organization. Fresh installs have no users and are left
// for the startup bootstrap, which knows the configured organization name.
func backfillDefaultOrganization(ctx context.Context, db *surrealdb.DB) error {
	users, err := rows[database.User](ctx, db, "SELECT * FROM users ORDER BY created_at", nil)
	if err != nil || len(users) == 0 {
		return err
	}
	admins, err := rows[database.CasbinRule](ctx, db, "SELECT * FROM casbin_rules WHERE ptype = 'g' AND v1 = 'organization_admin' ORDER BY created_at", nil)
	if err != nil {
		return err
	}
	isAdmin := map[string]bool{}
	for _, rule := range admins {
		isAdmin[rule.V0] = true
	}
	ownerID := string(users[0].ID)
	if len(admins) > 0 {
		ownerID = admins[0].V0
	}

	orgs, err := rows[database.Organization](ctx, db, "SELECT * FROM organizations WHERE kind = $kind ORDER BY created_at LIMIT 1", map[string]any{"kind": database.OrganizationKindOrganization})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var orgID string
	if len(orgs) > 0 {
		orgID = string(orgs[0].ID)
	} else {
		created, err := surrealdb.Create[database.Organization](ctx, db, "organizations", &database.Organization{
			Slug: "default", Name: "Default organization", Kind: database.OrganizationKindOrganization,
			OwnerID: ownerID, Settings: map[string]any{}, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("create default organization: %w", err)
		}
		orgID = string(created.ID)
	}

	vars := map[string]any{"org": orgID}
	for _, table := range orgScopedTables {
		if _, err := surrealdb.Query[any](ctx, db, fmt.Sprintf("UPDATE %s SET org_id = $org WHERE org_id = NONE OR org_id = '';", table), vars); err != nil {
			return fmt.Errorf("assign %s to default organization: %w", table, err)
		}
	}
	if _, err := surrealdb.Query[any](ctx, db, "UPDATE roles SET org_id = $org WHERE (org_id = NONE OR org_id = '') AND protected != true;", vars); err != nil {
		return err
	}
	if _, err := surrealdb.Query[any](ctx, db, "UPDATE casbin_rules SET v2 = $org WHERE ptype = 'g' AND v2 = 'org';", vars); err != nil {
		return err
	}
	if _, err := surrealdb.Query[any](ctx, db, "UPDATE casbin_rules SET v1 = $org WHERE ptype = 'p' AND v1 = '*' AND v0 != 'organization_admin';", vars); err != nil {
		return err
	}
	if _, err := surrealdb.Query[any](ctx, db, "UPDATE users SET last_org_id = $org WHERE last_org_id = NONE OR last_org_id = '';", vars); err != nil {
		return err
	}

	existing, err := rows[database.Membership](ctx, db, "SELECT * FROM memberships WHERE org_id = $org", vars)
	if err != nil {
		return err
	}
	member := map[string]bool{}
	for _, m := range existing {
		member[m.UserID] = true
	}
	for _, user := range users {
		id := string(user.ID)
		if member[id] {
			continue
		}
		role := database.MembershipRoleMember
		switch {
		case id == ownerID:
			role = database.MembershipRoleOwner
		case isAdmin[id]:
			role = database.MembershipRoleAdmin
		}
		status := database.MembershipStatusActive
		if user.Status == database.UserStatusDisabled {
			status = database.MembershipStatusSuspended
		}
		if _, err := surrealdb.Create[database.Membership](ctx, db, "memberships", &database.Membership{
			OrgID: orgID, UserID: id, Role: role, Status: status, JoinedAt: user.CreatedAt, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("create membership for %s: %w", id, err)
		}
	}
	return nil
}

func rows[T any](ctx context.Context, db *surrealdb.DB, statement string, vars map[string]any) ([]T, error) {
	result, err := surrealdb.Query[[]T](ctx, db, statement, vars)
	if err != nil || result == nil || len(*result) == 0 {
		return nil, err
	}
	return (*result)[0].Result, nil
}

func (m addOrganizations) Down(ctx context.Context, schema surrealgoorm.Schema) error {
	for _, name := range []string{"group_members", "groups", "memberships", "organizations"} {
		if err := removeTable(ctx, schema, name); err != nil {
			return err
		}
	}
	return nil
}
