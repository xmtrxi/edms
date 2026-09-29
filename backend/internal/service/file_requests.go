package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

var (
	ErrFileRequestClosed = errors.New("this file request is closed")
	ErrFileRequestFull   = errors.New("this file request has received all the files it accepts")
)

// FileRequestService lets people without an account drop files into a
// folder through a link. Files belong to whoever created the request.
type FileRequestService struct {
	db      *surrealdb.DB
	access  *AccessService
	uploads *UploadService
	appURL  string
}

func NewFileRequestService(db *surrealdb.DB, uploads *UploadService, appURL string) *FileRequestService {
	return &FileRequestService{db: db, access: NewAccessService(db), uploads: uploads, appURL: strings.TrimRight(appURL, "/")}
}

type FileRequestInput struct {
	FolderID     string    `json:"folder_id"`
	Title        string    `json:"title"`
	Instructions string    `json:"instructions"`
	MaxFiles     int       `json:"max_files"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Create needs upload access to the folder, since received files land there.
func (s *FileRequestService) Create(ctx context.Context, userID string, in FileRequestInput) (*database.FileRequest, string, error) {
	if strings.TrimSpace(in.Title) == "" || in.FolderID == "" {
		return nil, "", fmt.Errorf("a title and a folder are required")
	}
	if err := s.access.RequireFolder(ctx, userID, "upload", in.FolderID); err != nil {
		return nil, "", ErrForbidden
	}
	folder, err := s.access.folder(ctx, in.FolderID)
	if err != nil || folder == nil {
		return nil, "", ErrFolderNotFound
	}
	if in.MaxFiles <= 0 || in.MaxFiles > 500 {
		in.MaxFiles = 100
	}
	now := time.Now().UTC()
	if in.ExpiresAt.IsZero() || in.ExpiresAt.After(now.Add(365*24*time.Hour)) {
		in.ExpiresAt = now.Add(14 * 24 * time.Hour)
	}
	token, err := newShareToken()
	if err != nil {
		return nil, "", err
	}
	request, err := surrealdb.Create[database.FileRequest](ctx, s.db, "file_requests", map[string]any{
		"org_id": folder.OrgID, "folder_id": in.FolderID, "title": strings.TrimSpace(in.Title),
		"instructions": strings.TrimSpace(in.Instructions), "token_hash": secretHash(token), "created_by": userID,
		"max_files": in.MaxFiles, "received_files": 0, "expires_at": in.ExpiresAt, "created_at": now, "updated_at": now,
	})
	if err != nil {
		return nil, "", err
	}
	_ = writeAudit(ctx, s.db, userID, "file_request.created", "folder", in.FolderID, map[string]any{"title": request.Title})
	return request, s.appURL + "/r/" + token, nil
}

// List returns the requests for a folder the user can view.
func (s *FileRequestService) List(ctx context.Context, userID, folderID string) ([]database.FileRequest, error) {
	if err := s.access.RequireFolder(ctx, userID, "view", folderID); err != nil {
		return nil, ErrForbidden
	}
	return queryRows[database.FileRequest](ctx, s.db,
		"SELECT * FROM file_requests WHERE folder_id = $folder ORDER BY created_at DESC", map[string]any{"folder": folderID})
}

// Close stops a request from accepting files.
func (s *FileRequestService) Close(ctx context.Context, userID, id string) error {
	request, err := s.byID(ctx, id)
	if err != nil {
		return err
	}
	if request.CreatedBy != userID {
		if err := s.access.RequireFolder(ctx, userID, "manage_folder", request.FolderID); err != nil {
			return ErrForbidden
		}
	}
	rid, _ := database.ParseRecordID(id)
	_, err = surrealdb.Query[any](ctx, s.db, "UPDATE $request SET closed_at = time::now(), updated_at = time::now()", map[string]any{"request": rid})
	return err
}

// PublicFileRequest is what an uploader sees.
type PublicFileRequest struct {
	Title        string    `json:"title"`
	Instructions string    `json:"instructions,omitempty"`
	Organization string    `json:"organization"`
	RequestedBy  string    `json:"requested_by"`
	Remaining    int       `json:"remaining"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Open resolves a request link for an anonymous uploader.
func (s *FileRequestService) Open(ctx context.Context, token string) (*database.FileRequest, *PublicFileRequest, error) {
	requests, err := queryRows[database.FileRequest](ctx, s.db, "SELECT * FROM file_requests WHERE token_hash = $hash LIMIT 1", map[string]any{"hash": secretHash(token)})
	if err != nil {
		return nil, nil, err
	}
	if len(requests) == 0 {
		return nil, nil, ErrShareNotFound
	}
	request := requests[0]
	if !request.ClosedAt.IsZero() || time.Now().UTC().After(request.ExpiresAt) {
		return nil, nil, ErrFileRequestClosed
	}
	view := &PublicFileRequest{Title: request.Title, Instructions: request.Instructions, Remaining: request.MaxFiles - request.ReceivedFiles, ExpiresAt: request.ExpiresAt}
	if orgs, err := queryRows[database.Organization](ctx, s.db, "SELECT * FROM organizations WHERE <string> id = $org LIMIT 1", map[string]any{"org": request.OrgID}); err == nil && len(orgs) > 0 {
		view.Organization = orgs[0].Name
	}
	if user, err := (&OrganizationService{db: s.db}).userByID(ctx, request.CreatedBy); err == nil && user != nil {
		view.RequestedBy = firstNonEmpty(user.Name, user.Email)
	}
	if view.Remaining <= 0 {
		return &request, view, ErrFileRequestFull
	}
	return &request, view, nil
}

// uploaderID identifies anonymous uploads for one request, so only that
// link can add chunks to the sessions it started.
func uploaderID(request *database.FileRequest) string { return "file_request:" + string(request.ID) }

// StartUpload opens an upload session into the request's folder.
func (s *FileRequestService) StartUpload(ctx context.Context, token string, name, mime string, size int64) (*database.UploadSession, error) {
	request, _, err := s.Open(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.uploads.start(ctx, uploaderID(request), request.OrgID, string(request.ID), StartUploadInput{Name: name, MimeType: mime, Size: size, FolderID: request.FolderID})
}

func (s *FileRequestService) AppendChunk(ctx context.Context, token, sessionID string, offset int64, chunk io.Reader) (*database.UploadSession, error) {
	request, _, err := s.Open(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.uploads.Append(ctx, uploaderID(request), sessionID, offset, chunk)
}

// CompleteUpload files the upload under the request's creator and records
// who sent it.
func (s *FileRequestService) CompleteUpload(ctx context.Context, token, sessionID, senderName, senderEmail string) (*database.Document, error) {
	request, _, err := s.Open(ctx, token)
	if err != nil {
		return nil, err
	}
	doc, _, _, err := s.uploads.complete(ctx, uploaderID(request), sessionID, request.CreatedBy, map[string]any{
		"source": "file_request", "file_request_id": string(request.ID),
		"sender_name": strings.TrimSpace(senderName), "sender_email": strings.ToLower(strings.TrimSpace(senderEmail)),
	})
	if err != nil {
		return nil, err
	}
	rid, _ := database.ParseRecordID(string(request.ID))
	_, _ = surrealdb.Query[any](ctx, s.db, "UPDATE $request SET received_files += 1, updated_at = time::now()", map[string]any{"request": rid})
	_ = writeAudit(ctx, s.db, request.CreatedBy, "file_request.received", "document", string(doc.ID), map[string]any{"sender_email": senderEmail})
	return doc, nil
}

func (s *FileRequestService) byID(ctx context.Context, id string) (*database.FileRequest, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrShareNotFound
	}
	request, err := surrealdb.Select[database.FileRequest](ctx, s.db, rid)
	if err != nil || request == nil || request.ID == "" || request.OrgID != orgOf(ctx) {
		return nil, ErrShareNotFound
	}
	return request, nil
}
