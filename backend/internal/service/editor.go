package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/editor"
	"github.com/xmtrxi/edms/internal/storage"
)

var (
	ErrEditorDisabled    = errors.New("browser editing is not set up on this installation")
	ErrEditorUnsupported = errors.New("this file type can't be opened in the editor")
)

const maxEditedFile = 512 << 20

// EditorService opens documents in ONLYOFFICE Docs and saves what people
// change there as new versions.
type EditorService struct {
	cfg      config.EditorConfig
	db       *surrealdb.DB
	storage  storage.Provider
	docs     *DocumentService
	versions *VersionService
	client   *http.Client
	log      *slog.Logger
}

func NewEditorService(cfg config.EditorConfig, db *surrealdb.DB, provider storage.Provider, docs *DocumentService, logger *slog.Logger) *EditorService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EditorService{
		cfg: cfg, db: db, storage: provider, docs: docs, versions: NewVersionService(db, provider, docs),
		client: &http.Client{Timeout: 2 * time.Minute}, log: logger,
	}
}

func (s *EditorService) Enabled() bool { return s.cfg.Enabled() }

// EditorSession is what the browser needs to start the editor.
type EditorSession struct {
	DocumentServerURL string         `json:"document_server_url"`
	Mode              string         `json:"mode"`
	Config            map[string]any `json:"config"`
}

// Open prepares an editing session. People who can't edit, and formats
// ONLYOFFICE can only display, open read-only.
func (s *EditorService) Open(ctx context.Context, userID, documentID string, viewOnly bool) (*EditorSession, error) {
	if !s.Enabled() {
		return nil, ErrEditorDisabled
	}
	doc, err := s.docs.GetFor(ctx, userID, "view", documentID)
	if err != nil {
		return nil, err
	}
	ext, format, ok := editor.FormatOf(doc.Name)
	if !ok {
		return nil, ErrEditorUnsupported
	}
	canEdit := false
	if format.Editable && !viewOnly && doc.Status != database.DocumentStatusArchived {
		edit, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
		version, _ := s.docs.access.CanDocument(ctx, userID, "create_version", doc)
		canEdit = edit && version && !s.onHold(ctx, doc)
	}
	canDownload, _ := s.docs.access.CanDocument(ctx, userID, "download", doc)
	canComment, _ := s.docs.access.CanDocument(ctx, userID, "comment", doc)

	fileToken, err := editor.SignScoped(s.cfg.JWTSecret, editor.Scoped{Purpose: "file", DocumentID: string(doc.ID), Path: doc.StoragePath, Name: doc.Name}, 12*time.Hour)
	if err != nil {
		return nil, err
	}
	callbackToken, err := editor.SignScoped(s.cfg.JWTSecret, editor.Scoped{Purpose: "callback", DocumentID: string(doc.ID), OrgID: doc.OrgID}, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	person := newPeople(s.db).get(ctx, userID)
	mode := "view"
	if canEdit {
		mode = "edit"
	}
	cfg := map[string]any{
		"type":         "desktop",
		"documentType": format.DocumentType,
		"document": map[string]any{
			"fileType": ext,
			"key":      editor.Key(string(doc.ID), doc.CurrentVersionID, doc.Checksum),
			"title":    doc.Name,
			"url":      s.cfg.APIURL + "/editor/files/" + fileToken,
			"permissions": map[string]any{
				"edit": canEdit, "download": canDownload, "print": canDownload, "copy": canDownload,
				"comment": canComment && canEdit, "review": canEdit, "fillForms": canEdit,
			},
		},
		"editorConfig": map[string]any{
			"mode":        mode,
			"lang":        "en",
			"callbackUrl": s.cfg.APIURL + "/editor/callback/" + callbackToken,
			"user":        map[string]any{"id": userID, "name": person.Name},
			"customization": map[string]any{
				"autosave": true, "forcesave": false, "compactHeader": true, "toolbarNoTabs": false,
				"uiTheme": "theme-classic-light", "goback": false,
			},
			// Everyone sees each other's changes as they type.
			"coEditing": map[string]any{"mode": "fast", "change": true},
		},
	}
	token, err := editor.Sign(s.cfg.JWTSecret, cfg)
	if err != nil {
		return nil, err
	}
	cfg["token"] = token
	return &EditorSession{DocumentServerURL: s.cfg.URL, Mode: mode, Config: cfg}, nil
}

func (s *EditorService) onHold(ctx context.Context, doc *database.Document) bool {
	rows, err := queryRows[map[string]any](ctx, s.db, "SELECT id FROM legal_holds WHERE active = true AND document_ids CONTAINS $doc LIMIT 1", map[string]any{"doc": string(doc.ID)})
	return err == nil && len(rows) > 0
}

// File streams a document to ONLYOFFICE by a link issued in Open.
func (s *EditorService) File(ctx context.Context, token string) (io.ReadCloser, string, error) {
	if !s.Enabled() {
		return nil, "", ErrEditorDisabled
	}
	scope, err := editor.VerifyScoped(s.cfg.JWTSecret, token, "file")
	if err != nil || scope.Path == "" {
		return nil, "", ErrForbidden
	}
	body, err := s.storage.Download(ctx, scope.Path)
	return body, scope.Name, err
}

// Callback handles ONLYOFFICE's status reports. When the last editor leaves,
// the edited file is downloaded and stored as a new version by the first
// person who changed it.
func (s *EditorService) Callback(ctx context.Context, pathToken string, body []byte, authorization string) error {
	if !s.Enabled() {
		return ErrEditorDisabled
	}
	scope, err := editor.VerifyScoped(s.cfg.JWTSecret, pathToken, "callback")
	if err != nil {
		return ErrForbidden
	}
	callback, err := editor.ParseCallback(s.cfg.JWTSecret, body, authorization)
	if err != nil {
		return ErrForbidden
	}
	switch callback.Status {
	case editor.StatusReadyToSave, editor.StatusForceSave:
	case editor.StatusSaveError, editor.StatusForceSaveError:
		s.log.Warn("editor could not save", "document_id", scope.DocumentID, "status", callback.Status)
		return nil
	default:
		return nil
	}

	doc, err := s.document(ctx, scope.DocumentID)
	if err != nil {
		return err
	}
	// The saved session must be for the current revision, or edits would
	// overwrite a version saved in the meantime.
	if callback.Key != editor.Key(string(doc.ID), doc.CurrentVersionID, doc.Checksum) {
		s.log.Warn("editor saved a stale session", "document_id", scope.DocumentID)
	}
	ext, _, _ := editor.FormatOf(doc.Name)
	if callback.FileType != "" && !strings.EqualFold(callback.FileType, ext) {
		return fmt.Errorf("editor returned a %s file for a %s document", callback.FileType, ext)
	}
	author := doc.OwnerID
	if len(callback.Users) > 0 && callback.Users[0] != "" {
		author = callback.Users[0]
	}
	ctx = WithActor(ctx, Actor{UserID: author, OrgID: firstNonEmpty(doc.OrgID, scope.OrgID)})
	if ok, _ := s.docs.access.CanDocument(ctx, author, "create_version", doc); !ok {
		return ErrForbidden
	}
	file, err := s.download(ctx, callback.URL)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(callback.Users))
	people := newPeople(s.db)
	for _, id := range callback.Users {
		names = append(names, people.get(ctx, id).Name)
	}
	summary := "Edited in the browser"
	if len(names) > 0 {
		summary += " by " + strings.Join(names, ", ")
	}
	_, _, err = s.versions.AddVersion(ctx, author, string(doc.ID), NewVersionInput{
		File: bytes.NewReader(file), FileName: doc.Name, Source: database.VersionSourceOnlyOfficeEdit, Summary: summary, SkipAccessCheck: true,
	})
	return err
}

// download fetches the edited file, only from the ONLYOFFICE server itself.
func (s *EditorService) download(ctx context.Context, raw string) ([]byte, error) {
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, errors.New("invalid file address from the editor")
	}
	allowed := false
	for _, base := range []string{s.cfg.URL, s.cfg.InternalURL} {
		if parsed, err := url.Parse(base); err == nil && base != "" && strings.EqualFold(parsed.Host, target.Host) {
			allowed = true
		}
	}
	if !allowed {
		return nil, errors.New("the edited file must come from the editor server")
	}
	// ONLYOFFICE reports its public address; fetch through the internal one.
	if s.cfg.InternalURL != "" && strings.HasPrefix(raw, s.cfg.URL) {
		raw = s.cfg.InternalURL + strings.TrimPrefix(raw, s.cfg.URL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download edited file: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download edited file: %s", res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxEditedFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEditedFile {
		return nil, errors.New("the edited file is too large")
	}
	return data, nil
}

func (s *EditorService) document(ctx context.Context, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrDocumentNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" {
		return nil, ErrDocumentNotFound
	}
	return doc, nil
}
