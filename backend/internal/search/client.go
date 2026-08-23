package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"github.com/xmtrxi/edms/internal/config"
)

type Client struct {
	inner meilisearch.ServiceManager
	index string
}

func New(cfg config.SearchConfig) *Client {
	return &Client{
		inner: meilisearch.New(cfg.URL, meilisearch.WithAPIKey(cfg.APIKey)),
		index: cfg.Index,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}
	_, err := c.inner.HealthWithContext(ctx)
	return err
}

type Document struct {
	// ID is a Meilisearch-safe primary key. RecordID retains the canonical
	// SurrealDB id (for example, "documents:abc") used by the application.
	ID           string    `json:"id"`
	RecordID     string    `json:"record_id"`
	OwnerID      string    `json:"owner_id"`
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	Content      string    `json:"content"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func documentID(recordID string) string {
	sum := sha256.Sum256([]byte(recordID))
	return hex.EncodeToString(sum[:])
}

// Configure creates the index when necessary and applies the attributes used
// by authenticated searches as filters.
func (c *Client) Configure(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("search is not configured")
	}

	if _, err := c.inner.GetIndexWithContext(ctx, c.index); err != nil {
		task, createErr := c.inner.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{
			Uid:        c.index,
			PrimaryKey: "id",
		})
		if createErr != nil {
			return fmt.Errorf("create search index: %w", createErr)
		}
		if err := c.waitForTask(ctx, task.TaskUID); err != nil {
			return fmt.Errorf("create search index: %w", err)
		}
	}

	task, err := c.inner.Index(c.index).UpdateSettingsWithContext(ctx, &meilisearch.Settings{
		FilterableAttributes: []string{"owner_id", "status"},
		SearchableAttributes: []string{"name", "original_name", "content"},
	})
	if err != nil {
		return fmt.Errorf("configure search index: %w", err)
	}
	if err := c.waitForTask(ctx, task.TaskUID); err != nil {
		return fmt.Errorf("configure search index: %w", err)
	}
	return nil
}

func (c *Client) IndexDocument(ctx context.Context, doc Document) error {
	if c == nil || c.inner == nil {
		return nil
	}
	doc.RecordID = strings.TrimSpace(doc.RecordID)
	if doc.RecordID == "" {
		return fmt.Errorf("document record id is required")
	}
	doc.ID = documentID(doc.RecordID)
	pk := "id"
	task, err := c.inner.Index(c.index).AddDocumentsWithContext(ctx, []Document{doc}, &meilisearch.DocumentOptions{PrimaryKey: &pk})
	if err != nil {
		return err
	}
	return c.waitForTask(ctx, task.TaskUID)
}

func (c *Client) DeleteDocument(ctx context.Context, id string) error {
	if c == nil || c.inner == nil {
		return nil
	}
	task, err := c.inner.Index(c.index).DeleteDocumentWithContext(ctx, documentID(id), nil)
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
	Query    string
	OwnerID  string
	Status   string
	Page     int
	PageSize int
}

type SearchResult struct {
	IDs       []string
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

	filters := make([]string, 0, 3)
	if in.OwnerID != "" {
		filters = append(filters, fmt.Sprintf("owner_id = %q", in.OwnerID))
	}
	if in.Status != "" {
		filters = append(filters, fmt.Sprintf("status = %q", in.Status))
	}

	req := &meilisearch.SearchRequest{
		Offset: int64((in.Page - 1) * in.PageSize),
		Limit:  int64(in.PageSize),
	}
	if len(filters) > 0 {
		req.Filter = strings.Join(filters, " AND ")
	}

	res, err := c.inner.Index(c.index).SearchWithContext(ctx, in.Query, req)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(res.Hits))
	for _, hit := range res.Hits {
		var doc Document
		if err := hit.DecodeInto(&doc); err != nil {
			continue
		}
		recordID := strings.TrimSpace(doc.RecordID)
		// Backward compatibility for entries created before record_id existed.
		if recordID == "" && strings.Contains(doc.ID, ":") {
			recordID = doc.ID
		}
		if recordID != "" {
			ids = append(ids, recordID)
		}
	}

	return &SearchResult{
		IDs:       ids,
		TotalHits: res.TotalHits,
		Page:      in.Page,
		PageSize:  in.PageSize,
	}, nil
}
