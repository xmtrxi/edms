package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/storage"
)

var ErrVersionNotFound = errors.New("document version not found")

type VersionService struct {
	DB      *surrealdb.DB
	storage storage.Provider
	docs    *DocumentService
}

func NewVersionService(db *surrealdb.DB, provider storage.Provider, docs *DocumentService) *VersionService {
	return &VersionService{DB: db, storage: provider, docs: docs}
}

func VersionStoragePath(documentID string, version int, filename string) string {
	return filepath.ToSlash(filepath.Join("documents", documentID, "versions", fmt.Sprintf("v%d%s", version, filepath.Ext(filename))))
}

func (s *VersionService) CreateInitial(ctx context.Context, doc *database.Document, actorID, source string) (*database.DocumentVersion, error) {
	return createInitialVersion(ctx, s.DB, doc, actorID, source)
}

func createInitialVersion(ctx context.Context, db *surrealdb.DB, doc *database.Document, actorID, source string) (*database.DocumentVersion, error) {
	if doc == nil {
		return nil, ErrDocumentNotFound
	}
	created, err := surrealdb.Create[database.DocumentVersion](ctx, db, "document_versions", &database.DocumentVersion{
		DocumentID: string(doc.ID), Version: 1, StorageProvider: doc.StorageProvider, StoragePath: doc.StoragePath,
		FileName: doc.Name, MimeType: doc.MimeType, Checksum: doc.Checksum, Size: doc.Size, CreatedBy: actorID,
		Source: source, ChangeSummary: "Initial document version", ChangeManifest: map[string]any{"kind": source}, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if _, err := surrealdb.Create[database.DocumentVersionContributor](ctx, db, "document_version_contributors", &database.DocumentVersionContributor{
		VersionID: string(created.ID), UserID: actorID, Role: "author", CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	rid, err := database.ParseRecordID(string(doc.ID))
	if err != nil {
		return nil, err
	}
	if _, err := surrealdb.Merge[database.Document](ctx, db, rid, map[string]any{
		"current_version_id": string(created.ID), "version_count": 1, "updated_at": time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, db, actorID, "version.created", "document", string(doc.ID), map[string]any{
		"document_version_id": string(created.ID), "version": 1, "source": source, "summary": created.ChangeSummary,
	})
	return created, nil
}

func (s *VersionService) List(ctx context.Context, ownerID, documentID string) ([]dto.DocumentVersionResponse, error) {
	if _, err := s.docs.Get(ctx, ownerID, documentID); err != nil {
		return nil, err
	}
	results, err := surrealdb.Query[[]database.DocumentVersion](ctx, s.DB,
		"SELECT * FROM document_versions WHERE document_id = $document ORDER BY version DESC", map[string]any{"document": documentID})
	if err != nil {
		return nil, err
	}
	versions := []database.DocumentVersion{}
	if results != nil && len(*results) > 0 {
		versions = (*results)[0].Result
	}
	users, err := s.usersByID(ctx)
	if err != nil {
		return nil, err
	}
	response := make([]dto.DocumentVersionResponse, 0, len(versions))
	for i := range versions {
		item := dto.NewDocumentVersionResponse(&versions[i])
		if user := users[versions[i].CreatedBy]; user != nil {
			creator := dto.NewUserResponse(user)
			item.Creator = &creator
		}
		response = append(response, item)
	}
	return response, nil
}

func (s *VersionService) Download(ctx context.Context, ownerID, documentID, versionID string) (*database.DocumentVersion, io.ReadCloser, error) {
	if _, err := s.docs.Get(ctx, ownerID, documentID); err != nil {
		return nil, nil, err
	}
	version, err := s.get(ctx, documentID, versionID)
	if err != nil {
		return nil, nil, err
	}
	reader, err := s.storage.Download(ctx, version.StoragePath)
	return version, reader, err
}

func (s *VersionService) Restore(ctx context.Context, ownerID, documentID, versionID, summary string) (*database.DocumentVersion, error) {
	doc, err := s.docs.GetFor(ctx, ownerID, "edit", documentID)
	if err != nil {
		return nil, err
	}
	base, err := s.get(ctx, documentID, versionID)
	if err != nil {
		return nil, err
	}
	reader, err := s.storage.Download(ctx, base.StoragePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	next := doc.VersionCount + 1
	if next < 2 {
		next = 2
	}
	path := VersionStoragePath(documentID, next, base.FileName)
	if err := s.storage.Upload(ctx, path, reader); err != nil {
		return nil, err
	}
	if summary == "" {
		summary = fmt.Sprintf("Restored version %d", base.Version)
	}
	version, err := surrealdb.Create[database.DocumentVersion](ctx, s.DB, "document_versions", &database.DocumentVersion{
		DocumentID: documentID, ParentVersionID: string(base.ID), Version: next, StorageProvider: s.storage.Name(),
		StoragePath: path, FileName: base.FileName, MimeType: base.MimeType, Checksum: base.Checksum, Size: base.Size,
		CreatedBy: ownerID, Source: database.VersionSourceRestore, ChangeSummary: summary,
		ChangeManifest: map[string]any{"kind": "restore", "restored_version": base.Version}, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		_ = s.storage.Delete(ctx, path)
		return nil, err
	}
	if err := s.addContributor(ctx, string(version.ID), ownerID, "author"); err != nil {
		return nil, err
	}
	rid, _ := database.ParseRecordID(documentID)
	_, err = surrealdb.Merge[database.Document](ctx, s.DB, rid, map[string]any{
		"current_version_id": string(version.ID), "version_count": next, "storage_path": path,
		"mime_type": base.MimeType, "size": base.Size, "checksum": base.Checksum,
		"status": database.DocumentStatusUploaded, "updated_at": time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.DB, ownerID, "version.restored", "document", documentID, map[string]any{
		"document_version_id": string(version.ID), "restored_from_version_id": string(base.ID), "summary": summary,
	})
	doc.CurrentVersionID = string(version.ID)
	doc.StoragePath = path
	doc.Checksum = base.Checksum
	s.docs.stageOCR(ctx, doc, version)
	return version, nil
}

func (s *VersionService) Contributors(ctx context.Context, ownerID, documentID string) (*dto.DocumentContributorsResponse, error) {
	versions, err := s.List(ctx, ownerID, documentID)
	if err != nil {
		return nil, err
	}
	type aggregate struct {
		user        *dto.UserResponse
		count       int
		first, last time.Time
	}
	byUser := map[string]*aggregate{}
	for _, version := range versions {
		if version.Creator == nil || version.Source == database.VersionSourceUpload || version.Source == database.VersionSourceStorageSync {
			continue
		}
		a := byUser[version.CreatedBy]
		if a == nil {
			u := *version.Creator
			a = &aggregate{user: &u, first: version.CreatedAt}
			byUser[version.CreatedBy] = a
		}
		a.count++
		if a.first.IsZero() || version.CreatedAt.Before(a.first) {
			a.first = version.CreatedAt
		}
		if version.CreatedAt.After(a.last) {
			a.last = version.CreatedAt
		}
	}
	response := &dto.DocumentContributorsResponse{UniqueEditorCount: len(byUser), Contributors: []dto.DocumentContributorResponse{}}
	for _, a := range byUser {
		response.EditCount += a.count
		response.Contributors = append(response.Contributors, dto.DocumentContributorResponse{User: *a.user, VersionsCreated: a.count, FirstEditedAt: a.first, LastEditedAt: a.last})
	}
	return response, nil
}

func (s *VersionService) History(ctx context.Context, ownerID, documentID string) ([]dto.DocumentHistoryEvent, error) {
	if _, err := s.docs.Get(ctx, ownerID, documentID); err != nil {
		return nil, err
	}
	results, err := surrealdb.Query[[]database.AuditEvent](ctx, s.DB,
		"SELECT * FROM audit_events WHERE resource = 'document' AND resource_id = $document ORDER BY created_at DESC",
		map[string]any{"document": documentID})
	if err != nil {
		return nil, err
	}
	events := []dto.DocumentHistoryEvent{}
	if results == nil || len(*results) == 0 {
		return events, nil
	}
	for _, event := range (*results)[0].Result {
		versionID, summary := "", ""
		if value, ok := event.Metadata["document_version_id"].(string); ok {
			versionID = value
		}
		if value, ok := event.Metadata["summary"].(string); ok {
			summary = value
		}
		events = append(events, dto.DocumentHistoryEvent{ID: string(event.ID), Type: event.Action, ActorID: event.ActorID,
			Action: event.Action, Resource: event.Resource, ResourceID: event.ResourceID, DocumentVersionID: versionID,
			Summary: summary, Metadata: event.Metadata, CreatedAt: event.CreatedAt})
	}
	return events, nil
}

func (s *VersionService) get(ctx context.Context, documentID, versionID string) (*database.DocumentVersion, error) {
	rid, err := database.ParseRecordID(versionID)
	if err != nil {
		return nil, ErrVersionNotFound
	}
	version, err := surrealdb.Select[database.DocumentVersion](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	if version == nil || version.DocumentID != documentID {
		return nil, ErrVersionNotFound
	}
	return version, nil
}

func (s *VersionService) addContributor(ctx context.Context, versionID, userID, role string) error {
	_, err := surrealdb.Create[database.DocumentVersionContributor](ctx, s.DB, "document_version_contributors", &database.DocumentVersionContributor{
		VersionID: versionID, UserID: userID, Role: role, CreatedAt: time.Now().UTC(),
	})
	return err
}

func (s *VersionService) usersByID(ctx context.Context) (map[string]*database.User, error) {
	results, err := surrealdb.Query[[]database.User](ctx, s.DB, "SELECT * FROM users", nil)
	if err != nil {
		return nil, err
	}
	users := map[string]*database.User{}
	if results != nil && len(*results) > 0 {
		for i := range (*results)[0].Result {
			user := (*results)[0].Result[i]
			users[string(user.ID)] = &user
		}
	}
	return users, nil
}
