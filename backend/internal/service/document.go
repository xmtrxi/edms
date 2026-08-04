package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
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
	log     *slog.Logger
}

func NewDocumentService(cfg config.Config, db *surrealdb.DB, provider storage.Provider, q *queue.Client, s *search.Client, logger *slog.Logger) *DocumentService {
	return &DocumentService{
		cfg:     cfg,
		DB:      db,
		storage: provider,
		queue:   q,
		search:  s,
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
	OwnerID  string
	Name     string
	MimeType string
	File     io.Reader
}

func (s *DocumentService) Upload(ctx context.Context, in UploadInput) (*database.Document, error) {
	docID := newDocumentID()
	now := time.Now().UTC()
	storagePath := DocumentStoragePath(docID, in.Name)

	doc := &database.Document{
		ID:              database.RecordID(docID),
		OwnerID:         in.OwnerID,
		Name:            in.Name,
		OriginalName:    in.Name,
		MimeType:        in.MimeType,
		StorageProvider: s.cfg.StorageProvider,
		StoragePath:     storagePath,
		Status:          database.DocumentStatusUploaded,
		Metadata:        map[string]any{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.storage.Upload(ctx, storagePath, in.File); err != nil {
		return nil, fmt.Errorf("store file: %w", err)
	}

	created, err := surrealdb.Create[database.Document](ctx, s.DB, "documents", doc)
	if err != nil {
		_ = s.storage.Delete(ctx, storagePath)
		return nil, fmt.Errorf("create document record: %w", err)
	}

	s.stageOCR(ctx, created, storagePath)

	return created, nil
}

func (s *DocumentService) stageOCR(ctx context.Context, doc *database.Document, storagePath string) {
	job := &database.ProcessingJob{
		DocumentID: string(doc.ID),
		Type:       "paperless_ocr",
		Status:     database.ProcessingJobStatusQueued,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if _, err := surrealdb.Create[database.ProcessingJob](ctx, s.DB, "processing_jobs", job); err != nil {
		s.log.Error("create processing job", "error", err, "document_id", doc.ID)
	}

	if s.queue == nil {
		return
	}
	if err := s.queue.Enqueue(ctx, queue.TaskOCR, queue.OCRPayload{
		DocumentID:  string(doc.ID),
		StoragePath: storagePath,
	}); err != nil {
		s.log.Error("enqueue ocr task", "error", err, "document_id", doc.ID)
	}
}

// ReOCR clears the current extraction and re-enqueues OCR processing.
func (s *DocumentService) ReOCR(ctx context.Context, ownerID, id string) (*database.Document, error) {
	doc, err := s.Get(ctx, ownerID, id)
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

	s.stageOCR(ctx, doc, doc.StoragePath)
	return doc, nil
}

func deleteDocumentExtractions(ctx context.Context, client *surrealdb.DB, documentID string) error {
	_, err := surrealdb.Query[any](ctx, client, "DELETE FROM document_extractions WHERE document_id = $document_id", map[string]any{"document_id": documentID})
	return err
}

func (s *DocumentService) Get(ctx context.Context, ownerID, id string) (*database.Document, error) {
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
	if doc.OwnerID != ownerID {
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

func (s *DocumentService) Download(ctx context.Context, ownerID, id string) (*database.Document, io.ReadCloser, error) {
	doc, err := s.Get(ctx, ownerID, id)
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
	doc, err := s.Get(ctx, ownerID, id)
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
	if err := s.storage.Delete(ctx, doc.StoragePath); err != nil {
		s.log.Warn("delete file", "error", err, "path", doc.StoragePath)
	}
	if s.search != nil {
		if err := s.search.DeleteDocument(ctx, id); err != nil {
			s.log.Warn("remove document from index", "error", err, "document_id", id)
		}
	}
	return nil
}
