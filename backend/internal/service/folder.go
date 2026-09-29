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
	ErrFolderNameTaken     = errors.New("a folder with this name already exists here")
)

// FolderService manages the organization's folder tree. Folders belong to the
// organization; the creator is recorded but access comes from grants.
type FolderService struct {
	DB     *surrealdb.DB
	access *AccessService
}

func NewFolderService(db *surrealdb.DB) *FolderService {
	return &FolderService{DB: db, access: NewAccessService(db)}
}

type FolderInput struct {
	Name               string
	ParentID           string
	Description        string
	Color              string
	InheritPermissions bool
}

// List returns the folders the user can view, with the number of documents
// in each that the user can see.
func (s *FolderService) List(ctx context.Context, userID string) ([]dto.ManagedFolderResponse, error) {
	orgID := orgOf(ctx)
	visible, err := s.access.ViewableFolderIDs(ctx, userID, "view")
	if err != nil {
		return nil, err
	}
	if len(visible) == 0 {
		return []dto.ManagedFolderResponse{}, nil
	}
	folders, err := queryRows[database.Folder](ctx, s.DB,
		"SELECT * FROM folders WHERE org_id = $org AND <string> id INSIDE $ids ORDER BY depth, normalized_name",
		map[string]any{"org": orgID, "ids": visible})
	if err != nil {
		return nil, err
	}
	counts, err := s.visibleCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	manageable, err := s.access.ViewableFolderIDs(ctx, userID, "manage_folder")
	if err != nil {
		return nil, err
	}
	uploadable, err := s.access.ViewableFolderIDs(ctx, userID, "upload")
	if err != nil {
		return nil, err
	}
	responses := make([]dto.ManagedFolderResponse, 0, len(folders))
	for i := range folders {
		id := string(folders[i].ID)
		response := dto.NewManagedFolderResponse(&folders[i], counts[id])
		response.CanManage = slices.Contains(manageable, id)
		response.CanUpload = slices.Contains(uploadable, id)
		responses = append(responses, response)
	}
	return responses, nil
}

func (s *FolderService) visibleCounts(ctx context.Context, userID string) (map[string]int, error) {
	scope, err := s.access.Scope(ctx, userID, orgOf(ctx))
	if err != nil {
		return nil, err
	}
	clause, vars := scope.Clause()
	type countRow struct {
		FolderID string `json:"folder_id"`
		Total    int    `json:"total"`
	}
	rows, err := queryRows[countRow](ctx, s.DB,
		"SELECT folder_id, count() AS total FROM documents WHERE "+clause+" AND folder_id != NONE AND status NOT IN ['archived', 'trashed'] GROUP BY folder_id", vars)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.FolderID] = row.Total
	}
	return counts, nil
}

// Get returns a folder the user can view.
func (s *FolderService) Get(ctx context.Context, userID, id string) (*database.Folder, error) {
	return s.getFor(ctx, userID, "view", id)
}

func (s *FolderService) getFor(ctx context.Context, userID, capability, id string) (*database.Folder, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrFolderNotFound
	}
	folder, err := surrealdb.Select[database.Folder](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	if folder == nil || folder.ID == "" || (folder.OrgID != "" && folder.OrgID != orgOf(ctx)) {
		return nil, ErrFolderNotFound
	}
	allowed, err := s.access.CanFolder(ctx, userID, capability, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		if capability != "view" {
			if visible, _ := s.access.CanFolder(ctx, userID, "view", id); visible {
				return nil, ErrAccessDenied
			}
		}
		return nil, ErrFolderNotFound
	}
	return folder, nil
}

// Create adds a folder. Top-level folders can be created by any member who is
// not a guest; subfolders need upload access to the parent.
func (s *FolderService) Create(ctx context.Context, userID string, input FolderInput) (*dto.ManagedFolderResponse, error) {
	input, err := normalizeFolderInput(input)
	if err != nil {
		return nil, err
	}
	actor, _ := ActorFrom(ctx)
	if input.ParentID == "" && actor.IsGuest() {
		return nil, ErrAccessDenied
	}
	ancestors, depth, err := s.parentDetails(ctx, userID, "", input.ParentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUniqueName(ctx, "", input.ParentID, input.Name); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	folder, err := surrealdb.Create[database.Folder](ctx, s.DB, "folders", &database.Folder{
		OrgID: orgOf(ctx), OwnerID: userID, ParentID: input.ParentID, AncestorIDs: ancestors, Depth: depth,
		Name: input.Name, NormalizedName: strings.ToLower(input.Name), Description: input.Description,
		Color: input.Color, InheritPermissions: input.InheritPermissions, PermissionVersion: 1,
		CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	response := dto.NewManagedFolderResponse(folder, 0)
	response.CanManage, response.CanUpload = true, true
	return &response, nil
}

func (s *FolderService) Update(ctx context.Context, userID, id string, input FolderInput) (*dto.ManagedFolderResponse, error) {
	folder, err := s.getFor(ctx, userID, "manage_folder", id)
	if err != nil {
		return nil, err
	}
	input, err = normalizeFolderInput(input)
	if err != nil {
		return nil, err
	}
	if input.ParentID != folder.ParentID {
		if err := s.moveSubtree(ctx, userID, folder, input.ParentID); err != nil {
			return nil, err
		}
	}
	if !strings.EqualFold(input.Name, folder.Name) || input.ParentID != folder.ParentID {
		if err := s.ensureUniqueName(ctx, id, input.ParentID, input.Name); err != nil {
			return nil, err
		}
	}
	rid, _ := database.ParseRecordID(id)
	if _, err := surrealdb.Merge[database.Folder](ctx, s.DB, rid, map[string]any{
		"name": input.Name, "normalized_name": strings.ToLower(input.Name), "description": input.Description,
		"color": input.Color, "inherit_permissions": input.InheritPermissions, "updated_at": time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	updated, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	counts, err := s.visibleCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	response := dto.NewManagedFolderResponse(updated, counts[id])
	response.CanManage, response.CanUpload = true, true
	return &response, nil
}

// ListGrants shows everyone with access to the folder, including grants
// inherited from parent folders. Anyone who can view the folder can see it.
func (s *FolderService) ListGrants(ctx context.Context, userID, folderID string) ([]GrantView, error) {
	folder, err := s.Get(ctx, userID, folderID)
	if err != nil {
		return nil, err
	}
	views := []GrantView{}
	current := folder
	for depth := 0; current != nil; depth++ {
		grants, err := queryRows[database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE folder_id = $folder ORDER BY created_at", map[string]any{"folder": string(current.ID)})
		if err != nil {
			return nil, err
		}
		source := "folder"
		if depth > 0 {
			source = "inherited"
		}
		for _, grant := range grants {
			views = append(views, GrantView{ID: string(grant.ID), PrincipalType: grant.PrincipalType, PrincipalID: grant.PrincipalID, Role: grant.Role, Allow: grant.Allow, Deny: grant.Deny, Source: source, SourceName: current.Name})
		}
		if current.ParentID == "" || !current.InheritPermissions {
			break
		}
		if current, err = s.access.folder(ctx, current.ParentID); err != nil {
			return nil, err
		}
	}
	return views, nil
}

// UpsertGrant adds or changes a grant. Sharing a folder needs the share capability.
func (s *FolderService) UpsertGrant(ctx context.Context, userID, folderID string, input GrantInput) (*database.FolderGrant, error) {
	folder, err := s.getFor(ctx, userID, "share", folderID)
	if err != nil {
		return nil, err
	}
	input, err = normalizeGrant(ctx, s.DB, folder.OrgID, input, false)
	if err != nil {
		return nil, err
	}
	existing, err := queryRows[database.FolderGrant](ctx, s.DB, "SELECT * FROM folder_grants WHERE folder_id = $folder AND principal_type = $type AND principal_id = $principal LIMIT 1", map[string]any{"folder": folderID, "type": input.PrincipalType, "principal": input.PrincipalID})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if len(existing) > 0 {
		rid, _ := database.ParseRecordID(string(existing[0].ID))
		return surrealdb.Merge[database.FolderGrant](ctx, s.DB, rid, map[string]any{"role": input.Role, "allow": nonNil(input.Allow), "deny": nonNil(input.Deny), "updated_at": now})
	}
	grant, err := surrealdb.Create[database.FolderGrant](ctx, s.DB, "folder_grants", &database.FolderGrant{OrgID: folder.OrgID, FolderID: folderID, PrincipalType: input.PrincipalType, PrincipalID: input.PrincipalID, Role: input.Role, Allow: nonNil(input.Allow), Deny: nonNil(input.Deny), CreatedBy: userID, CreatedAt: now, UpdatedAt: now})
	if err == nil {
		_ = writeAudit(ctx, s.DB, userID, "folder.shared", "folder", folderID, map[string]any{"principal_type": input.PrincipalType, "principal_id": input.PrincipalID, "role": input.Role})
	}
	return grant, err
}

func (s *FolderService) DeleteGrant(ctx context.Context, userID, folderID, grantID string) error {
	if _, err := s.getFor(ctx, userID, "share", folderID); err != nil {
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
	if _, err = surrealdb.Delete[database.FolderGrant](ctx, s.DB, rid); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.DB, userID, "folder.unshared", "folder", folderID, map[string]any{"principal_type": grant.PrincipalType, "principal_id": grant.PrincipalID})
	return nil
}

func (s *FolderService) moveSubtree(ctx context.Context, userID string, folder *database.Folder, parentID string) error {
	ancestors, depth, err := s.parentDetails(ctx, userID, string(folder.ID), parentID)
	if err != nil {
		return err
	}
	descendants, err := queryRows[database.Folder](ctx, s.DB, "SELECT * FROM folders WHERE org_id = $org AND $id INSIDE ancestor_ids", map[string]any{"org": folder.OrgID, "id": string(folder.ID)})
	if err != nil {
		return err
	}
	items := append([]database.Folder{*folder}, descendants...)
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

func (s *FolderService) Delete(ctx context.Context, userID, id string) error {
	if _, err := s.getFor(ctx, userID, "manage_folder", id); err != nil {
		return err
	}
	children, err := queryRows[database.Folder](ctx, s.DB, "SELECT * FROM folders WHERE parent_id = $parent LIMIT 1", map[string]any{"parent": id})
	if err != nil {
		return err
	}
	// Every document counts here, including ones this user cannot see.
	count, err := queryCount(ctx, s.DB, "SELECT count() AS count FROM documents WHERE folder_id = $folder AND status != 'trashed' GROUP ALL", map[string]any{"folder": id})
	if err != nil {
		return err
	}
	if count > 0 || len(children) > 0 {
		return ErrFolderNotEmpty
	}
	rid, _ := database.ParseRecordID(id)
	if _, err = surrealdb.Delete[database.Folder](ctx, s.DB, rid); err != nil {
		return err
	}
	_, err = surrealdb.Query[any](ctx, s.DB, "DELETE folder_grants WHERE folder_id = $folder", map[string]any{"folder": id})
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

func (s *FolderService) ensureUniqueName(ctx context.Context, folderID, parentID, name string) error {
	query := "SELECT * FROM folders WHERE org_id = $org AND normalized_name = $name AND "
	vars := map[string]any{"org": orgOf(ctx), "name": strings.ToLower(name)}
	if parentID == "" {
		query += "(parent_id = NONE OR parent_id = '')"
	} else {
		query += "parent_id = $parent"
		vars["parent"] = parentID
	}
	existing, err := queryRows[database.Folder](ctx, s.DB, query+" LIMIT 2", vars)
	if err != nil {
		return err
	}
	for _, folder := range existing {
		if string(folder.ID) != folderID {
			return ErrFolderNameTaken
		}
	}
	return nil
}

// parentDetails validates a new parent: it must be in the organization, the
// user must be able to add to it, and it cannot be the folder or a descendant.
func (s *FolderService) parentDetails(ctx context.Context, userID, folderID, parentID string) ([]string, int, error) {
	if parentID == "" {
		return nil, 0, nil
	}
	if parentID == folderID {
		return nil, 0, ErrInvalidFolderParent
	}
	parent, err := s.getFor(ctx, userID, "upload", parentID)
	if err != nil {
		if errors.Is(err, ErrAccessDenied) {
			return nil, 0, err
		}
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
