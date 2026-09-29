package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrUploadOffset   = errors.New("chunk does not start where the upload left off")
	ErrUploadTooLarge = errors.New("file is larger than this installation allows")
	ErrUploadPending  = errors.New("upload is not finished yet")
)

// maxChunkBytes caps one PUT so a single request cannot hold the server.
const maxChunkBytes = 32 << 20

// UploadService receives large files in chunks so an interrupted upload
// can resume. The finished file goes through DocumentService.Upload, so
// access, duplicates and OCR work exactly as for a normal upload.
type UploadService struct {
	db     *surrealdb.DB
	docs   *DocumentService
	access *AccessService
	dir    string
	max    int64

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewUploadService(cfg config.UploadConfig, db *surrealdb.DB, docs *DocumentService) *UploadService {
	return &UploadService{db: db, docs: docs, access: NewAccessService(db), dir: cfg.Dir, max: cfg.MaxBytes, locks: map[string]*sync.Mutex{}}
}

// StartUploadInput describes the file before its bytes arrive.
type StartUploadInput struct {
	Name       string        `json:"name"`
	MimeType   string        `json:"mime_type"`
	Size       int64         `json:"size"`
	FolderID   string        `json:"folder_id"`
	CategoryID string        `json:"category_id"`
	Access     *UploadAccess `json:"access"`
}

// Start opens a session. Folder permissions are checked now, so a user
// learns before sending gigabytes that they cannot upload there.
func (s *UploadService) Start(ctx context.Context, userID string, in StartUploadInput) (*database.UploadSession, error) {
	return s.start(ctx, userID, orgOf(ctx), "", in)
}

func (s *UploadService) start(ctx context.Context, userID, orgID, fileRequestID string, in StartUploadInput) (*database.UploadSession, error) {
	name := strings.TrimSpace(filepath.Base(in.Name))
	if name == "" || name == "." || name == "/" {
		return nil, fmt.Errorf("a valid file name is required")
	}
	if in.Size < 0 {
		return nil, fmt.Errorf("file size is required")
	}
	if s.max > 0 && in.Size > s.max {
		return nil, ErrUploadTooLarge
	}
	if in.FolderID != "" && fileRequestID == "" {
		if err := s.access.RequireFolder(ctx, userID, "upload", in.FolderID); err != nil {
			return nil, ErrForbidden
		}
	}
	mime := strings.TrimSpace(in.MimeType)
	if mime == "" {
		mime = mimeTypeForName(name)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, fmt.Errorf("prepare upload directory: %w", err)
	}
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": orgID, "user_id": userID, "name": name, "mime_type": mime, "size": in.Size,
		"received": 0, "status": database.UploadStatusOpen, "expires_at": now.Add(48 * time.Hour),
		"created_at": now, "updated_at": now,
	}
	if in.FolderID != "" {
		record["folder_id"] = in.FolderID
	}
	if in.CategoryID != "" {
		record["category_id"] = in.CategoryID
	}
	if fileRequestID != "" {
		record["file_request_id"] = fileRequestID
	}
	if in.Access != nil {
		encoded, err := json.Marshal(in.Access)
		if err != nil {
			return nil, err
		}
		record["access"] = string(encoded)
	}
	session, err := surrealdb.Create[database.UploadSession](ctx, s.db, "upload_sessions", record)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(s.stagingPath(session), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("stage upload: %w", err)
	}
	return session, file.Close()
}

// Status returns the session so a client can resume from Received.
func (s *UploadService) Status(ctx context.Context, userID, id string) (*database.UploadSession, error) {
	return s.session(ctx, userID, id)
}

// Append writes one chunk. offset must equal the bytes already received;
// otherwise the client resends from the returned position.
func (s *UploadService) Append(ctx context.Context, userID, id string, offset int64, chunk io.Reader) (*database.UploadSession, error) {
	lock := s.lockFor(id)
	lock.Lock()
	defer lock.Unlock()

	session, err := s.session(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if offset != session.Received {
		return session, ErrUploadOffset
	}
	remaining := session.Size - session.Received
	file, err := os.OpenFile(s.stagingPath(session), os.O_WRONLY, 0o600)
	if err != nil {
		return nil, ErrUploadNotFound
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	limit := remaining
	if limit > maxChunkBytes {
		limit = maxChunkBytes
	}
	// Read one byte past the limit to detect a client sending too much.
	written, err := io.Copy(file, io.LimitReader(chunk, limit+1))
	if err != nil {
		return nil, fmt.Errorf("write chunk: %w", err)
	}
	if written > limit {
		_ = file.Truncate(offset)
		return nil, fmt.Errorf("chunk is larger than the remaining file or %d bytes", maxChunkBytes)
	}
	received := offset + written
	rid, _ := database.ParseRecordID(id)
	return surrealdb.Merge[database.UploadSession](ctx, s.db, rid, map[string]any{"received": received, "updated_at": time.Now().UTC()})
}

// Complete turns the staged bytes into a document owned by the uploader.
func (s *UploadService) Complete(ctx context.Context, userID, id string) (*database.Document, bool, *database.UploadSession, error) {
	return s.complete(ctx, userID, id, "", nil)
}

// complete files the upload. owner overrides the session user (file
// requests file into their creator's name) and metadata is recorded on the
// document.
func (s *UploadService) complete(ctx context.Context, userID, id, owner string, metadata map[string]any) (*database.Document, bool, *database.UploadSession, error) {
	lock := s.lockFor(id)
	lock.Lock()
	defer lock.Unlock()

	session, err := s.session(ctx, userID, id)
	if err != nil {
		return nil, false, nil, err
	}
	if session.Received != session.Size {
		return nil, false, session, ErrUploadPending
	}
	var access UploadAccess
	if session.Access != "" {
		if err := json.Unmarshal([]byte(session.Access), &access); err != nil {
			return nil, false, session, fmt.Errorf("stored access settings are invalid")
		}
	}
	path := s.stagingPath(session)
	file, err := os.Open(path)
	if err != nil {
		return nil, false, session, ErrUploadNotFound
	}
	defer file.Close()
	if owner == "" {
		owner = session.UserID
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	doc, duplicate, err := s.docs.Upload(ctx, UploadInput{
		OrgID: session.OrgID, Access: access, OwnerID: owner, FolderID: session.FolderID,
		Name: session.Name, MimeType: session.MimeType, File: file, Metadata: metadata,
	})
	if err != nil {
		return nil, false, session, err
	}
	rid, _ := database.ParseRecordID(id)
	_, _ = surrealdb.Merge[database.UploadSession](ctx, s.db, rid, map[string]any{"status": database.UploadStatusCompleted, "updated_at": time.Now().UTC()})
	_ = os.Remove(path)
	s.forget(id)
	return doc, duplicate, session, nil
}

// Cancel discards an unfinished upload.
func (s *UploadService) Cancel(ctx context.Context, userID, id string) error {
	session, err := s.session(ctx, userID, id)
	if err != nil {
		return err
	}
	_ = os.Remove(s.stagingPath(session))
	rid, _ := database.ParseRecordID(id)
	_, err = surrealdb.Delete[database.UploadSession](ctx, s.db, rid)
	s.forget(id)
	return err
}

// CleanupExpired removes abandoned sessions and their staged bytes.
func (s *UploadService) CleanupExpired(ctx context.Context) (int, error) {
	sessions, err := queryRows[database.UploadSession](ctx, s.db,
		"SELECT * FROM upload_sessions WHERE status = $status AND expires_at < $now",
		map[string]any{"status": database.UploadStatusOpen, "now": time.Now().UTC()})
	if err != nil {
		return 0, err
	}
	for i := range sessions {
		_ = os.Remove(s.stagingPath(&sessions[i]))
		rid, _ := database.ParseRecordID(string(sessions[i].ID))
		_, _ = surrealdb.Delete[database.UploadSession](ctx, s.db, rid)
	}
	return len(sessions), nil
}

func (s *UploadService) session(ctx context.Context, userID, id string) (*database.UploadSession, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrUploadNotFound
	}
	session, err := surrealdb.Select[database.UploadSession](ctx, s.db, rid)
	if err != nil || session == nil || session.ID == "" || session.UserID != userID ||
		session.Status != database.UploadStatusOpen || time.Now().UTC().After(session.ExpiresAt) {
		return nil, ErrUploadNotFound
	}
	return session, nil
}

func (s *UploadService) stagingPath(session *database.UploadSession) string {
	_, key, _ := strings.Cut(string(session.ID), ":")
	return filepath.Join(s.dir, filepath.Base(key)+".part")
}

func (s *UploadService) lockFor(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.locks[id]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[id] = lock
	}
	return lock
}

func (s *UploadService) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locks, id)
}
