package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
)

var (
	ErrFolderNotFound      = errors.New("folder not found")
	ErrFolderNotEmpty      = errors.New("folder contains documents or subfolders")
	ErrInvalidFolderParent = errors.New("invalid folder parent")
)

type FolderService struct{ DB *surrealdb.DB }

func NewFolderService(db *surrealdb.DB) *FolderService { return &FolderService{DB: db} }

type FolderInput struct {
	Name               string
	ParentID           string
	Description        string
	Color              string
	InheritPermissions bool
}

type FolderGrantInput struct {
	PrincipalType, PrincipalID, Role string
	Allow, Deny                      []string
}

func (s *FolderService) List(ctx context.Context, ownerID string) ([]dto.ManagedFolderResponse, error) {
	result, err := surrealdb.Query[[]database.Folder](ctx, s.DB,
		"SELECT * FROM folders WHERE owner_id = $owner ORDER BY depth, normalized_name",
		map[string]any{"owner": ownerID})
	if err != nil {
		return nil, err
	}
	folders := []database.Folder{}
	if result != nil && len(*result) > 0 {
		folders = (*result)[0].Result
	}
	responses := make([]dto.ManagedFolderResponse, 0, len(folders))
	for i := range folders {
		count, err := s.documentCount(ctx, ownerID, string(folders[i].ID))
		if err != nil {
			return nil, err
		}
		responses = append(responses, dto.NewManagedFolderResponse(&folders[i], count))
	}
	return responses, nil
}

func (s *FolderService) Get(ctx context.Context, ownerID, id string) (*database.Folder, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrFolderNotFound
	}
	folder, err := surrealdb.Select[database.Folder](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	if folder == nil || folder.OwnerID != ownerID {
		return nil, ErrFolderNotFound
	}
	return folder, nil
}

func (s *FolderService) Create(ctx context.Context, ownerID string, input FolderInput) (*dto.ManagedFolderResponse, error) {
	input, err := normalizeFolderInput(input)
	if err != nil {
		return nil, err
	}
	ancestors, depth, err := s.parentDetails(ctx, ownerID, "", input.ParentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	folder, err := surrealdb.Create[database.Folder](ctx, s.DB, "folders", &database.Folder{
		OwnerID: ownerID, ParentID: input.ParentID, AncestorIDs: ancestors, Depth: depth,
		Name: input.Name, NormalizedName: strings.ToLower(input.Name), Description: input.Description,
		Color: input.Color, InheritPermissions: input.InheritPermissions, PermissionVersion: 1,
		CreatedBy: ownerID, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	response := dto.NewManagedFolderResponse(folder, 0)
	return &response, nil
}

func (s *FolderService) Update(ctx context.Context, ownerID, id string, input FolderInput) (*dto.ManagedFolderResponse, error) {
	folder, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	input, err = normalizeFolderInput(input)
	if err != nil {
		return nil, err
	}
	if input.ParentID != folder.ParentID {
		if err := s.moveSubtree(ctx, ownerID, folder, input.ParentID); err != nil {
			return nil, err
		}
	}
	rid, _ := database.ParseRecordID(id)
	updated, err := surrealdb.Merge[database.Folder](ctx, s.DB, rid, map[string]any{
		"name": input.Name, "normalized_name": strings.ToLower(input.Name), "description": input.Description,
		"color": input.Color, "inherit_permissions": input.InheritPermissions, "updated_at": time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	count, err := s.documentCount(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	updated, err = s.Get(ctx, ownerID, string(updated.ID))
	if err != nil {
		return nil, err
	}
	response := dto.NewManagedFolderResponse(updated, count)
	return &response, nil
}

func (s *FolderService) ListGrants(ctx context.Context, ownerID, folderID string) ([]database.FolderGrant, error) {
	if _, err := s.Get(ctx, ownerID, folderID); err != nil {
		return nil, err
	}
	result, err := surrealdb.Query[[]database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE folder_id = $folder ORDER BY created_at", map[string]any{"folder": folderID})
	if err != nil || result == nil || len(*result) == 0 {
		return nil, err
	}
	return (*result)[0].Result, nil
}

func (s *FolderService) UpsertGrant(ctx context.Context, ownerID, folderID string, input FolderGrantInput) (*database.FolderGrant, error) {
	if _, err := s.Get(ctx, ownerID, folderID); err != nil {
		return nil, err
	}
	input.PrincipalType, input.PrincipalID, input.Role = strings.TrimSpace(input.PrincipalType), strings.TrimSpace(input.PrincipalID), strings.TrimSpace(input.Role)
	if (input.PrincipalType != "user" && input.PrincipalType != "department") || input.PrincipalID == "" {
		return nil, errors.New("select a user or department")
	}
	if _, ok := roleCapabilities[input.Role]; !ok {
		return nil, errors.New("select a valid role")
	}
	result, err := surrealdb.Query[[]database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE folder_id = $folder AND principal_type = $type AND principal_id = $principal LIMIT 1", map[string]any{"folder": folderID, "type": input.PrincipalType, "principal": input.PrincipalID})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		grant := (*result)[0].Result[0]
		rid, _ := database.ParseRecordID(string(grant.ID))
		return surrealdb.Merge[database.FolderGrant](ctx, s.DB, rid, map[string]any{"role": input.Role, "allow": input.Allow, "deny": input.Deny, "updated_at": now})
	}
	return surrealdb.Create[database.FolderGrant](ctx, s.DB, "folder_grants", &database.FolderGrant{FolderID: folderID, PrincipalType: input.PrincipalType, PrincipalID: input.PrincipalID, Role: input.Role, Allow: input.Allow, Deny: input.Deny, CreatedBy: ownerID, CreatedAt: now, UpdatedAt: now})
}

func (s *FolderService) DeleteGrant(ctx context.Context, ownerID, folderID, grantID string) error {
	if _, err := s.Get(ctx, ownerID, folderID); err != nil {
		return err
	}
	rid, err := database.ParseRecordID(grantID)
	if err != nil {
		return ErrFolderNotFound
	}
	grant, err := surrealdb.Select[database.FolderGrant](ctx, s.DB, rid)
	if err != nil {
		return err
	}
	if grant == nil || grant.FolderID != folderID {
		return ErrFolderNotFound
	}
	_, err = surrealdb.Delete[database.FolderGrant](ctx, s.DB, rid)
	return err
}

func (s *FolderService) moveSubtree(ctx context.Context, ownerID string, folder *database.Folder, parentID string) error {
	ancestors, depth, err := s.parentDetails(ctx, ownerID, string(folder.ID), parentID)
	if err != nil {
		return err
	}
	result, err := surrealdb.Query[[]database.Folder](ctx, s.DB, "SELECT * FROM folders WHERE owner_id = $owner AND $id INSIDE ancestor_ids", map[string]any{"owner": ownerID, "id": string(folder.ID)})
	if err != nil {
		return err
	}
	items := []database.Folder{*folder}
	if result != nil && len(*result) > 0 {
		items = append(items, (*result)[0].Result...)
	}
	for _, item := range items {
		itemAncestors, itemDepth, itemParent := ancestors, depth, parentID
		if string(item.ID) != string(folder.ID) {
			index := slices.Index(item.AncestorIDs, string(folder.ID))
			if index < 0 {
				continue
			}
			itemAncestors = append(append(append([]string{}, ancestors...), string(folder.ID)), item.AncestorIDs[index+1:]...)
			itemDepth = len(itemAncestors)
			itemParent = item.ParentID
		}
		rid, _ := database.ParseRecordID(string(item.ID))
		if _, err := surrealdb.Merge[database.Folder](ctx, s.DB, rid, map[string]any{"parent_id": itemParent, "ancestor_ids": itemAncestors, "depth": itemDepth, "permission_version": item.PermissionVersion + 1, "updated_at": time.Now().UTC()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *FolderService) Delete(ctx context.Context, ownerID, id string) error {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return err
	}
	children, err := surrealdb.Query[[]database.Folder](ctx, s.DB, "SELECT * FROM folders WHERE parent_id = $parent LIMIT 1", map[string]any{"parent": id})
	if err != nil {
		return err
	}
	count, err := s.documentCount(ctx, ownerID, id)
	if err != nil {
		return err
	}
	if count > 0 || (children != nil && len(*children) > 0 && len((*children)[0].Result) > 0) {
		return ErrFolderNotEmpty
	}
	rid, _ := database.ParseRecordID(id)
	_, err = surrealdb.Delete[database.Folder](ctx, s.DB, rid)
	return err
}

func normalizeFolderInput(input FolderInput) (FolderInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return input, errors.New("folder name is required")
	}
	if strings.Contains(input.Name, "/") {
		return input, errors.New("folder name cannot contain a slash")
	}
	input.ParentID = strings.TrimSpace(input.ParentID)
	input.Description = strings.TrimSpace(input.Description)
	input.Color = strings.TrimSpace(input.Color)
	return input, nil
}

func (s *FolderService) parentDetails(ctx context.Context, ownerID, folderID, parentID string) ([]string, int, error) {
	if parentID == "" {
		return nil, 0, nil
	}
	if parentID == folderID {
		return nil, 0, ErrInvalidFolderParent
	}
	parent, err := s.Get(ctx, ownerID, parentID)
	if err != nil {
		return nil, 0, ErrInvalidFolderParent
	}
	for _, ancestorID := range parent.AncestorIDs {
		if ancestorID == folderID {
			return nil, 0, ErrInvalidFolderParent
		}
	}
	ancestors := append(append([]string{}, parent.AncestorIDs...), parentID)
	return ancestors, parent.Depth + 1, nil
}

func (s *FolderService) documentCount(ctx context.Context, ownerID, folderID string) (int, error) {
	result, err := surrealdb.Query[[]map[string]any](ctx, s.DB,
		"SELECT count() AS total FROM documents WHERE owner_id = $owner AND folder_id = $folder GROUP ALL",
		map[string]any{"owner": ownerID, "folder": folderID})
	if err != nil || result == nil || len(*result) == 0 || len((*result)[0].Result) == 0 {
		return 0, err
	}
	switch total := (*result)[0].Result[0]["total"].(type) {
	case int64:
		return int(total), nil
	case int:
		return total, nil
	case uint64:
		return int(total), nil
	case float64:
		return int(total), nil
	}
	return 0, nil
}
