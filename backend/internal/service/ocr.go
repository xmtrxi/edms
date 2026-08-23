package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"strconv"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"
	paperlessngxgo "github.com/xmtrxi/paperless-ngx.go"

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

	docID, err := database.ParseRecordID(documentID)
	if err != nil {
		return err
	}

	doc, err := surrealdb.Select[database.Document](ctx, s.DB, docID)
	if err != nil {
		return err
	}
	if doc == nil {
		return ErrDocumentNotFound
	}

	jobID := ""
	job, err := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("document_id", documentID).First(ctx)
	if err == nil && job != nil {
		jobID = string(job.ID)
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"status":     database.ProcessingJobStatusRunning,
				"updated_at": time.Now().UTC(),
			})
		}
	}

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, docID, map[string]any{
		"status":     database.DocumentStatusProcessing,
		"updated_at": time.Now().UTC(),
	})

	file, err := s.storage.Download(ctx, storagePath)
	if err != nil {
		s.failDocument(ctx, doc, jobID, fmt.Errorf("download original: %w", err))
		return err
	}
	defer file.Close()

	var processed *paperlessngxgo.Document
	taskID := ""
	if paperlessID := metadataInt(doc.Metadata, "paperless_document_id"); paperlessID > 0 {
		processed, err = s.paperless.GetDocument(ctx, paperlessID)
		if err == nil {
			s.log.Info("reusing linked paperless document", "document_id", doc.ID, "paperless_document_id", paperlessID)
		}
	}
	if processed == nil {
		processed, taskID, err = s.submitAndWait(ctx, doc, file, jobID)
	}
	if err != nil {
		s.failDocument(ctx, doc, jobID, err)
		return err
	}
	if processed == nil {
		err = fmt.Errorf("paperless returned no processed document")
		s.failDocument(ctx, doc, jobID, err)
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

func (s *OCRService) submitAndWait(ctx context.Context, doc *database.Document, file io.Reader, jobID string) (*paperlessngxgo.Document, string, error) {
	taskID, err := s.paperless.UploadDocument(ctx, file, doc.OriginalName, doc.Name)
	if err != nil {
		return nil, "", fmt.Errorf("submit to paperless: %w", err)
	}
	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"external_id": taskID,
				"updated_at":  time.Now().UTC(),
			})
		}
	}
	result, duplicateID, err := s.waitForTask(ctx, taskID)
	if err != nil {
		return nil, taskID, err
	}
	if duplicateID > 0 {
		processed, fetchErr := s.paperless.GetDocument(ctx, duplicateID)
		s.log.Info("reusing duplicate paperless document", "document_id", doc.ID, "paperless_document_id", duplicateID)
		return processed, taskID, fetchErr
	}
	processed, err := s.findProcessedDocument(ctx, doc, result)
	return processed, taskID, err
}

func metadataInt(metadata map[string]any, key string) int {
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		result, _ := value.Int64()
		return int(result)
	case string:
		result, _ := strconv.Atoi(value)
		return result
	}
	return 0
}

func (s *OCRService) waitForTask(ctx context.Context, taskID string) (result *string, duplicateID int, err error) {
	for attempt := 0; attempt < 60; attempt++ {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		task, err := s.paperless.GetTask(ctx, taskID)
		if err != nil {
			continue
		}
		if task.Status == paperlessngxgo.StatusSuccess {
			return task.Result, 0, nil
		}
		if task.Status == paperlessngxgo.StatusFailure {
			result := ""
			if task.Result != nil {
				result = *task.Result
			}
			if id, ok := paperless.DuplicateDocumentID(result); ok {
				return task.Result, id, nil
			}
			return nil, 0, fmt.Errorf("paperless processing failed: %s", result)
		}
	}
	return nil, 0, fmt.Errorf("paperless processing timed out")
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
	docRID, _ := database.ParseRecordID(string(doc.ID))

	if err := deleteDocumentExtractions(ctx, s.DB, string(doc.ID)); err != nil {
		s.log.Warn("clear old extraction", "error", err, "document_id", doc.ID)
	}

	if _, err := surrealdb.Create[database.DocumentExtraction](ctx, s.DB, "document_extractions", &database.DocumentExtraction{
		DocumentID: string(doc.ID),
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

	metadata := maps.Clone(doc.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["extracted"] = true
	metadata["paperless_document_id"] = processed.ID
	metadata["paperless_title"] = processed.Title

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, docRID, map[string]any{
		"status":     database.DocumentStatusReady,
		"metadata":   metadata,
		"updated_at": now,
	})

	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"status":       database.ProcessingJobStatusCompleted,
				"result":       map[string]any{"task_id": taskID, "paperless_document_id": processed.ID, "archive_path": archivePath},
				"completed_at": now,
				"updated_at":   now,
			})
		}
	}

	if s.search != nil {
		if err := s.search.IndexDocument(ctx, search.Document{
			RecordID:     string(doc.ID),
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
	docRID, _ := database.ParseRecordID(string(doc.ID))
	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, docRID, map[string]any{
		"status":     database.DocumentStatusFailed,
		"updated_at": now,
	})
	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"status":     database.ProcessingJobStatusFailed,
				"last_error": cause.Error(),
				"updated_at": now,
			})
		}
	}
	if s.search != nil {
		if err := s.search.DeleteDocument(ctx, string(doc.ID)); err != nil {
			s.log.Warn("remove document from index", "error", err, "document_id", doc.ID)
		}
	}
	s.log.Error("document ocr failed", "document_id", doc.ID, "error", cause)
}

func archivePath(originalPath string) string {
	dir, file := filepath.Split(originalPath)
	return dir + "archive" + filepath.Ext(file)
}
