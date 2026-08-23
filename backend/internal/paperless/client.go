package paperless

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/xmtrxi/edms/internal/config"
	paperlessngxgo "github.com/xmtrxi/paperless-ngx.go"
)

type Client struct {
	inner *paperlessngxgo.PaperlessNgx
}

var duplicateDocumentPattern = regexp.MustCompile(`(?i)duplicate[\s\S]*\(#(\d+)\)`)

// DuplicateDocumentID extracts the existing Paperless document ID from a
// duplicate-consumption failure, for example "It is a duplicate ... (#3).".
func DuplicateDocumentID(result string) (int, bool) {
	matches := duplicateDocumentPattern.FindStringSubmatch(result)
	if len(matches) != 2 {
		return 0, false
	}
	id, err := strconv.Atoi(matches[1])
	return id, err == nil && id > 0
}

func New(cfg config.PaperlessConfig) *Client {
	return &Client{
		inner: paperlessngxgo.New(paperlessngxgo.PaperlessNgxConfig{
			BaseURL: cfg.BaseURL,
			Token:   cfg.Token,
		}),
	}
}

func (c *Client) UploadDocument(ctx context.Context, file io.Reader, filename, title string) (string, error) {
	return c.inner.UploadDocument(ctx, paperlessngxgo.UploadDocumentInput{
		File:     file,
		FileName: filename,
		Title:    title,
	})
}

func (c *Client) GetTask(ctx context.Context, taskID string) (*paperlessngxgo.TasksView, error) {
	tasks, err := c.inner.ListTasks(ctx, paperlessngxgo.ListTasksParams{TaskID: taskID})
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.TaskID == taskID {
			return &task, nil
		}
	}
	return nil, fmt.Errorf("paperless task %q not found", taskID)
}

func (c *Client) GetDocument(ctx context.Context, id int) (*paperlessngxgo.Document, error) {
	return c.inner.GetDocument(ctx, id)
}

// DocumentIDFromTaskResult extracts the created document id from a consume_file
// task result. Falls back to false when the result cannot be parsed.
func (c *Client) DocumentIDFromTaskResult(result *string) (int, bool) {
	if result == nil {
		return 0, false
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(*result), &data); err != nil {
		return 0, false
	}
	for _, key := range []string{"document_id", "id"} {
		if v, ok := data[key].(float64); ok {
			return int(v), true
		}
	}
	return 0, false
}

func (c *Client) GetDocumentByTitle(ctx context.Context, title string) (*paperlessngxgo.Document, error) {
	page, err := c.inner.ListDocuments(ctx, paperlessngxgo.ListDocumentsParams{
		Search:   title,
		PageSize: 1,
	})
	if err != nil {
		return nil, err
	}
	if page == nil || len(page.Results) == 0 {
		return nil, fmt.Errorf("no paperless document found for %q", title)
	}
	return &page.Results[0], nil
}

func (c *Client) DownloadArchive(ctx context.Context, id int) (io.ReadCloser, error) {
	dl, err := c.inner.DownloadDocument(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return dl.Body, nil
}
