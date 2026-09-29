package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"github.com/xmtrxi/edms/internal/config"
)

const indexBatchSize = 500

type Client struct {
	inner meilisearch.ServiceManager
	index string
}

func New(cfg config.SearchConfig) *Client {
	return &Client{inner: meilisearch.New(cfg.URL, meilisearch.WithAPIKey(cfg.APIKey)), index: cfg.Index}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}
	_, err := c.inner.HealthWithContext(ctx)
	return err
}

// DocumentChunk is the unit indexed in Meilisearch. The JSONL artifact in
// object storage remains the source of truth and can rebuild this record.
type DocumentChunk struct {
	ID           string    `json:"id"`
	DocumentID   string    `json:"document_id"`
	VersionID    string    `json:"version_id"`
	GenerationID string    `json:"generation_id"`
	ChunkIndex   int       `json:"chunk_index"`
	Title        string    `json:"title"`
	Filename     string    `json:"filename"`
	Headings     []string  `json:"headings"`
	PageNumbers  []int     `json:"page_numbers"`
	PageStart    int       `json:"page_start"`
	PageEnd      int       `json:"page_end"`
	OrgID        string    `json:"org_id"`
	OwnerID      string    `json:"owner_id"`
	FolderID     string    `json:"folder_id"`
	MimeType     string    `json:"mime_type"`
	Extension    string    `json:"extension"`
	OCREngine    string    `json:"ocr_engine"`
	Status       string    `json:"status"`
	Content      string    `json:"content"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func chunkID(chunk DocumentChunk) string {
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", chunk.DocumentID, chunk.VersionID, chunk.GenerationID, chunk.ChunkIndex)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (c *Client) Configure(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}
	if _, err := c.inner.GetIndexWithContext(ctx, c.index); err != nil {
		task, createErr := c.inner.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{Uid: c.index, PrimaryKey: "id"})
		if createErr != nil {
			return fmt.Errorf("create search index: %w", createErr)
		}
		if err := c.waitForTask(ctx, task.TaskUID); err != nil {
			return fmt.Errorf("create search index: %w", err)
		}
	}
	task, err := c.inner.Index(c.index).UpdateSettingsWithContext(ctx, indexSettings())
	if err != nil {
		return fmt.Errorf("configure search index: %w", err)
	}
	return c.waitForTask(ctx, task.TaskUID)
}

func indexSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		FilterableAttributes: []string{"document_id", "version_id", "generation_id", "org_id", "owner_id", "folder_id", "status", "mime_type", "extension", "ocr_engine"},
		SearchableAttributes: []string{"title", "filename", "headings", "content"},
		DisplayedAttributes:  []string{"id", "document_id", "version_id", "generation_id", "chunk_index", "title", "filename", "headings", "page_numbers", "page_start", "page_end", "org_id", "owner_id", "folder_id", "mime_type", "extension", "ocr_engine", "status", "content", "updated_at"},
		SortableAttributes:   []string{"updated_at", "title", "page_start", "chunk_index"},
		RankingRules:         []string{"words", "typo", "proximity", "attribute", "sort", "exactness"},
	}
}

func (c *Client) IndexChunks(ctx context.Context, chunks []DocumentChunk) error {
	if c == nil || c.inner == nil || len(chunks) == 0 {
		return nil
	}
	for i := range chunks {
		chunks[i].DocumentID = strings.TrimSpace(chunks[i].DocumentID)
		if chunks[i].DocumentID == "" || chunks[i].VersionID == "" || chunks[i].GenerationID == "" {
			return fmt.Errorf("chunk document, version, and generation ids are required")
		}
		chunks[i].ID = chunkID(chunks[i])
	}
	return c.indexChunks(ctx, c.index, chunks)
}

func (c *Client) indexChunks(ctx context.Context, index string, chunks []DocumentChunk) error {
	pk := "id"
	for start := 0; start < len(chunks); start += indexBatchSize {
		end := min(start+indexBatchSize, len(chunks))
		task, err := c.inner.Index(index).AddDocumentsWithContext(ctx, chunks[start:end], &meilisearch.DocumentOptions{PrimaryKey: &pk})
		if err != nil {
			return err
		}
		if err := c.waitForTask(ctx, task.TaskUID); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceAll builds a verified temporary index, atomically swaps it into the
// live UID, then removes the old index now held under the temporary UID.
func (c *Client) ReplaceAll(ctx context.Context, chunks []DocumentChunk) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}
	if err := c.Configure(ctx); err != nil {
		return err
	}
	tempIndex := fmt.Sprintf("%s_rebuild_%d", c.index, time.Now().UTC().UnixNano())
	createTask, err := c.inner.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{Uid: tempIndex, PrimaryKey: "id"})
	if err != nil {
		return fmt.Errorf("create replacement index: %w", err)
	}
	if err := c.waitForTask(ctx, createTask.TaskUID); err != nil {
		return err
	}
	cleanup := func() {
		if task, deleteErr := c.inner.DeleteIndexWithContext(context.Background(), tempIndex); deleteErr == nil {
			_ = c.waitForTask(context.Background(), task.TaskUID)
		}
	}
	settingsTask, err := c.inner.Index(tempIndex).UpdateSettingsWithContext(ctx, indexSettings())
	if err != nil {
		cleanup()
		return err
	}
	if err := c.waitForTask(ctx, settingsTask.TaskUID); err != nil {
		cleanup()
		return err
	}
	for i := range chunks {
		chunks[i].DocumentID = strings.TrimSpace(chunks[i].DocumentID)
		if chunks[i].DocumentID == "" || chunks[i].VersionID == "" || chunks[i].GenerationID == "" {
			cleanup()
			return fmt.Errorf("replacement chunk document, version, and generation ids are required")
		}
		chunks[i].ID = chunkID(chunks[i])
	}
	if err := c.indexChunks(ctx, tempIndex, chunks); err != nil {
		cleanup()
		return fmt.Errorf("populate replacement index: %w", err)
	}
	swapTask, err := c.inner.SwapIndexesWithContext(ctx, []*meilisearch.SwapIndexesParams{{Indexes: []string{c.index, tempIndex}}})
	if err != nil {
		cleanup()
		return fmt.Errorf("swap replacement index: %w", err)
	}
	if err := c.waitForTask(ctx, swapTask.TaskUID); err != nil {
		cleanup()
		return err
	}
	deleteTask, err := c.inner.DeleteIndexWithContext(ctx, tempIndex)
	if err != nil {
		return fmt.Errorf("remove old search index: %w", err)
	}
	return c.waitForTask(ctx, deleteTask.TaskUID)
}

func (c *Client) DeleteDocument(ctx context.Context, documentID string) error {
	return c.deleteByFilter(ctx, fmt.Sprintf("document_id = %s", quote(documentID)))
}

func (c *Client) DeleteOlderGenerations(ctx context.Context, documentID, generationID string) error {
	return c.deleteByFilter(ctx, fmt.Sprintf("document_id = %s AND generation_id != %s", quote(documentID), quote(generationID)))
}

func (c *Client) deleteByFilter(ctx context.Context, filter string) error {
	if c == nil || c.inner == nil {
		return nil
	}
	task, err := c.inner.Index(c.index).DeleteDocumentsByFilterWithContext(ctx, filter, nil)
	if err != nil {
		return err
	}
	return c.waitForTask(ctx, task.TaskUID)
}

func (c *Client) waitForTask(ctx context.Context, taskUID int64) error {
	task, err := c.inner.WaitForTaskWithContext(ctx, taskUID, 50*time.Millisecond)
	if err != nil {
		return err
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		return fmt.Errorf("meilisearch task %d ended with status %s", taskUID, task.Status)
	}
	return nil
}

type SearchInput struct {
	Query  string
	UserID string
	// OrgID limits results to one organization. Chunks indexed before
	// organizations existed have no org_id and stay searchable until a reindex;
	// callers re-check access on every hit.
	OrgID string
	// AllInOrg skips the per-user filter (organization admins).
	AllInOrg bool
	// DocumentIDs are individually granted documents.
	DocumentIDs []string
	FolderIDs   []string
	DocumentID  string
	Status      string
	MimeType    string
	Sort        string
	Mode        string
	Page        int
	PageSize    int
}

type Hit struct {
	DocumentID   string
	VersionID    string
	GenerationID string
	ChunkIndex   int
	Title        string
	Filename     string
	Headings     []string
	PageNumbers  []int
	PageStart    int
	PageEnd      int
	MimeType     string
	Status       string
	Snippet      string
	UpdatedAt    time.Time
}

type SearchResult struct {
	Hits      []Hit
	TotalHits int64
	Page      int
	PageSize  int
}

func (c *Client) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("search is not configured")
	}
	if in.PageSize <= 0 {
		in.PageSize = 50
	}
	if in.Page <= 0 {
		in.Page = 1
	}
	filters := []string{accessFilter(in)}
	if in.DocumentID != "" {
		filters = append(filters, fmt.Sprintf("document_id = %s", quote(in.DocumentID)))
	}
	if in.Status != "" {
		filters = append(filters, fmt.Sprintf("status = %s", quote(in.Status)))
	}
	if in.MimeType != "" {
		filters = append(filters, fmt.Sprintf("mime_type = %s", quote(in.MimeType)))
	}
	req := &meilisearch.SearchRequest{
		Offset: int64((in.Page - 1) * in.PageSize), Limit: int64(in.PageSize),
		AttributesToHighlight: []string{"title", "filename", "headings", "content"},
		AttributesToCrop:      []string{"content:45"}, CropMarker: "…",
		HighlightPreTag: "", HighlightPostTag: "", Filter: strings.Join(filters, " AND "),
	}
	if in.Mode != "passages" {
		req.Distinct = "document_id"
	}
	if in.Sort != "" {
		req.Sort = []string{in.Sort}
	}
	res, err := c.inner.Index(c.index).SearchWithContext(ctx, in.Query, req)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(res.Hits))
	for _, rawHit := range res.Hits {
		fields := map[string]any{}
		if err := rawHit.DecodeInto(&fields); err != nil {
			continue
		}
		formatted, _ := fields["_formatted"].(map[string]any)
		chunk := chunkFromMap(fields)
		if chunk.DocumentID == "" {
			continue
		}
		hits = append(hits, Hit{
			DocumentID: chunk.DocumentID, VersionID: chunk.VersionID, GenerationID: chunk.GenerationID,
			ChunkIndex: chunk.ChunkIndex, Title: chunk.Title, Filename: chunk.Filename,
			Headings: chunk.Headings, PageNumbers: chunk.PageNumbers, PageStart: chunk.PageStart,
			PageEnd: chunk.PageEnd, MimeType: chunk.MimeType, Status: chunk.Status,
			Snippet: bestSnippet(formatted, chunk), UpdatedAt: chunk.UpdatedAt,
		})
	}
	return &SearchResult{Hits: hits, TotalHits: res.TotalHits, Page: in.Page, PageSize: in.PageSize}, nil
}

func accessFilter(in SearchInput) string {
	org := ""
	if in.OrgID != "" {
		org = fmt.Sprintf("(org_id = %s OR org_id NOT EXISTS)", quote(in.OrgID))
		if in.AllInOrg {
			return org
		}
	}
	clauses := []string{fmt.Sprintf("owner_id = %s", quote(in.UserID))}
	if len(in.FolderIDs) > 0 {
		clauses = append(clauses, fmt.Sprintf("folder_id IN [%s]", quoteAll(in.FolderIDs)))
	}
	if len(in.DocumentIDs) > 0 {
		clauses = append(clauses, fmt.Sprintf("document_id IN [%s]", quoteAll(in.DocumentIDs)))
	}
	visible := clauses[0]
	if len(clauses) > 1 {
		visible = "(" + strings.Join(clauses, " OR ") + ")"
	}
	if org == "" {
		return visible
	}
	return org + " AND " + visible
}

func quoteAll(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, quote(value))
	}
	return strings.Join(quoted, ",")
}

func quote(value string) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}

func chunkFromMap(fields map[string]any) DocumentChunk {
	return DocumentChunk{
		DocumentID: stringField(fields, "document_id"), VersionID: stringField(fields, "version_id"),
		GenerationID: stringField(fields, "generation_id"), ChunkIndex: intField(fields, "chunk_index"),
		Title: stringField(fields, "title"), Filename: stringField(fields, "filename"),
		Headings: stringsField(fields, "headings"), PageNumbers: intsField(fields, "page_numbers"),
		PageStart: intField(fields, "page_start"), PageEnd: intField(fields, "page_end"),
		MimeType: stringField(fields, "mime_type"), Status: stringField(fields, "status"),
		Content: stringField(fields, "content"), UpdatedAt: timeField(fields, "updated_at"),
	}
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func intField(fields map[string]any, key string) int {
	switch value := fields[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	}
	return 0
}

func stringsField(fields map[string]any, key string) []string {
	values, _ := fields[key].([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func intsField(fields map[string]any, key string) []int {
	values, _ := fields[key].([]any)
	result := make([]int, 0, len(values))
	for _, value := range values {
		switch n := value.(type) {
		case float64:
			result = append(result, int(n))
		case int:
			result = append(result, n)
		}
	}
	return result
}

func timeField(fields map[string]any, key string) time.Time {
	value, _ := fields[key].(string)
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func bestSnippet(formatted map[string]any, chunk DocumentChunk) string {
	for _, key := range []string{"content", "headings", "title", "filename"} {
		switch value := formatted[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case []any:
			parts := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, " › ")
			}
		}
	}
	content := strings.TrimSpace(chunk.Content)
	if len(content) > 320 {
		return content[:320] + "…"
	}
	return content
}
