package service

import (
	"testing"

	"github.com/xmtrxi/edms/internal/database"
)

func TestCanFolderFromSnapshotHonorsInheritedDeny(t *testing.T) {
	folders := map[string]database.Folder{
		"folders:root":  {ID: database.RecordID("folders:root"), InheritPermissions: true},
		"folders:child": {ID: database.RecordID("folders:child"), ParentID: "folders:root", InheritPermissions: true},
	}
	grants := map[string][]database.FolderGrant{
		"folders:root": {{Allow: []string{"view"}, Deny: []string{"view"}, Role: "viewer"}},
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
	grants := map[string][]database.FolderGrant{"folders:root": {{Role: "viewer"}}}
	if !canFolderFromSnapshot("folders:child", "view", folders, grants) {
		t.Fatal("viewer role should grant inherited view")
	}
}
