package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/database"
)

func TestCanFolderFromSnapshotHonorsInheritedDeny(t *testing.T) {
	folders := map[string]database.Folder{
		"folders:root":  {ID: database.RecordID("folders:root"), InheritPermissions: true},
		"folders:child": {ID: database.RecordID("folders:child"), ParentID: "folders:root", InheritPermissions: true},
	}
	grants := map[string][]grantView{
		"folders:root": {{allow: []string{"view"}, deny: []string{"view"}, role: "viewer"}},
	}
	if canFolderFromSnapshot("folders:child", "view", folders, grants) {
		t.Fatal("deny must win over inherited allow")
	}
}

func TestCanFolderFromSnapshotUsesInheritedRole(t *testing.T) {
	folders := map[string]database.Folder{
		"folders:root":  {ID: database.RecordID("folders:root"), InheritPermissions: true},
		"folders:child": {ID: database.RecordID("folders:child"), ParentID: "folders:root", InheritPermissions: true},
	}
	grants := map[string][]grantView{"folders:root": {{role: "viewer"}}}
	if !canFolderFromSnapshot("folders:child", "view", folders, grants) {
		t.Fatal("viewer role should grant inherited view")
	}
}

func TestCanFolderFromSnapshotStopsAtBrokenInheritance(t *testing.T) {
	folders := map[string]database.Folder{
		"folders:root":  {ID: database.RecordID("folders:root"), InheritPermissions: true},
		"folders:child": {ID: database.RecordID("folders:child"), ParentID: "folders:root", InheritPermissions: false},
	}
	grants := map[string][]grantView{"folders:root": {{role: "manager"}}}
	if canFolderFromSnapshot("folders:child", "view", folders, grants) {
		t.Fatal("a folder that does not inherit must ignore its parent's grants")
	}
}

func TestDecideWithoutGrantsKeepsLooking(t *testing.T) {
	if _, found := decide(nil, "view"); found {
		t.Fatal("no grants must not decide")
	}
	if allowed, found := decide([]grantView{{role: "uploader"}}, "view"); !found || allowed {
		t.Fatal("a grant that does not include the capability decides deny at its level")
	}
}

func TestDescendantDepartments(t *testing.T) {
	departments := []database.Department{
		{ID: "departments:finance"},
		{ID: "departments:payroll", ParentID: "departments:finance"},
		{ID: "departments:ap", ParentID: "departments:finance"},
		{ID: "departments:ap_east", ParentID: "departments:ap"},
		{ID: "departments:hr"},
	}
	got := descendantDepartments(departments, []string{"departments:finance"})
	slices.Sort(got)
	want := []string{"departments:ap", "departments:ap_east", "departments:payroll"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPrincipalsHas(t *testing.T) {
	p := &Principals{keys: map[string]bool{principalKey("department", "departments:finance"): true}}
	if !p.Has("department", "departments:finance") || p.Has("department", "departments:hr") {
		t.Fatal("principal lookup is wrong")
	}
}

func TestScopeClauseAdminExcludesRestricted(t *testing.T) {
	member, _ := (&VisibleScope{OrgID: "organizations:a", UserID: "users:u"}).Clause()
	admin, vars := (&VisibleScope{OrgID: "organizations:a", UserID: "users:u", Admin: true}).Clause()
	if strings.Contains(member, "OR sensitivity != 'restricted'") {
		t.Fatal("members must not get the admin clause")
	}
	if !strings.Contains(admin, "OR sensitivity != 'restricted'") {
		t.Fatal("admins see everything except restricted documents")
	}
	if !strings.HasPrefix(admin, "org_id = $scope_org") {
		t.Fatal("scope must always be limited to the organization")
	}
	if folders, ok := vars["scope_folders"].([]string); !ok || folders == nil {
		t.Fatal("empty scopes must bind empty arrays, not null")
	}
}

func TestShareCapabilities(t *testing.T) {
	viewer := shareCapabilities(database.Share{Permission: database.ShareRoleViewer})
	if slices.Contains(viewer, "download") {
		t.Fatal("viewer links without download must not allow download")
	}
	if !slices.Contains(shareCapabilities(database.Share{Permission: "download"}), "download") {
		t.Fatal("legacy download shares must still allow download")
	}
	if !slices.Contains(shareCapabilities(database.Share{Permission: database.ShareRoleEditor}), "edit") {
		t.Fatal("editors can edit")
	}
}
