package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	paperlessngxgo "github.com/xmtrxi/paperless-ngx.go"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/paperless"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/storage"
)

type OCRService struct {
	DB        *surrealdb.DB
	storage   storage.Provider
	paperless *paperless.Client
	search    *search.Client
	log       *slog.Logger
}

func NewOCRService(db *surrealdb.DB, provider storage.Provider, p *paperless.Client, s *search.Client, logger *slog.Logger) *OCRService {
	return &OCRService{DB: db, storage: provider, paperless: p, search: s, log: logger}
}

func (s *OCRService) ProcessOCR(ctx context.Context, documentID, storagePath string) error {
	if s.paperless == nil {
		return fmt.Errorf("paperless client is not configured")
	}

	doc, err := surrealdb.Select[database.Document](ctx, s.DB, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return ErrDocumentNotFound
	}

	jobID := ""
	job, err := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("document_id", documentID).First(ctx)
	if err == nil && job != nil {
		jobID = job.ID
		_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, job.ID, map[string]any{
			"status":     database.ProcessingJobStatusRunning,
			"updated_at": time.Now().UTC(),
		})
	}

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, documentID, map[string]any{
		"status":     database.DocumentStatusProcessing,
		"updated_at": time.Now().UTC(),
	})

	file, err := s.storage.Download(ctx, storagePath)
	if err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("download original: %w", err))
		return err
	}
	defer file.Close()

	taskID, err := s.paperless.UploadDocument(ctx, file, doc.OriginalName, doc.Name)
	if err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("submit to paperless: %w", err))
		return err
	}

	if jobID != "" {
		_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobID, map[string]any{
			"external_id": taskID,
			"updated_at":  time.Now().UTC(),
		})
	}

	result, err := s.waitForTask(ctx, taskID)
	if err != nil {
		s.failDocument(ctx, doc, jobID, err)
		return err
	}

	processed, err := s.findProcessedDocument(ctx, doc, result)
	if err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("fetch processed document: %w", err))
		return err
	}

	archive, err := s.paperless.DownloadArchive(ctx, processed.ID)
	if err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("download archive: %w", err))
		return err
	}
	defer archive.Close()

	archivePath := archivePath(storagePath)
	if err := s.storage.Upload(ctx, archivePath, archive); err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("store archive: %w", err))
		return err
	}

	s.completeDocument(ctx, doc, jobID, taskID, processed, archivePath)
	return nil
}

func (s *OCRService) waitForTask(ctx context.Context, taskID string) (*string, error) {
	for attempt := 0; attempt < 60; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		task, err := s.paperless.GetTask(ctx, taskID)
		if err != nil {
			continue
		}
		if task.Status == paperlessngxgo.StatusSuccess {
			return task.Result, nil
		}
		if task.Status == paperlessngxgo.StatusFailure {
			result := ""
			if task.Result != nil {
				result = *task.Result
			}
			return nil, fmt.Errorf("paperless processing failed: %s", result)
		}
	}
	return nil, fmt.Errorf("paperless processing timed out")
}

func (s *OCRService) findProcessedDocument(ctx context.Context, doc *database.Document, result *string) (*paperlessngxgo.Document, error) {
	if id, ok := s.paperless.DocumentIDFromTaskResult(result); ok {
		if processed, err := s.paperless.GetDocument(ctx, id); err == nil && processed != nil {
			return processed, nil
		}
	}
	return s.paperless.GetDocumentByTitle(ctx, doc.Name)
}

func (s *OCRService) completeDocument(ctx context.Context, doc *database.Document, jobID, taskID string, processed *paperlessngxgo.Document, archivePath string) {
	now := time.Now().UTC()

	if err := deleteDocumentExtractions(ctx, s.DB, doc.ID); err != nil {
		s.log.Warn("clear old extraction", "error", err, "document_id", doc.ID)
	}

	if _, err := surrealdb.Create[database.DocumentExtraction](ctx, s.DB, "document_extractions", &database.DocumentExtraction{
		DocumentID: doc.ID,
		Text:       processed.Content,
		Language:   "en",
		Metadata: map[string]any{
			"paperless_document_id": processed.ID,
			"title":                 processed.Title,
			"created":               processed.Created,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		s.log.Warn("create extraction", "error", err, "document_id", doc.ID)
	}

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, doc.ID, map[string]any{
		"status":     database.DocumentStatusReady,
		"metadata":   map[string]any{"extracted": true, "paperless_document_id": processed.ID},
		"updated_at": now,
	})

	if jobID != "" {
		_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobID, map[string]any{
			"status":       database.ProcessingJobStatusCompleted,
			"result":       map[string]any{"task_id": taskID, "paperless_document_id": processed.ID, "archive_path": archivePath},
			"completed_at": now,
			"updated_at":   now,
		})
	}

	if s.search != nil {
		if err := s.search.IndexDocument(ctx, search.Document{
			ID:           doc.ID,
			OwnerID:      doc.OwnerID,
			Name:         doc.Name,
			OriginalName: doc.OriginalName,
			MimeType:     doc.MimeType,
			Content:      processed.Content,
			Status:       database.DocumentStatusReady,
			CreatedAt:    doc.CreatedAt,
			UpdatedAt:    now,
		}); err != nil {
			s.log.Warn("index document", "error", err, "document_id", doc.ID)
		}
	}

	s.log.Info("document ocr complete", "document_id", doc.ID)
}

func (s *OCRService) failDocument(ctx context.Context, doc *database.Document, jobID string, cause error) {
	now := time.Now().UTC()
	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, doc.ID, map[string]any{
		"status":     database.DocumentStatusFailed,
		"updated_at": now,
	})
	if jobID != "" {
		_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobID, map[string]any{
			"status":     database.ProcessingJobStatusFailed,
			"last_error": cause.Error(),
			"updated_at": now,
		})
	}
	if s.search != nil {
		if err := s.search.DeleteDocument(ctx, doc.ID); err != nil {
			s.log.Warn("remove document from index", "error", err, "document_id", doc.ID)
		}
	}
	s.log.Error("document ocr failed", "document_id", doc.ID, "error", cause)
}

func archivePath(originalPath string) string {
	dir, file := filepath.Split(originalPath)
	return dir + "archive" + filepath.Ext(file)
}
