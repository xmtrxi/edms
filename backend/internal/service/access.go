package service

import (
	"context"
	"errors"
	"slices"

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
	"viewer":      {"view", "download"},
	"uploader":    {"upload"},
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

func (s *AccessService) CanDocument(ctx context.Context, userID, capability string, document *database.Document) (bool, error) {
	if document == nil {
		return false, ErrDocumentNotFound
	}
	if document.OwnerID == userID {
		return true, nil
	}
	if document.FolderID == "" {
		return false, nil
	}
	return s.CanFolder(ctx, userID, capability, document.FolderID)
}

func (s *AccessService) CanFolder(ctx context.Context, userID, capability, folderID string) (bool, error) {
	folder, err := s.folder(ctx, folderID)
	if err != nil || folder == nil {
		return false, err
	}
	if folder.OwnerID == userID {
		return true, nil
	}

	current := folder
	for {
		decision, found, err := s.grantDecision(ctx, userID, capability, string(current.ID))
		if err != nil {
			return false, err
		}
		if found {
			return decision, nil
		}
		if current.ParentID == "" || !current.InheritPermissions {
			return false, nil
		}
		current, err = s.folder(ctx, current.ParentID)
		if err != nil || current == nil {
			return false, err
		}
	}
}

func (s *AccessService) folder(ctx context.Context, id string) (*database.Folder, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrFolderNotFound
	}
	return surrealdb.Select[database.Folder](ctx, s.DB, rid)
}

func (s *AccessService) grantDecision(ctx context.Context, userID, capability, folderID string) (bool, bool, error) {
	grants, err := surrealdb.Query[[]database.FolderGrant](ctx, s.DB,
		"SELECT * FROM folder_grants WHERE folder_id = $folder", map[string]any{"folder": folderID})
	if err != nil {
		return false, false, err
	}
	if grants == nil || len(*grants) == 0 {
		return false, false, nil
	}
	items := (*grants)[0].Result
	if len(items) == 0 {
		return false, false, nil
	}
	departmentIDs, err := s.departmentIDs(ctx, userID)
	if err != nil {
		return false, false, err
	}
	applicable := make([]database.FolderGrant, 0, len(items))
	for _, grant := range items {
		if grant.PrincipalType == "user" && grant.PrincipalID == userID || grant.PrincipalType == "department" && slices.Contains(departmentIDs, grant.PrincipalID) {
			applicable = append(applicable, grant)
		}
	}
	if len(applicable) == 0 {
		return false, false, nil
	}
	for _, grant := range applicable {
		if slices.Contains(grant.Deny, capability) {
			return false, true, nil
		}
	}
	for _, grant := range applicable {
		if slices.Contains(grant.Allow, capability) {
			return true, true, nil
		}
	}
	for _, grant := range applicable {
		if slices.Contains(roleCapabilities[grant.Role], capability) {
			return true, true, nil
		}
	}
	return false, true, nil
}

func (s *AccessService) departmentIDs(ctx context.Context, userID string) ([]string, error) {
	result, err := surrealdb.Query[[]database.DepartmentMember](ctx, s.DB, "SELECT * FROM department_members WHERE user_id = $user", map[string]any{"user": userID})
	if err != nil || result == nil || len(*result) == 0 {
		return nil, err
	}
	ids := make([]string, 0, len((*result)[0].Result))
	for _, membership := range (*result)[0].Result {
		ids = append(ids, membership.DepartmentID)
	}
	return ids, nil
}
