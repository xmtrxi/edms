package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/storage"
)

var ErrLegalHold = errors.New("document is protected by an active legal hold")

type DashboardOverview struct {
	TotalDocuments    int                       `json:"total_documents"`
	ReadyDocuments    int                       `json:"ready_documents"`
	PendingForms      int                       `json:"pending_forms"`
	ActiveWorkflows   int                       `json:"active_workflows"`
	VisitorRequests   int                       `json:"visitor_requests"`
	RecentDocuments   []database.Document       `json:"recent_documents"`
	RecentSubmissions []database.FormSubmission `json:"recent_submissions"`
}

type WorkspaceService struct {
	db      *surrealdb.DB
	cfg     config.Config
	storage storage.Provider
	access  *AccessService
	sharing *SharingService
}

func NewWorkspaceService(db *surrealdb.DB, cfg config.Config, provider storage.Provider) *WorkspaceService {
	return &WorkspaceService{db: db, cfg: cfg, storage: provider, access: NewAccessService(db), sharing: NewSharingService(cfg, db, nil)}
}

func (s *WorkspaceService) Dashboard(ctx context.Context, actor string) (*DashboardOverview, error) {
	scope, err := s.access.Scope(ctx, actor, orgOf(ctx))
	if err != nil {
		return nil, err
	}
	clause, vars := scope.Clause()
	documents, err := queryRows[database.Document](ctx, s.db, "SELECT * FROM documents WHERE "+clause+" AND status != 'trashed' ORDER BY updated_at DESC LIMIT 8", vars)
	if err != nil {
		return nil, err
	}
	submissions, err := queryRows[database.FormSubmission](ctx, s.db, "SELECT * FROM form_submissions WHERE submitted_by = $actor ORDER BY updated_at DESC LIMIT 8", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	workflows, err := queryRows[database.WorkflowRequest](ctx, s.db, "SELECT * FROM workflow_requests WHERE created_by = $actor AND status IN ['active', 'waiting_for_changes', 'waiting_for_signature']", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	visitors, err := queryRows[database.VisitorRequest](ctx, s.db, "SELECT * FROM visitor_requests WHERE created_by = $actor AND status != 'revoked'", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	total, err := queryCount(ctx, s.db, "SELECT count() AS count FROM documents WHERE "+clause+" AND status != 'trashed' GROUP ALL", vars)
	if err != nil {
		return nil, err
	}
	ready, err := queryCount(ctx, s.db, "SELECT count() AS count FROM documents WHERE "+clause+" AND status = 'ready' GROUP ALL", vars)
	if err != nil {
		return nil, err
	}
	pending, err := queryCount(ctx, s.db, "SELECT count() AS count FROM form_submissions WHERE submitted_by = $actor AND status = 'awaiting_approval' GROUP ALL", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	return &DashboardOverview{TotalDocuments: total, ReadyDocuments: ready, PendingForms: pending, ActiveWorkflows: len(workflows), VisitorRequests: len(visitors), RecentDocuments: documents, RecentSubmissions: submissions}, nil
}

func (s *WorkspaceService) Favorites(ctx context.Context, actor string) ([]database.Document, error) {
	favorites, err := queryRows[database.DocumentFavorite](ctx, s.db, "SELECT * FROM document_favorites WHERE user_id = $actor ORDER BY created_at DESC", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	items := []database.Document{}
	for i := range favorites {
		if document, getErr := s.documentFor(ctx, actor, "view", favorites[i].DocumentID); getErr == nil {
			items = append(items, *document)
		}
	}
	return items, nil
}

func (s *WorkspaceService) SetFavorite(ctx context.Context, actor, documentID string, favorite bool) error {
	if _, err := s.documentFor(ctx, actor, "view", documentID); err != nil {
		return err
	}
	existing, err := queryRows[database.DocumentFavorite](ctx, s.db, "SELECT * FROM document_favorites WHERE user_id = $actor AND document_id = $document LIMIT 1", map[string]any{"actor": actor, "document": documentID})
	if err != nil {
		return err
	}
	if favorite && len(existing) == 0 {
		_, err = surrealdb.Create[database.DocumentFavorite](ctx, s.db, "document_favorites", &database.DocumentFavorite{UserID: actor, DocumentID: documentID, CreatedAt: time.Now().UTC()})
		if err == nil {
			_ = writeAudit(ctx, s.db, actor, "document.favorited", "document", documentID, nil)
		}
		return err
	}
	if !favorite && len(existing) > 0 {
		_, err = surrealdb.Delete[database.DocumentFavorite](ctx, s.db, mustRecordID(string(existing[0].ID)))
		if err == nil {
			_ = writeAudit(ctx, s.db, actor, "document.unfavorited", "document", documentID, nil)
		}
	}
	return err
}

// Trash lists trashed documents of the organization the user could delete.
func (s *WorkspaceService) Trash(ctx context.Context, actor string) ([]database.Document, error) {
	scope, err := s.access.Scope(ctx, actor, orgOf(ctx))
	if err != nil {
		return nil, err
	}
	clause, vars := scope.Clause()
	candidates, err := queryRows[database.Document](ctx, s.db, "SELECT * FROM documents WHERE "+clause+" AND status = 'trashed' ORDER BY deleted_at DESC", vars)
	if err != nil {
		return nil, err
	}
	items := []database.Document{}
	for i := range candidates {
		if allowed, err := s.access.CanDocument(ctx, actor, "delete", &candidates[i]); err == nil && allowed {
			items = append(items, candidates[i])
		}
	}
	return items, nil
}
func (s *WorkspaceService) MoveToTrash(ctx context.Context, actor, id string) error {
	doc, err := s.documentFor(ctx, actor, "delete", id)
	if err != nil {
		return err
	}
	_, err = surrealdb.Merge[database.Document](ctx, s.db, mustRecordID(id), map[string]any{"status": database.DocumentStatusTrashed, "deleted_from_status": doc.Status, "deleted_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "document.trashed", "document", id, nil)
	}
	return err
}
func (s *WorkspaceService) Restore(ctx context.Context, actor, id string) error {
	doc, err := s.documentFor(ctx, actor, "delete", id)
	if err != nil || doc.Status != database.DocumentStatusTrashed {
		return fmt.Errorf("trashed document not found")
	}
	status := doc.DeletedFromStatus
	if status == "" || status == database.DocumentStatusTrashed {
		status = database.DocumentStatusReady
	}
	_, err = surrealdb.Merge[database.Document](ctx, s.db, mustRecordID(id), map[string]any{"status": status, "deleted_from_status": "", "deleted_at": nil, "updated_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "document.restored", "document", id, nil)
	}
	return err
}

func (s *WorkspaceService) PermanentlyDelete(ctx context.Context, actor, id string) error {
	doc, err := s.documentFor(ctx, actor, "delete", id)
	if err != nil {
		return err
	}
	if doc.Status != database.DocumentStatusTrashed {
		return fmt.Errorf("only trashed documents can be permanently deleted")
	}
	holds, err := queryRows[database.LegalHold](ctx, s.db, "SELECT * FROM legal_holds WHERE active = true AND document_ids CONTAINS $document LIMIT 1", map[string]any{"document": id})
	if err != nil {
		return err
	}
	if len(holds) > 0 {
		return ErrLegalHold
	}
	versions, err := queryRows[database.DocumentVersion](ctx, s.db, "SELECT * FROM document_versions WHERE document_id = $document", map[string]any{"document": id})
	if err != nil {
		return err
	}
	paths := map[string]struct{}{doc.StoragePath: {}}
	for i := range versions {
		paths[versions[i].StoragePath] = struct{}{}
	}
	for path := range paths {
		if path != "" {
			if err := s.storage.Delete(ctx, path); err != nil {
				return fmt.Errorf("delete stored document: %w", err)
			}
		}
	}
	for _, statement := range []string{
		"DELETE document_version_contributors WHERE version_id IN (SELECT VALUE <string> id FROM document_versions WHERE document_id = $document)",
		"DELETE document_versions WHERE document_id = $document",
		"DELETE document_favorites WHERE document_id = $document",
		"DELETE document_categories WHERE document_id = $document",
		"DELETE shares WHERE document_id = $document",
		"DELETE document_grants WHERE document_id = $document",
		"DELETE access_requests WHERE document_id = $document",
		"DELETE documents WHERE id = $record",
	} {
		if _, err := surrealdb.Query[any](ctx, s.db, statement, map[string]any{"document": id, "record": mustRecordID(id)}); err != nil {
			return err
		}
	}
	_ = writeAudit(ctx, s.db, actor, "document.permanently_deleted", "document", id, nil)
	return nil
}

// Shared lists documents shared with the user, including from other organizations.
func (s *WorkspaceService) Shared(ctx context.Context, actor string) ([]database.Document, error) {
	user, err := s.user(ctx, actor)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.sharing.SharedWithMe(ctx, user)
}

func (s *WorkspaceService) Workflows(ctx context.Context, actor string) ([]database.WorkflowRequest, error) {
	owned, err := queryRows[database.WorkflowRequest](ctx, s.db, "SELECT * FROM workflow_requests WHERE created_by = $actor ORDER BY updated_at DESC", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	steps, err := queryRows[database.WorkflowStep](ctx, s.db, "SELECT * FROM workflow_steps WHERE assigned_user_id = $actor", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	memberships, err := queryRows[database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE user_id = $actor", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	for i := range memberships {
		departmentSteps, queryErr := queryRows[database.WorkflowStep](ctx, s.db, "SELECT * FROM workflow_steps WHERE assigned_department_id = $department", map[string]any{"department": memberships[i].DepartmentID})
		if queryErr != nil {
			return nil, queryErr
		}
		steps = append(steps, departmentSteps...)
	}
	seen := map[string]bool{}
	for i := range owned {
		seen[string(owned[i].ID)] = true
	}
	for i := range steps {
		if seen[steps[i].WorkflowRequestID] {
			continue
		}
		rid, parseErr := database.ParseRecordID(steps[i].WorkflowRequestID)
		if parseErr != nil {
			continue
		}
		workflow, selectErr := surrealdb.Select[database.WorkflowRequest](ctx, s.db, rid)
		if selectErr == nil && workflow != nil {
			owned = append(owned, *workflow)
			seen[steps[i].WorkflowRequestID] = true
		}
	}
	return owned, nil
}
func (s *WorkspaceService) Settings(ctx context.Context, actor string) (map[string]any, error) {
	rows, err := queryRows[database.UserSetting](ctx, s.db, "SELECT * FROM user_settings WHERE user_id = $actor LIMIT 1", map[string]any{"actor": actor})
	if err != nil || len(rows) == 0 {
		return map[string]any{}, err
	}
	return rows[0].Values, nil
}
func (s *WorkspaceService) SaveSettings(ctx context.Context, actor string, values map[string]any) (map[string]any, error) {
	rows, err := queryRows[database.UserSetting](ctx, s.db, "SELECT * FROM user_settings WHERE user_id = $actor LIMIT 1", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if len(rows) == 0 {
		_, err = surrealdb.Create[database.UserSetting](ctx, s.db, "user_settings", &database.UserSetting{UserID: actor, Values: values, UpdatedAt: now})
	} else {
		_, err = surrealdb.Merge[database.UserSetting](ctx, s.db, mustRecordID(string(rows[0].ID)), map[string]any{"values": values, "updated_at": now})
	}
	return values, err
}
func (s *WorkspaceService) StorageStatus(ctx context.Context) map[string]any {
	err := s.storage.HealthCheck(ctx)
	return map[string]any{"provider": s.storage.Name(), "configured": true, "healthy": err == nil, "error": errorString(err)}
}
func (s *WorkspaceService) PrivacyExport(ctx context.Context, actor string) (map[string]any, error) {
	user, err := s.user(ctx, actor)
	if err != nil {
		return nil, err
	}
	documents, err := queryRows[database.Document](ctx, s.db, "SELECT * FROM documents WHERE owner_id = $actor", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	submissions, err := queryRows[database.FormSubmission](ctx, s.db, "SELECT * FROM form_submissions WHERE submitted_by = $actor", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	return map[string]any{"profile": map[string]any{"id": user.ID, "email": user.Email, "name": user.Name, "status": user.Status}, "documents": documents, "submissions": submissions, "exported_at": time.Now().UTC()}, nil
}
func (s *WorkspaceService) RequestDeletion(ctx context.Context, actor string) (*database.PrivacyRequest, error) {
	item, err := surrealdb.Create[database.PrivacyRequest](ctx, s.db, "privacy_requests", &database.PrivacyRequest{UserID: actor, Kind: "deletion", Status: "pending", RequestedAt: time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "privacy.deletion_requested", "user", actor, nil)
	}
	return item, err
}
func (s *WorkspaceService) LegalHolds(ctx context.Context) ([]database.LegalHold, error) {
	return queryRows[database.LegalHold](ctx, s.db, "SELECT * FROM legal_holds WHERE org_id = $org ORDER BY created_at DESC", map[string]any{"org": orgOf(ctx)})
}
func (s *WorkspaceService) CreateLegalHold(ctx context.Context, actor, name, reason string, documentIDs []string) (*database.LegalHold, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(reason) == "" || len(documentIDs) == 0 {
		return nil, fmt.Errorf("name, reason and documents are required")
	}
	for _, id := range documentIDs {
		if _, err := s.documentFor(ctx, actor, "view", id); err != nil {
			return nil, fmt.Errorf("document %s not found", id)
		}
	}
	item, err := surrealdb.Create[database.LegalHold](ctx, s.db, "legal_holds", map[string]any{"org_id": orgOf(ctx), "name": strings.TrimSpace(name), "reason": strings.TrimSpace(reason), "document_ids": documentIDs, "created_by": actor, "active": true, "created_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "legal_hold.created", "legal_hold", string(item.ID), map[string]any{"documents": documentIDs})
	}
	return item, err
}

func (s *WorkspaceService) ReleaseLegalHold(ctx context.Context, actor, id string) (*database.LegalHold, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("legal hold not found")
	}
	hold, err := surrealdb.Select[database.LegalHold](ctx, s.db, rid)
	if err != nil || hold == nil {
		return nil, fmt.Errorf("legal hold not found")
	}
	if !hold.Active {
		return hold, nil
	}
	hold, err = surrealdb.Merge[database.LegalHold](ctx, s.db, rid, map[string]any{"active": false, "released_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "legal_hold.released", "legal_hold", id, map[string]any{"documents": hold.DocumentIDs})
	}
	return hold, err
}

func (s *WorkspaceService) documentFor(ctx context.Context, actor, capability, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("document not found")
	}
	document, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || document == nil || document.ID == "" {
		return nil, fmt.Errorf("document not found")
	}
	allowed, err := s.access.CanDocument(ctx, actor, capability, document)
	if err != nil || !allowed {
		return nil, fmt.Errorf("document not found")
	}
	return document, nil
}
func (s *WorkspaceService) user(ctx context.Context, id string) (*database.User, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	return surrealdb.Select[database.User](ctx, s.db, rid)
}
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func queryCount(ctx context.Context, db *surrealdb.DB, statement string, vars map[string]any) (int, error) {
	type countRow struct {
		Count int `json:"count"`
	}
	rows, err := queryRows[countRow](ctx, db, statement, vars)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	return rows[0].Count, nil
}
