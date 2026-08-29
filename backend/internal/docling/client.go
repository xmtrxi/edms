package docling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xmtrxi/edms/internal/config"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type Options struct {
	ToFormats       []string
	DoOCR           bool
	TableMode       string
	ImageExportMode string
}

type TaskStatus struct {
	TaskID   string
	Status   string
	Position *int
	Meta     map[string]any
	Result   *string
	Error    string
}

type ConvertResult struct {
	TextContent string
	MdContent   string
	JsonContent map[string]any
	RawJSON     []byte
	Pages       int
	Status      string
	Timings     map[string]float64
}

func New(cfg config.DoclingConfig) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) ConvertFileAsync(ctx context.Context, filename string, r io.Reader, opts Options) (string, error) {
	if c == nil || c.baseURL == "" {
		return "", fmt.Errorf("docling client is not configured")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		part, err := writer.CreateFormFile("files", filename)
		if err == nil {
			_, err = io.Copy(part, r)
		}
		if err == nil {
			writeOptions(writer, opts)
			err = writer.Close()
		}
		if err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/convert/file/async"), pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.authorize(req)

	var data map[string]any
	if err := c.doJSON(req, &data); err != nil {
		return "", err
	}
	taskID := firstString(data, "task_id", "taskId", "id")
	if taskID == "" {
		return "", fmt.Errorf("docling async conversion returned no task id")
	}
	return taskID, nil
}

func (c *Client) ConvertSourceAsync(ctx context.Context, sourceURL string, opts Options) (string, error) {
	if c == nil || c.baseURL == "" {
		return "", fmt.Errorf("docling client is not configured")
	}
	body := map[string]any{
		"http_sources": []map[string]string{{"url": sourceURL}},
	}
	applyOptions(body, opts)
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/convert/source/async"), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)

	var data map[string]any
	if err := c.doJSON(req, &data); err != nil {
		return "", err
	}
	taskID := firstString(data, "task_id", "taskId", "id")
	if taskID == "" {
		return "", fmt.Errorf("docling source conversion returned no task id")
	}
	return taskID, nil
}

func (c *Client) Poll(ctx context.Context, taskID string) (*TaskStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/v1/status/poll/"+url.PathEscape(taskID)), nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)

	var data map[string]any
	if err := c.doJSON(req, &data); err != nil {
		return nil, err
	}
	result := firstString(data, "result")
	status := &TaskStatus{
		TaskID:   firstString(data, "task_id", "taskId", "id"),
		Status:   strings.ToLower(firstString(data, "task_status", "status", "state")),
		Position: firstIntPointer(data, "task_position", "position"),
		Meta:     firstMap(data, "task_meta", "meta"),
		Error:    firstString(data, "error", "message", "detail"),
	}
	if result != "" {
		status.Result = &result
	}
	if status.TaskID == "" {
		status.TaskID = taskID
	}
	return status, nil
}

func firstIntPointer(data map[string]any, keys ...string) *int {
	for _, key := range keys {
		switch value := data[key].(type) {
		case float64:
			result := int(value)
			return &result
		case int:
			return &value
		}
	}
	return nil
}

func firstMap(data map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := data[key].(map[string]any); ok {
			return value
		}
	}
	return nil
}

func (c *Client) Result(ctx context.Context, taskID string) (*ConvertResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/v1/result/"+url.PathEscape(taskID)), nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)

	var data map[string]any
	raw, err := c.doRawJSON(req, &data)
	if err != nil {
		return nil, err
	}
	document, _ := data["document"].(map[string]any)
	if document == nil {
		return nil, fmt.Errorf("docling result contains no document")
	}
	jsonContent, _ := document["json_content"].(map[string]any)
	result := &ConvertResult{
		TextContent: firstString(document, "text_content", "plain_text", "text"),
		MdContent:   firstString(document, "md_content", "markdown", "markdown_content"),
		JsonContent: jsonContent,
		RawJSON:     raw,
		Pages:       findInt(jsonContent, "pages", "num_pages", "page_count"),
		Status:      strings.ToLower(firstString(data, "status")),
		Timings:     findFloatMap(data, "timings"),
	}
	if result.JsonContent == nil {
		result.JsonContent = data
	}
	if result.TextContent == "" && result.MdContent != "" {
		result.TextContent = result.MdContent
	}
	return result, nil
}

func (c *Client) endpoint(suffix string) string {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return c.baseURL + suffix
	}
	u.Path = path.Join(u.Path, suffix)
	return u.String()
}

func (c *Client) authorize(req *http.Request) {
	if c.apiKey == "" {
		return
	}
	req.Header.Set("X-Api-Key", c.apiKey)
}

func (c *Client) doJSON(req *http.Request, dst any) error {
	_, err := c.doRawJSON(req, dst)
	return err
}

func (c *Client) doRawJSON(req *http.Request, dst any) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docling %s %s failed: %s", req.Method, req.URL.Path, strings.TrimSpace(string(body)))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("docling returned an empty response")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return nil, fmt.Errorf("decode docling response: %w", err)
	}
	return body, nil
}

func writeOptions(w *multipart.Writer, opts Options) {
	formats := opts.ToFormats
	if len(formats) == 0 {
		formats = []string{"json", "md", "text"}
	}
	for _, format := range formats {
		_ = w.WriteField("to_formats", format)
	}
	_ = w.WriteField("do_ocr", strconv.FormatBool(opts.DoOCR))
	if opts.TableMode != "" {
		_ = w.WriteField("table_mode", opts.TableMode)
	}
	if opts.ImageExportMode != "" {
		_ = w.WriteField("image_export_mode", opts.ImageExportMode)
	}
}

func applyOptions(body map[string]any, opts Options) {
	formats := opts.ToFormats
	if len(formats) == 0 {
		formats = []string{"json", "md", "text"}
	}
	body["to_formats"] = formats
	body["do_ocr"] = opts.DoOCR
	if opts.TableMode != "" {
		body["table_mode"] = opts.TableMode
	}
	if opts.ImageExportMode != "" {
		body["image_export_mode"] = opts.ImageExportMode
	}
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok {
			return value
		}
	}
	return ""
}

func findString(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if v, ok := typed[key].(string); ok {
				return v
			}
		}
		for _, child := range typed {
			if v := findString(child, keys...); v != "" {
				return v
			}
		}
	case []any:
		for _, child := range typed {
			if v := findString(child, keys...); v != "" {
				return v
			}
		}
	}
	return ""
}

func findMap(value any, keys ...string) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if v, ok := typed[key].(map[string]any); ok {
				return v
			}
		}
		for _, child := range typed {
			if v := findMap(child, keys...); v != nil {
				return v
			}
		}
	case []any:
		for _, child := range typed {
			if v := findMap(child, keys...); v != nil {
				return v
			}
		}
	}
	return nil
}

func findInt(value any, keys ...string) int {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			switch v := typed[key].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
		for _, child := range typed {
			if v := findInt(child, keys...); v > 0 {
				return v
			}
		}
	case []any:
		for _, child := range typed {
			if v := findInt(child, keys...); v > 0 {
				return v
			}
		}
	}
	return 0
}

func findFloatMap(value any, key string) map[string]float64 {
	data, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := data[key].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]float64, len(raw))
	for k, v := range raw {
		if n, ok := v.(float64); ok {
			result[k] = n
		}
	}
	return result
}
