package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/storage"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrForbidden        = errors.New("forbidden")
)

type DocumentService struct {
	cfg     config.Config
	DB      *surrealdb.DB
	storage storage.Provider
	queue   *queue.Client
	search  *search.Client
	access  *AccessService
	log     *slog.Logger
	mu      sync.Mutex
}

func NewDocumentService(cfg config.Config, db *surrealdb.DB, provider storage.Provider, q *queue.Client, s *search.Client, logger *slog.Logger) *DocumentService {
	return &DocumentService{
		cfg:     cfg,
		DB:      db,
		storage: provider,
		queue:   q,
		search:  s,
		access:  NewAccessService(db),
		log:     logger,
	}
}

func newDocumentID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("doc_%d", time.Now().UnixNano())
	}
	return "doc_" + hex.EncodeToString(bytes)
}

func DocumentStoragePath(docID, filename string) string {
	return filepath.ToSlash(filepath.Join("documents", docID, "original"+filepath.Ext(filename)))
}

type UploadInput struct {
	OwnerID           string
	FolderID          string
	Name              string
	MimeType          string
	FolderPath        string
	FolderColor       string
	FolderCategoryIDs []string
	File              io.Reader
	Metadata          map[string]any
}

// Upload hashes to a temporary file first. This makes duplicate detection
// deterministic without buffering large documents in memory.
func (s *DocumentService) Upload(ctx context.Context, in UploadInput) (*database.Document, bool, error) {
	temp, err := os.CreateTemp("", "edms-upload-*")
	if err != nil {
		return nil, false, fmt.Errorf("create upload buffer: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), in.File)
	if err != nil {
		return nil, false, fmt.Errorf("read upload: %w", err)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Serialize the checksum backfill, lookup, and row creation. This closes the
	// race where two concurrent requests upload the same bytes before either row
	// exists. Backfilling also protects documents created before checksums were
	// introduced.
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backfillMissingChecksums(ctx, in.OwnerID); err != nil {
		return nil, false, fmt.Errorf("backfill document checksums: %w", err)
	}

	existing, err := s.findByChecksum(ctx, in.OwnerID, checksum)
	if err != nil {
		return nil, false, fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		// The bytes already exist, so never create another row or send them
		// through OCR again. Re-OCR is an explicit action on the existing document.
		return existing, true, nil
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, false, fmt.Errorf("rewind upload: %w", err)
	}

	docID := newDocumentID()
	now := time.Now().UTC()
	storagePath := DocumentStoragePath(docID, in.Name)

	doc := &database.Document{
		ID:              database.RecordID(docID),
		OwnerID:         in.OwnerID,
		FolderID:        in.FolderID,
		Name:            in.Name,
		OriginalName:    in.Name,
		MimeType:        in.MimeType,
		Size:            size,
		StorageProvider: s.cfg.StorageProvider,
		StoragePath:     storagePath,
		Checksum:        checksum,
		Status:          database.DocumentStatusUploaded,
		Metadata:        generatedMetadata(in.Name, storagePath, checksum, folderMetadata(in.Metadata, in.FolderPath, in.FolderColor, in.FolderCategoryIDs)),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.storage.Upload(ctx, storagePath, temp); err != nil {
		return nil, false, fmt.Errorf("store file: %w", err)
	}

	created, err := surrealdb.Create[database.Document](ctx, s.DB, "documents", doc)
	if err != nil {
		_ = s.storage.Delete(ctx, storagePath)
		return nil, false, fmt.Errorf("create document record: %w", err)
	}
	version, err := createInitialVersion(ctx, s.DB, created, in.OwnerID, database.VersionSourceUpload)
	if err != nil {
		_ = s.storage.Delete(ctx, storagePath)
		return nil, false, fmt.Errorf("create initial document version: %w", err)
	}

	s.stageOCR(ctx, created, version)

	return created, false, nil
}

// Folders derives a SharePoint-like folder tree from document metadata. It
// includes ancestor folders so a deep storage path can be browsed naturally.
func (s *DocumentService) Folders(ctx context.Context, ownerID string) ([]dto.FolderResponse, error) {
	results, err := surrealdb.Query[[]database.Document](ctx, s.DB,
		"SELECT * FROM documents WHERE owner_id = $owner AND status != 'archived' ORDER BY updated_at DESC",
		map[string]any{"owner": ownerID})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []dto.FolderResponse{}, nil
	}

	type folderStats struct {
		folder dto.FolderResponse
		latest time.Time
	}
	stats := map[string]*folderStats{}
	for _, doc := range (*results)[0].Result {
		path := cleanFolderPath(metadataString(doc.Metadata, "folder_path", ""))
		if path == "" {
			continue
		}
		parts := strings.Split(path, "/")
		for i := range parts {
			current := strings.Join(parts[:i+1], "/")
			entry := stats[current]
			if entry == nil {
				parent := strings.Join(parts[:i], "/")
				entry = &folderStats{folder: dto.FolderResponse{Path: current, Name: parts[i], ParentPath: parent}}
				stats[current] = entry
			}
			entry.folder.Count++
			entry.folder.Size += doc.Size
			if doc.UpdatedAt.After(entry.latest) {
				entry.latest = doc.UpdatedAt
				entry.folder.UpdatedAt = doc.UpdatedAt
			}
			if current == path {
				if color := metadataString(doc.Metadata, "folder_color", ""); color != "" {
					entry.folder.Color = color
				}
				entry.folder.CategoryIDs = metadataStringSlice(doc.Metadata, "folder_category_ids")
			}
		}
	}

	folders := make([]dto.FolderResponse, 0, len(stats))
	for _, entry := range stats {
		folders = append(folders, entry.folder)
	}
	slices.SortFunc(folders, func(a, b dto.FolderResponse) int { return strings.Compare(a.Path, b.Path) })
	return folders, nil
}

func (s *DocumentService) stageOCR(ctx context.Context, doc *database.Document, version *database.DocumentVersion) {
	if version == nil {
		s.log.Error("stage ocr without a document version", "document_id", doc.ID)
		return
	}
	job := &database.ProcessingJob{
		DocumentID: string(doc.ID),
		Type:       "docling_ocr",
		Status:     database.ProcessingJobStatusQueued,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	createdJob, err := surrealdb.Create[database.ProcessingJob](ctx, s.DB, "processing_jobs", job)
	if err != nil {
		s.log.Error("create processing job", "error", err, "document_id", doc.ID)
	}

	if s.queue == nil {
		s.log.Warn("ocr queue is not configured", "document_id", doc.ID)
		return
	}
	if err := s.queue.Enqueue(ctx, queue.TaskOCR, queue.OCRPayload{
		DocumentID: string(doc.ID), StoragePath: version.StoragePath,
		VersionID: string(version.ID), Checksum: version.Checksum,
		GenerationID: newDocumentID(), JobID: processingJobID(createdJob),
	}); err != nil {
		s.log.Error("enqueue ocr task", "error", err, "document_id", doc.ID)
	}
}

// ReOCR clears the current extraction and re-enqueues OCR processing.
func (s *DocumentService) ReOCR(ctx context.Context, ownerID, id string) (*database.Document, error) {
	doc, err := s.GetFor(ctx, ownerID, "edit", id)
	if err != nil {
		return nil, err
	}

	if err := deleteDocumentExtractions(ctx, s.DB, id); err != nil {
		s.log.Warn("clear old extraction", "error", err, "document_id", id)
	}

	if rid, rerr := database.ParseRecordID(id); rerr == nil {
		_, _ = surrealdb.Merge[database.Document](ctx, s.DB, rid, map[string]any{
			"status":     database.DocumentStatusUploaded,
			"updated_at": time.Now().UTC(),
		})
	}

	versionRID, err := database.ParseRecordID(doc.CurrentVersionID)
	if err != nil {
		return nil, err
	}
	version, err := surrealdb.Select[database.DocumentVersion](ctx, s.DB, versionRID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, ErrVersionNotFound
	}
	s.stageOCR(ctx, doc, version)
	return doc, nil
}

func processingJobID(job *database.ProcessingJob) string {
	if job == nil {
		return ""
	}
	return string(job.ID)
}

// LatestProcessingJob returns the most recent OCR job after confirming the
// caller can read the document. Job data is intentionally exposed through the
// EDMS API instead of letting browsers access Docling credentials directly.
func (s *DocumentService) LatestProcessingJob(ctx context.Context, ownerID, id string) (*database.ProcessingJob, error) {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return nil, err
	}
	job, err := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("document_id", id).OrderBy("created_at DESC").First(ctx)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func deleteDocumentExtractions(ctx context.Context, client *surrealdb.DB, documentID string) error {
	_, err := surrealdb.Query[any](ctx, client, "DELETE FROM document_extractions WHERE document_id = $document_id", map[string]any{"document_id": documentID})
	return err
}

func (s *DocumentService) Get(ctx context.Context, ownerID, id string) (*database.Document, error) {
	return s.GetFor(ctx, ownerID, "view", id)
}

func (s *DocumentService) GetFor(ctx context.Context, userID, capability, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrDocumentNotFound
	}
	allowed, err := s.access.CanDocument(ctx, userID, capability, doc)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return doc, nil
}

func (s *DocumentService) GetOCR(ctx context.Context, ownerID, id string) (*database.Document, *database.DocumentExtraction, error) {
	doc, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return nil, nil, err
	}

	extraction, err := surrealgoorm.Query[database.DocumentExtraction](s.DB, "document_extractions").WhereEq("document_id", id).First(ctx)
	if err != nil {
		return nil, nil, err
	}
	return doc, extraction, nil
}

func (s *DocumentService) GetFullOCR(ctx context.Context, ownerID, id string) (*database.Document, *database.DocumentExtraction, error) {
	doc, extraction, err := s.GetOCR(ctx, ownerID, id)
	if err != nil || extraction == nil {
		return doc, extraction, err
	}
	if extraction.Metadata == nil {
		extraction.Metadata = map[string]any{}
	}
	if path := metadataString(extraction.Metadata, "full_text_path", ""); path != "" {
		text, err := s.readOCRArtifact(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		extraction.Text = text
	}
	if path := metadataString(extraction.Metadata, "markdown_path", ""); path != "" {
		markdown, err := s.readOCRArtifact(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		extraction.Metadata["markdown_preview"] = markdown
	}
	extraction.Metadata["full_content"] = true
	return doc, extraction, nil
}

func (s *DocumentService) readOCRArtifact(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	reader, err := s.storage.Download(ctx, path)
	if err != nil {
		return "", fmt.Errorf("download OCR artifact: %w", err)
	}
	defer reader.Close()
	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read OCR artifact: %w", err)
	}
	return string(contents), nil
}

func (s *DocumentService) Download(ctx context.Context, ownerID, id string) (*database.Document, io.ReadCloser, error) {
	doc, err := s.GetFor(ctx, ownerID, "download", id)
	if err != nil {
		return nil, nil, err
	}
	reader, err := s.storage.Download(ctx, doc.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("download file: %w", err)
	}
	return doc, reader, nil
}

func (s *DocumentService) Delete(ctx context.Context, ownerID, id string) error {
	_, err := s.GetFor(ctx, ownerID, "delete", id)
	if err != nil {
		return err
	}
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return err
	}
	if _, err := surrealdb.Merge[database.Document](ctx, s.DB, rid, map[string]any{
		"status":     database.DocumentStatusArchived,
		"updated_at": time.Now().UTC(),
	}); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.DB, ownerID, "document.archived", "document", id, nil)
	return nil
}
