package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/docling"
	"github.com/xmtrxi/edms/internal/gotenberg"
	markdownutil "github.com/xmtrxi/edms/internal/markdown"
	"github.com/xmtrxi/edms/internal/search"
	"github.com/xmtrxi/edms/internal/storage"
)

const (
	maxExtractionPreviewBytes = 50_000
	maxMarkdownPreviewBytes   = 10_000
	maxIndexContentBytes      = 150_000
)

type OCRService struct {
	DB      *surrealdb.DB
	storage storage.Provider
	docling *docling.Client
	office  *gotenberg.Client
	search  *search.Client
	log     *slog.Logger
}

func NewOCRService(db *surrealdb.DB, provider storage.Provider, d *docling.Client, office *gotenberg.Client, s *search.Client, logger *slog.Logger) *OCRService {
	return &OCRService{DB: db, storage: provider, docling: d, office: office, search: s, log: logger}
}

func (s *OCRService) ProcessOCR(ctx context.Context, documentID, storagePath string) error {
	if s.docling == nil {
		return fmt.Errorf("docling client is not configured")
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
	job, err := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("document_id", documentID).OrderBy("created_at DESC").First(ctx)
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

	filename := doc.OriginalName
	input := io.Reader(file)
	if isLegacyOffice(filename) {
		if s.office == nil {
			err := fmt.Errorf("legacy office document requires gotenberg conversion")
			s.failDocument(ctx, doc, jobID, err)
			return err
		}
		converted, err := s.office.ConvertOfficeToPDF(ctx, filename, file)
		if err != nil {
			err = fmt.Errorf("legacy office convert: %w", err)
			s.failDocument(ctx, doc, jobID, err)
			return err
		}
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".pdf"
		input = bytes.NewReader(converted)
	}

	opts := docling.Options{
		ToFormats: []string{"json", "md", "text"},
		DoOCR:     true,
		TableMode: "accurate",
		// Keep Markdown portable and readable. Embedded images are data URLs and
		// make both the OCR preview and indexed text unusable for large documents.
		ImageExportMode: "placeholder",
	}
	taskID, err := s.docling.ConvertFileAsync(ctx, filename, input, opts)
	if err != nil {
		err = fmt.Errorf("submit to docling: %w", err)
		s.failDocument(ctx, doc, jobID, err)
		return err
	}
	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"external_id": taskID,
				"updated_at":  time.Now().UTC(),
			})
		}
	}

	res, err := s.waitForResult(ctx, taskID)
	if err != nil {
		s.failDocument(ctx, doc, jobID, err)
		return err
	}
	if err := s.completeDocument(ctx, doc, jobID, taskID, res); err != nil {
		s.failDocument(ctx, doc, jobID, err)
		return err
	}
	return nil
}

func (s *OCRService) waitForResult(ctx context.Context, taskID string) (*docling.ConvertResult, error) {
	delay := 2 * time.Second
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		task, err := s.docling.Poll(ctx, taskID)
		if err != nil {
			if delay < 5*time.Second {
				delay += time.Second
			}
			continue
		}
		switch task.Status {
		case "success", "succeeded", "complete", "completed", "done":
			return s.docling.Result(ctx, taskID)
		case "failure", "failed", "error":
			if task.Error != "" {
				return nil, fmt.Errorf("docling processing failed: %s", task.Error)
			}
			if task.Result != nil {
				return nil, fmt.Errorf("docling processing failed: %s", *task.Result)
			}
			return nil, fmt.Errorf("docling processing failed")
		}
		s.recordTaskProgress(ctx, taskID, task)
		if delay < 5*time.Second {
			delay += time.Second
		}
	}
	return nil, fmt.Errorf("docling processing timed out")
}

func (s *OCRService) recordTaskProgress(ctx context.Context, taskID string, task *docling.TaskStatus) {
	job, err := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("external_id", taskID).First(ctx)
	if err != nil || job == nil {
		return
	}
	jobID, err := database.ParseRecordID(string(job.ID))
	if err != nil {
		return
	}
	result := map[string]any{"task_status": task.Status}
	if task.Position != nil {
		result["task_position"] = *task.Position
	}
	if task.Meta != nil {
		result["task_meta"] = task.Meta
	}
	if task.Error != "" {
		result["task_error"] = task.Error
	}
	_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobID, map[string]any{
		"result": result, "updated_at": time.Now().UTC(),
	})
}

func (s *OCRService) completeDocument(ctx context.Context, doc *database.Document, jobID, taskID string, res *docling.ConvertResult) error {
	now := time.Now().UTC()
	docRID, _ := database.ParseRecordID(string(doc.ID))
	ocrJSONPath := ocrArtifactPath(doc, "ocr.json")
	textPath := ocrArtifactPath(doc, "extracted.txt")
	markdownPath := ocrArtifactPath(doc, "extracted.md")
	searchableText := markdownutil.WithoutEmbeddedDataImages(res.TextContent)
	searchableMarkdown := markdownutil.WithoutEmbeddedDataImages(res.MdContent)
	textPreview := capUTF8(searchableText, maxExtractionPreviewBytes)
	indexContent := capUTF8(searchableText, maxIndexContentBytes)
	markdownPreview := capUTF8(searchableMarkdown, maxMarkdownPreviewBytes)
	imagePaths, err := s.storeEmbeddedImages(ctx, doc, res.RawJSON)
	if err != nil {
		return err
	}

	if err := deleteDocumentExtractions(ctx, s.DB, string(doc.ID)); err != nil {
		s.log.Warn("clear old extraction", "error", err, "document_id", doc.ID)
	}

	if len(res.RawJSON) > 0 {
		if err := s.storage.Upload(ctx, ocrJSONPath, bytes.NewReader(res.RawJSON)); err != nil {
			return fmt.Errorf("store docling json: %w", err)
		}
	}
	if searchableText != "" {
		if err := s.storage.Upload(ctx, textPath, strings.NewReader(searchableText)); err != nil {
			return fmt.Errorf("store extracted text: %w", err)
		}
	}
	if res.MdContent != "" {
		if err := s.storage.Upload(ctx, markdownPath, strings.NewReader(res.MdContent)); err != nil {
			return fmt.Errorf("store extracted markdown: %w", err)
		}
	}

	if _, err := surrealdb.Create[database.DocumentExtraction](ctx, s.DB, "document_extractions", &database.DocumentExtraction{
		DocumentID: string(doc.ID),
		Text:       textPreview,
		Language:   "en",
		Pages:      res.Pages,
		Metadata: map[string]any{
			"engine":           "docling",
			"task_id":          taskID,
			"ocr_json_path":    ocrJSONPath,
			"full_text_path":   textPath,
			"markdown_path":    markdownPath,
			"full_text_bytes":  len(searchableText),
			"text_truncated":   len(textPreview) < len(searchableText),
			"markdown_preview": markdownPreview,
			"pages":            res.Pages,
			"timings":          res.Timings,
			"image_paths":      imagePaths,
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
	metadata["ocr_engine"] = "docling"
	metadata["ocr_json_path"] = ocrJSONPath
	metadata["ocr_text_path"] = textPath
	metadata["ocr_markdown_path"] = markdownPath
	metadata["ocr_image_paths"] = imagePaths

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, docRID, map[string]any{
		"status":     database.DocumentStatusReady,
		"metadata":   metadata,
		"updated_at": now,
	})

	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"status":       database.ProcessingJobStatusCompleted,
				"result":       map[string]any{"task_id": taskID, "engine": "docling", "ocr_json_path": ocrJSONPath, "full_text_path": textPath, "markdown_path": markdownPath, "image_paths": imagePaths},
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
			Content:      indexContent,
			Summary:      capUTF8(searchableText, 600),
			Extension:    strings.TrimPrefix(strings.ToLower(filepath.Ext(doc.OriginalName)), "."),
			OCREngine:    "docling",
			Status:       database.DocumentStatusReady,
			CreatedAt:    doc.CreatedAt,
			UpdatedAt:    now,
		}); err != nil {
			s.log.Warn("index document", "error", err, "document_id", doc.ID)
		}
	}

	s.log.Info("document ocr complete", "document_id", doc.ID)
	return nil
}

func (s *OCRService) storeEmbeddedImages(ctx context.Context, doc *database.Document, rawJSON []byte) ([]string, error) {
	const marker = "data:image/"
	var paths []string
	for offset := 0; ; {
		start := bytes.Index(rawJSON[offset:], []byte(marker))
		if start < 0 {
			break
		}
		start += offset
		comma := bytes.IndexByte(rawJSON[start:], ',')
		if comma < 0 {
			break
		}
		comma += start
		header := string(rawJSON[start:comma])
		if !strings.Contains(header, ";base64") {
			offset = comma + 1
			continue
		}
		end := comma + 1
		for end < len(rawJSON) && (rawJSON[end] == '+' || rawJSON[end] == '/' || rawJSON[end] == '=' || rawJSON[end] == '-' || rawJSON[end] == '_' || rawJSON[end] >= 'A' && rawJSON[end] <= 'Z' || rawJSON[end] >= 'a' && rawJSON[end] <= 'z' || rawJSON[end] >= '0' && rawJSON[end] <= '9') {
			end++
		}
		data, err := base64.StdEncoding.DecodeString(string(rawJSON[comma+1 : end]))
		if err != nil {
			return nil, fmt.Errorf("decode embedded docling image: %w", err)
		}
		if len(data) == 0 {
			offset = end
			continue
		}
		path := fmt.Sprintf("documents/%s/images/image-%03d.%s", documentArtifactID(doc), len(paths)+1, imageExtension(header))
		if err := s.storage.Upload(ctx, path, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("store docling image: %w", err)
		}
		paths = append(paths, path)
		offset = end
	}
	return paths, nil
}

func imageExtension(header string) string {
	typeEnd := strings.IndexByte(strings.TrimPrefix(header, "data:image/"), ';')
	if typeEnd < 0 {
		return "bin"
	}
	switch strings.ToLower(header[len("data:image/") : len("data:image/")+typeEnd]) {
	case "jpeg", "jpg":
		return "jpg"
	case "png", "webp", "gif":
		return strings.ToLower(header[len("data:image/") : len("data:image/")+typeEnd])
	default:
		return "bin"
	}
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

func ocrArtifactPath(doc *database.Document, filename string) string {
	return "documents/" + documentArtifactID(doc) + "/" + filename
}

func documentArtifactID(doc *database.Document) string {
	id := string(doc.ID)
	if _, key, ok := strings.Cut(id, ":"); ok {
		return key
	}
	return id
}

func isLegacyOffice(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".doc", ".dot", ".xls", ".xlt", ".ppt", ".pps", ".pot":
		return true
	default:
		return false
	}
}

func capUTF8(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	capped := value[:limit]
	for !utf8.ValidString(capped) && limit > 0 {
		limit--
		capped = value[:limit]
	}
	return capped + "\n\n[truncated - full text is stored with the OCR artifacts]"
}
