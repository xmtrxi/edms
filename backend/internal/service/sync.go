package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	surrealgoorm "github.com/kemboi22/surreal-go-orm"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

// SyncResult describes one storage reconciliation run.
type SyncResult struct {
	DryRun      bool     `json:"dry_run"`
	Scanned     int      `json:"scanned"`
	Imported    int      `json:"imported"`
	WouldImport int      `json:"would_import,omitempty"`
	Skipped     int      `json:"skipped"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors,omitempty"`
}

// Sync reconciles files from the configured storage provider. A dry run reads
// and checks provider files but never creates document rows or queues OCR.
// Workers run concurrently; OCR remains a durable queue task for real imports.
func (s *DocumentService) Sync(ctx context.Context, ownerID string, workers int, dryRun bool) (*SyncResult, error) {
	paths, err := s.listStorageFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list storage files: %w", err)
	}
	if workers <= 0 {
		workers = min(4, runtime.GOMAXPROCS(0))
	}
	if workers > 16 {
		workers = 16
	}

	result := &SyncResult{DryRun: dryRun, Scanned: len(paths)}
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex

	worker := func() {
		defer wg.Done()
		for path := range jobs {
			imported, importErr := s.importStorageFile(ctx, ownerID, path, dryRun)
			mu.Lock()
			switch {
			case importErr != nil:
				result.Failed++
				if len(result.Errors) < 20 {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, importErr))
				}
			case imported && dryRun:
				result.WouldImport++
			case imported:
				result.Imported++
			default:
				result.Skipped++
			}
			mu.Unlock()
		}
	}

	wg.Add(workers)
	for range workers {
		go worker()
	}
	for _, path := range paths {
		select {
		case jobs <- path:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return result, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return result, nil
}

func (s *DocumentService) listStorageFiles(ctx context.Context) ([]string, error) {
	var paths []string
	var cursor *string
	for {
		batch, next, err := s.storage.List(ctx, "", cursor, 500)
		if err != nil {
			return nil, err
		}
		paths = append(paths, batch...)
		if next == nil || *next == "" {
			break
		}
		cursor = next
	}
	return paths, nil
}

func (s *DocumentService) importStorageFile(ctx context.Context, ownerID, storagePath string, dryRun bool) (bool, error) {
	storagePath = filepath.ToSlash(strings.TrimPrefix(filepath.Clean(storagePath), "/"))
	if storagePath == "." || isGeneratedArtifact(storagePath) {
		return false, nil
	}

	existing, err := s.findByStoragePath(ctx, storagePath)
	if err != nil || existing != nil {
		return false, err
	}

	file, err := s.storage.Download(ctx, storagePath)
	if err != nil {
		return false, fmt.Errorf("download: %w", err)
	}
	checksum, size, err := hashReader(file)
	closeErr := file.Close()
	if err != nil {
		return false, fmt.Errorf("checksum: %w", err)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close: %w", closeErr)
	}

	// Serialize the final checks with uploads and other sync workers. Both checks
	// are repeated inside the critical section to prevent concurrent imports.
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, err = s.findByStoragePath(ctx, storagePath); err != nil || existing != nil {
		return false, err
	}

	// A file copied to another folder is still the same document. Keep the
	// existing row instead of sending its bytes through OCR a second time.
	existing, err = s.findByChecksum(ctx, ownerID, checksum)
	if err != nil || existing != nil {
		return false, err
	}

	// At this point a real sync would import the file. Dry runs stop before any
	// database, queue, OCR, or search side effects.
	if dryRun {
		return true, nil
	}

	now := time.Now().UTC()
	name := filepath.Base(storagePath)
	folderPath := folderPathForStoragePath(storagePath)
	doc := &database.Document{
		ID:              database.RecordID(newDocumentID()),
		OwnerID:         ownerID,
		Name:            name,
		OriginalName:    name,
		MimeType:        mimeTypeForName(name),
		Size:            size,
		StorageProvider: s.storage.Name(),
		StoragePath:     storagePath,
		Checksum:        checksum,
		Status:          database.DocumentStatusUploaded,
		Metadata: generatedMetadata(name, storagePath, checksum, map[string]any{
			"source":      "storage_sync",
			"folder_path": folderPath,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	}
	created, err := surrealdb.Create[database.Document](ctx, s.DB, "documents", doc)
	if err != nil {
		return false, fmt.Errorf("create document record: %w", err)
	}
	if _, err := createInitialVersion(ctx, s.DB, created, ownerID, database.VersionSourceStorageSync); err != nil {
		return false, fmt.Errorf("create initial document version: %w", err)
	}
	s.stageOCR(ctx, created, storagePath)
	return true, nil
}

func (s *DocumentService) backfillMissingChecksums(ctx context.Context, ownerID string) error {
	results, err := surrealdb.Query[[]database.Document](ctx, s.DB,
		"SELECT * FROM documents WHERE owner_id = $owner AND (checksum = NONE OR checksum = '')",
		map[string]any{"owner": ownerID},
	)
	if err != nil {
		return err
	}
	if results == nil || len(*results) == 0 {
		return nil
	}
	for _, doc := range (*results)[0].Result {
		file, err := s.storage.Download(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("download %s: %w", doc.StoragePath, err)
		}
		checksum, size, hashErr := hashReader(file)
		closeErr := file.Close()
		if hashErr != nil {
			return fmt.Errorf("hash %s: %w", doc.StoragePath, hashErr)
		}
		if closeErr != nil {
			return closeErr
		}
		rid, err := database.ParseRecordID(string(doc.ID))
		if err != nil {
			return err
		}
		if _, err := surrealdb.Merge[database.Document](ctx, s.DB, rid, map[string]any{
			"checksum": checksum,
			"size":     size,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *DocumentService) findByChecksum(ctx context.Context, ownerID, checksum string) (*database.Document, error) {
	if checksum == "" {
		return nil, nil
	}
	return surrealgoorm.Query[database.Document](s.DB, "documents").
		WhereEq("owner_id", ownerID).
		WhereEq("checksum", checksum).
		First(ctx)
}

func (s *DocumentService) findByStoragePath(ctx context.Context, path string) (*database.Document, error) {
	return surrealgoorm.Query[database.Document](s.DB, "documents").
		WhereEq("storage_path", path).
		First(ctx)
}

func hashReader(reader io.Reader) (string, int64, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, reader)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func mimeTypeForName(name string) string {
	if value := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); value != "" {
		return value
	}
	return "application/octet-stream"
}

func generatedMetadata(name, storagePath, checksum string, supplied map[string]any) map[string]any {
	metadata := map[string]any{
		"filename":     name,
		"extension":    strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
		"storage_path": storagePath,
		"sha256":       checksum,
	}
	for key, value := range supplied {
		metadata[key] = value
	}
	return metadata
}

func isGeneratedArtifact(path string) bool {
	cleaned := filepath.ToSlash(path)
	if strings.HasPrefix(cleaned, "documents/") {
		parts := strings.Split(cleaned, "/")
		if len(parts) > 3 && strings.HasPrefix(parts[1], "doc_") && parts[2] == "images" {
			return true
		}
		if len(parts) == 3 && strings.HasPrefix(parts[1], "doc_") {
			switch parts[2] {
			case "ocr.json", "extracted.txt", "extracted.md":
				return true
			}
		}
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return base == "archive" && strings.HasPrefix(cleaned, "documents/doc_")
}

func folderPathForStoragePath(storagePath string) string {
	dir := filepath.ToSlash(filepath.Dir(storagePath))
	if dir == "." || dir == "" {
		return ""
	}
	if strings.HasPrefix(dir, "documents/doc_") {
		return ""
	}
	return cleanFolderPath(dir)
}

func folderMetadata(metadata map[string]any, folderPath, folderColor string, categoryIDs []string) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	if path := cleanFolderPath(folderPath); path != "" {
		metadata["folder_path"] = path
	}
	if color := strings.TrimSpace(folderColor); color != "" {
		metadata["folder_color"] = color
	}
	if ids := cleanStringSlice(categoryIDs); len(ids) > 0 {
		metadata["folder_category_ids"] = ids
	}
	return metadata
}

func cleanFolderPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.Trim(path, "/")
	if path == "." {
		return ""
	}
	parts := strings.Split(path, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		clean = append(clean, part)
	}
	return strings.Join(clean, "/")
}

func cleanStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return cleanStringSlice(values)
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value, ok := value.(string); ok {
				result = append(result, value)
			}
		}
		return cleanStringSlice(result)
	}
	return nil
}
