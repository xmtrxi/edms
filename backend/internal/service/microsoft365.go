package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/storage"
)

var (
	ErrM365Unavailable  = errors.New("opening in Microsoft 365 needs SharePoint storage")
	ErrM365Format       = errors.New("Microsoft 365 for the web can edit Word, Excel and PowerPoint files")
	ErrM365NoSession    = errors.New("this document isn't open in Microsoft 365")
	m365EditableFormats = map[string]bool{"docx": true, "docm": true, "xlsx": true, "xlsm": true, "pptx": true, "pptm": true}
)

// Microsoft365Service opens documents in Office for the web through the
// SharePoint drive that stores them. Each person edits a working copy, which
// is checked in as a new version.
type Microsoft365Service struct {
	db       *surrealdb.DB
	storage  storage.Provider
	docs     *DocumentService
	versions *VersionService
	log      *slog.Logger
}

func NewMicrosoft365Service(db *surrealdb.DB, provider storage.Provider, docs *DocumentService, versions *VersionService, logger *slog.Logger) *Microsoft365Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Microsoft365Service{db: db, storage: provider, docs: docs, versions: versions, log: logger}
}

// Enabled reports whether files live in SharePoint.
func (s *Microsoft365Service) Enabled() bool {
	return s.storage != nil && s.storage.Name() == "sharepoint"
}

// CanEditInMicrosoft365 reports whether Office for the web edits this file type.
func CanEditInMicrosoft365(name string) bool {
	return m365EditableFormats[strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")]
}

// Session returns the person's open working copy of the document, if any.
func (s *Microsoft365Service) Session(ctx context.Context, userID, documentID string) (*database.M365Session, error) {
	rows, err := queryRows[database.M365Session](ctx, s.db,
		"SELECT * FROM m365_sessions WHERE document_id = $document AND user_id = $user AND closed_at = NONE LIMIT 1",
		map[string]any{"document": documentID, "user": userID})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// Open returns a link that edits the person's working copy, creating the
// copy from the current version the first time.
func (s *Microsoft365Service) Open(ctx context.Context, userID, documentID string) (*database.M365Session, error) {
	if !s.Enabled() {
		return nil, ErrM365Unavailable
	}
	doc, err := s.docs.GetFor(ctx, userID, "create_version", documentID)
	if err != nil {
		return nil, err
	}
	if !CanEditInMicrosoft365(doc.Name) {
		return nil, ErrM365Format
	}
	if session, err := s.Session(ctx, userID, documentID); err != nil || session != nil {
		return session, err
	}
	version, err := surrealdb.Select[database.DocumentVersion](ctx, s.db, mustRecordID(doc.CurrentVersionID))
	if err != nil || version == nil {
		return nil, ErrVersionNotFound
	}
	reader, err := s.storage.Download(ctx, version.StoragePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	_, docKey, _ := strings.Cut(documentID, ":")
	_, userKey, _ := strings.Cut(userID, ":")
	workingPath := path.Join("m365-working", docKey, userKey, path.Base(doc.Name))
	if err := s.storage.Upload(ctx, workingPath, reader); err != nil {
		return nil, fmt.Errorf("create working copy: %w", err)
	}
	webURL, err := s.storage.GetUrl(ctx, workingPath)
	if err != nil {
		_ = s.storage.Delete(ctx, workingPath)
		return nil, err
	}
	now := time.Now().UTC()
	session, err := surrealdb.Create[database.M365Session](ctx, s.db, "m365_sessions", map[string]any{
		"org_id": doc.OrgID, "document_id": documentID, "user_id": userID, "path": workingPath,
		"web_url": editURL(webURL), "base_version_id": doc.CurrentVersionID, "checksum": version.Checksum,
		"created_at": now, "updated_at": now,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, workingPath)
		return nil, err
	}
	_ = writeAudit(ctx, s.db, userID, "document.m365_opened", "document", documentID, nil)
	return session, nil
}

// CheckIn saves the working copy as a new version when it changed. With
// done, the working copy is removed afterwards.
func (s *Microsoft365Service) CheckIn(ctx context.Context, userID, documentID string, done bool) (*database.DocumentVersion, error) {
	session, err := s.Session(ctx, userID, documentID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrM365NoSession
	}
	return s.checkIn(ctx, session, done)
}

func (s *Microsoft365Service) checkIn(ctx context.Context, session *database.M365Session, done bool) (*database.DocumentVersion, error) {
	reader, err := s.storage.Download(ctx, session.Path)
	if err != nil {
		return nil, fmt.Errorf("read working copy: %w", err)
	}
	defer reader.Close()
	temp, err := os.CreateTemp("", "edms-m365-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hash), reader); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	var version *database.DocumentVersion
	if checksum != session.Checksum {
		doc, err := surrealdb.Select[database.Document](ctx, s.db, mustRecordID(session.DocumentID))
		if err != nil || doc == nil {
			return nil, ErrDocumentNotFound
		}
		summary := "Edited in Microsoft 365"
		if doc.CurrentVersionID != session.BaseVersionID {
			summary += " (started from an earlier version)"
		}
		version, _, err = s.versions.AddVersion(ctx, session.UserID, session.DocumentID, NewVersionInput{
			File: temp, FileName: doc.Name, MimeType: doc.MimeType, Source: database.VersionSourceMicrosoft365, Summary: summary,
		})
		if err != nil {
			return nil, err
		}
	}
	// updated_at moves only when something changed, so idle copies age out.
	updates := map[string]any{"checksum": checksum}
	if version != nil {
		updates["base_version_id"] = string(version.ID)
		updates["updated_at"] = time.Now().UTC()
	}
	if done {
		updates["closed_at"] = time.Now().UTC()
		if err := s.storage.Delete(ctx, session.Path); err != nil {
			s.log.Warn("remove Microsoft 365 working copy", "path", session.Path, "error", err)
		}
	}
	if _, err := surrealdb.Merge[database.M365Session](ctx, s.db, mustRecordID(string(session.ID)), updates); err != nil {
		return version, err
	}
	return version, nil
}

// Discard closes the working copy without saving it.
func (s *Microsoft365Service) Discard(ctx context.Context, userID, documentID string) error {
	session, err := s.Session(ctx, userID, documentID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrM365NoSession
	}
	if err := s.storage.Delete(ctx, session.Path); err != nil {
		s.log.Warn("remove Microsoft 365 working copy", "path", session.Path, "error", err)
	}
	_, err = surrealdb.Merge[database.M365Session](ctx, s.db, mustRecordID(string(session.ID)), map[string]any{"closed_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
	return err
}

// SyncOpen checks in every open working copy, so edits are kept even when
// people forget to press "Done". Copies untouched for a week are closed.
func (s *Microsoft365Service) SyncOpen(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	sessions, err := queryRows[database.M365Session](ctx, s.db, "SELECT * FROM m365_sessions WHERE closed_at = NONE LIMIT 500", nil)
	if err != nil {
		return err
	}
	for i := range sessions {
		session := &sessions[i]
		done := time.Since(session.UpdatedAt) > 7*24*time.Hour
		actorCtx := WithActor(ctx, Actor{UserID: session.UserID, OrgID: session.OrgID})
		if _, err := s.checkIn(actorCtx, session, done); err != nil {
			s.log.Warn("sync Microsoft 365 edits", "document", session.DocumentID, "error", err)
		}
	}
	return nil
}

// editURL asks Office for the web to open in editing mode.
func editURL(webURL string) string {
	u, err := url.Parse(webURL)
	if err != nil {
		return webURL
	}
	q := u.Query()
	q.Set("action", "edit")
	u.RawQuery = q.Encode()
	return u.String()
}
