package service

import (
	"bytes"
	"context"
	"encoding/json"
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
	chunkerVersion            = "docling-hybrid-v1:max_tokens=512"
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

func (s *OCRService) ProcessOCR(ctx context.Context, documentID, storagePath, versionID, checksum, generationID, queuedJobID string) error {
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
	if versionID == "" {
		versionID = doc.CurrentVersionID
	}
	if checksum == "" {
		checksum = doc.Checksum
	}
	if generationID == "" {
		generationID = newDocumentID()
	}
	if doc.CurrentVersionID != versionID || (checksum != "" && doc.Checksum != checksum) {
		s.completeSupersededJob(ctx, queuedJobID, versionID)
		return nil
	}

	jobID := queuedJobID
	if jobID == "" {
		job, lookupErr := surrealgoorm.Query[database.ProcessingJob](s.DB, "processing_jobs").WhereEq("document_id", documentID).OrderBy("created_at DESC").First(ctx)
		if lookupErr == nil && job != nil {
			jobID = string(job.ID)
		}
	}
	if jobID != "" {
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
		s.failDocument(ctx, doc, jobID, versionID, fmt.Errorf("download original: %w", err))
		return err
	}
	defer file.Close()

	filename := doc.OriginalName
	input := io.Reader(file)
	if isLegacyOffice(filename) {
		if s.office == nil {
			err := fmt.Errorf("legacy office document requires gotenberg conversion")
			s.failDocument(ctx, doc, jobID, versionID, err)
			return err
		}
		converted, err := s.office.ConvertOfficeToPDF(ctx, filename, file)
		if err != nil {
			err = fmt.Errorf("legacy office convert: %w", err)
			s.failDocument(ctx, doc, jobID, versionID, err)
			return err
		}
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".pdf"
		input = bytes.NewReader(converted)
	}

	opts := docling.ChunkOptions{
		Options:   docling.Options{DoOCR: true, TableMode: "accurate", ImageExportMode: "placeholder"},
		MaxTokens: 512, MergePeers: true, UseMarkdownTables: true,
		UseMarkdownImages: false, ImagePlaceholder: "", IncludeRawText: true,
		IncludeConvertedDoc: true,
		// Keep Markdown portable and readable. Embedded images are data URLs and
		// make both the OCR preview and indexed text unusable for large documents.
	}
	taskID, err := s.docling.ChunkFileAsync(ctx, filename, input, opts)
	if err != nil {
		err = fmt.Errorf("submit to docling: %w", err)
		s.failDocument(ctx, doc, jobID, versionID, err)
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
		s.failDocument(ctx, doc, jobID, versionID, err)
		return err
	}
	fresh, err := surrealdb.Select[database.Document](ctx, s.DB, docID)
	if err != nil {
		return err
	}
	if fresh == nil || fresh.CurrentVersionID != versionID || (checksum != "" && fresh.Checksum != checksum) {
		s.completeSupersededJob(ctx, jobID, versionID)
		return nil
	}
	if err := s.completeDocument(ctx, fresh, jobID, taskID, versionID, generationID, res); err != nil {
		s.failDocument(ctx, doc, jobID, versionID, err)
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

func (s *OCRService) completeDocument(ctx context.Context, doc *database.Document, jobID, taskID, versionID, generationID string, res *docling.ConvertResult) error {
	now := time.Now().UTC()
	docRID, _ := database.ParseRecordID(string(doc.ID))
	ocrJSONPath := versionOCRArtifactPath(doc, versionID, "ocr.json")
	textPath := versionOCRArtifactPath(doc, versionID, "extracted.txt")
	markdownPath := versionOCRArtifactPath(doc, versionID, "extracted.md")
	chunksPath := versionOCRArtifactPath(doc, versionID, "chunks.jsonl")
	searchableText := markdownutil.WithoutEmbeddedDataImages(res.TextContent)
	searchableMarkdown := markdownutil.WithoutEmbeddedDataImages(res.MdContent)
	textPreview := capUTF8(searchableText, maxExtractionPreviewBytes)
	markdownPreview := capUTF8(searchableMarkdown, maxMarkdownPreviewBytes)
	chunks := buildSearchChunks(doc, versionID, generationID, res.Chunks, searchableText, now)
	overview := extractiveOverview(chunks)
	chunkArtifact, err := encodeChunksJSONL(chunks)
	if err != nil {
		return fmt.Errorf("encode chunks: %w", err)
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
	if err := s.storage.Upload(ctx, chunksPath, bytes.NewReader(chunkArtifact)); err != nil {
		return fmt.Errorf("store chunks: %w", err)
	}
	current, err := s.isCurrentVersion(ctx, string(doc.ID), versionID, doc.Checksum)
	if err != nil {
		return err
	}
	if !current {
		s.completeSupersededJob(ctx, jobID, versionID)
		return nil
	}
	if err := deleteDocumentExtractions(ctx, s.DB, string(doc.ID)); err != nil {
		s.log.Warn("clear old extraction", "error", err, "document_id", doc.ID)
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
			"chunks_path":      chunksPath,
			"chunk_count":      len(chunks),
			"chunker_version":  chunkerVersion,
			"version_id":       versionID,
			"generation_id":    generationID,
			"overview":         overview,
			"full_text_bytes":  len(searchableText),
			"text_truncated":   len(textPreview) < len(searchableText),
			"markdown_preview": markdownPreview,
			"pages":            res.Pages,
			"timings":          res.Timings,
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
	metadata["ocr_chunks_path"] = chunksPath
	metadata["ocr_chunk_count"] = len(chunks)
	metadata["ocr_chunker_version"] = chunkerVersion
	metadata["ocr_version_id"] = versionID
	metadata["ocr_generation_id"] = generationID
	metadata["ocr_overview"] = overview

	_, _ = surrealdb.Merge[database.Document](ctx, s.DB, docRID, map[string]any{
		"status":     database.DocumentStatusReady,
		"metadata":   metadata,
		"updated_at": now,
	})

	if jobID != "" {
		if jobRID, rerr := database.ParseRecordID(jobID); rerr == nil {
			_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, jobRID, map[string]any{
				"status":       database.ProcessingJobStatusCompleted,
				"result":       map[string]any{"task_id": taskID, "engine": "docling", "ocr_json_path": ocrJSONPath, "full_text_path": textPath, "markdown_path": markdownPath, "chunks_path": chunksPath, "chunk_count": len(chunks), "version_id": versionID, "generation_id": generationID},
				"completed_at": now,
				"updated_at":   now,
			})
		}
	}

	if s.search != nil {
		if err := s.search.IndexChunks(ctx, chunks); err != nil {
			s.log.Warn("index document", "error", err, "document_id", doc.ID)
		} else if err := s.search.DeleteOlderGenerations(ctx, string(doc.ID), generationID); err != nil {
			s.log.Warn("remove old search generation", "error", err, "document_id", doc.ID)
		}
	}

	s.log.Info("document ocr complete", "document_id", doc.ID)
	return nil
}

func (s *OCRService) failDocument(ctx context.Context, doc *database.Document, jobID, versionID string, cause error) {
	current, err := s.isCurrentVersion(ctx, string(doc.ID), versionID, doc.Checksum)
	if err == nil && !current {
		s.completeSupersededJob(ctx, jobID, versionID)
		return
	}
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

func (s *OCRService) isCurrentVersion(ctx context.Context, documentID, versionID, checksum string) (bool, error) {
	rid, err := database.ParseRecordID(documentID)
	if err != nil {
		return false, err
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.DB, rid)
	if err != nil || doc == nil {
		return false, err
	}
	return doc.CurrentVersionID == versionID && (checksum == "" || doc.Checksum == checksum), nil
}

func (s *OCRService) completeSupersededJob(ctx context.Context, jobID, versionID string) {
	if jobID == "" {
		return
	}
	rid, err := database.ParseRecordID(jobID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	_, _ = surrealdb.Merge[database.ProcessingJob](ctx, s.DB, rid, map[string]any{
		"status":       database.ProcessingJobStatusCompleted,
		"result":       map[string]any{"status": "superseded", "version_id": versionID},
		"completed_at": now,
		"updated_at":   now,
	})
}

func buildSearchChunks(doc *database.Document, versionID, generationID string, source []docling.Chunk, fallbackText string, now time.Time) []search.DocumentChunk {
	if len(source) == 0 {
		source = fallbackChunks(fallbackText)
	}
	result := make([]search.DocumentChunk, 0, len(source))
	for _, item := range source {
		content := markdownutil.WithoutEmbeddedDataImages(item.Text)
		if content == "" {
			content = markdownutil.WithoutEmbeddedDataImages(item.RawText)
		}
		if content == "" {
			continue
		}
		pages := positiveInts(item.PageNumbers)
		pageStart, pageEnd := 0, 0
		if len(pages) > 0 {
			pageStart, pageEnd = pages[0], pages[0]
			for _, page := range pages[1:] {
				if page < pageStart {
					pageStart = page
				}
				if page > pageEnd {
					pageEnd = page
				}
			}
		}
		result = append(result, search.DocumentChunk{
			DocumentID: string(doc.ID), VersionID: versionID, GenerationID: generationID,
			ChunkIndex: len(result), Title: doc.Name, Filename: doc.OriginalName,
			Headings: cleanStrings(item.Headings), PageNumbers: pages, PageStart: pageStart, PageEnd: pageEnd,
			OwnerID: doc.OwnerID, FolderID: doc.FolderID, MimeType: doc.MimeType,
			Extension: strings.TrimPrefix(strings.ToLower(filepath.Ext(doc.OriginalName)), "."),
			OCREngine: "docling", Status: database.DocumentStatusReady, Content: content, UpdatedAt: now,
		})
	}
	return result
}

func fallbackChunks(value string) []docling.Chunk {
	value = markdownutil.WithoutEmbeddedDataImages(value)
	if value == "" {
		return nil
	}
	const target = 3000
	paragraphs := strings.Split(value, "\n\n")
	chunks := make([]docling.Chunk, 0, len(paragraphs)/3+1)
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			chunks = append(chunks, docling.Chunk{ChunkIndex: len(chunks), Text: text})
		}
		current.Reset()
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if current.Len() > 0 && current.Len()+len(paragraph)+2 > target {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	flush()
	return chunks
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if cleaned := markdownutil.WithoutEmbeddedDataImages(value); cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func positiveInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := map[int]bool{}
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func extractiveOverview(chunks []search.DocumentChunk) string {
	parts := make([]string, 0, 3)
	for _, chunk := range chunks {
		if len(parts) == 3 {
			break
		}
		if text := strings.TrimSpace(chunk.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return capUTF8(strings.Join(parts, "\n\n"), 1200)
}

func encodeChunksJSONL(chunks []search.DocumentChunk) ([]byte, error) {
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	for _, chunk := range chunks {
		if err := encoder.Encode(chunk); err != nil {
			return nil, err
		}
	}
	return payload.Bytes(), nil
}

func versionOCRArtifactPath(doc *database.Document, versionID, filename string) string {
	versionKey := versionID
	if _, key, ok := strings.Cut(versionID, ":"); ok {
		versionKey = key
	}
	return "documents/" + documentArtifactID(doc) + "/versions/" + versionKey + "/ocr/" + filename
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
