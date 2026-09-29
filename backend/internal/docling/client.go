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

type ChunkOptions struct {
	Options
	MaxTokens           int
	MergePeers          bool
	UseMarkdownTables   bool
	UseMarkdownImages   bool
	ImagePlaceholder    string
	IncludeRawText      bool
	IncludeConvertedDoc bool
}

type Chunk struct {
	Filename    string         `json:"filename"`
	ChunkIndex  int            `json:"chunk_index"`
	Text        string         `json:"text"`
	RawText     string         `json:"raw_text,omitempty"`
	NumTokens   int            `json:"num_tokens,omitempty"`
	Headings    []string       `json:"headings,omitempty"`
	Captions    []string       `json:"captions,omitempty"`
	DocItems    []string       `json:"doc_items,omitempty"`
	PageNumbers []int          `json:"page_numbers,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
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
	Chunks      []Chunk
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

// ChunkFileAsync converts and chunks a document with Docling's native
// HybridChunker. Chunk-specific multipart fields intentionally use Docling
// Serve's chunking_ prefix; conversion fields use convert_.
func (c *Client) ChunkFileAsync(ctx context.Context, filename string, r io.Reader, opts ChunkOptions) (string, error) {
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
			writeChunkOptions(writer, opts)
			err = writer.Close()
		}
		if err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/chunk/hybrid/file/async"), pr)
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
		return "", fmt.Errorf("docling async chunking returned no task id")
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
		document = firstConvertedDocument(data)
	}
	chunks := decodeChunks(data["chunks"])
	if document == nil && len(chunks) == 0 {
		return nil, fmt.Errorf("docling result contains no document or chunks")
	}
	if document == nil {
		document = map[string]any{}
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
		Chunks:      chunks,
	}
	if result.JsonContent == nil {
		result.JsonContent = data
	}
	if result.TextContent == "" && result.MdContent != "" {
		result.TextContent = result.MdContent
	}
	if result.TextContent == "" && len(chunks) > 0 {
		parts := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			text := chunk.RawText
			if text == "" {
				text = chunk.Text
			}
			if strings.TrimSpace(text) != "" {
				parts = append(parts, strings.TrimSpace(text))
			}
		}
		result.TextContent = strings.Join(parts, "\n\n")
	}
	if result.MdContent == "" {
		result.MdContent = result.TextContent
	}
	return result, nil
}

func writeChunkOptions(w *multipart.Writer, opts ChunkOptions) {
	_ = w.WriteField("convert_do_ocr", strconv.FormatBool(opts.DoOCR))
	if opts.TableMode != "" {
		_ = w.WriteField("convert_table_mode", opts.TableMode)
	}
	if opts.ImageExportMode != "" {
		_ = w.WriteField("convert_image_export_mode", opts.ImageExportMode)
	}
	_ = w.WriteField("include_converted_doc", strconv.FormatBool(opts.IncludeConvertedDoc))
	if opts.MaxTokens > 0 {
		_ = w.WriteField("chunking_max_tokens", strconv.Itoa(opts.MaxTokens))
	}
	_ = w.WriteField("chunking_merge_peers", strconv.FormatBool(opts.MergePeers))
	_ = w.WriteField("chunking_use_markdown_tables", strconv.FormatBool(opts.UseMarkdownTables))
	_ = w.WriteField("chunking_use_markdown_images", strconv.FormatBool(opts.UseMarkdownImages))
	_ = w.WriteField("chunking_image_placeholder", opts.ImagePlaceholder)
	_ = w.WriteField("chunking_include_raw_text", strconv.FormatBool(opts.IncludeRawText))
}

func firstConvertedDocument(data map[string]any) map[string]any {
	documents, _ := data["documents"].([]any)
	if len(documents) == 0 {
		return nil
	}
	doc, _ := documents[0].(map[string]any)
	if content, ok := doc["content"].(map[string]any); ok {
		return content
	}
	return doc
}

func decodeChunks(value any) []Chunk {
	items, _ := value.([]any)
	chunks := make([]Chunk, 0, len(items))
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		chunk := Chunk{
			Filename: firstString(fields, "filename"), ChunkIndex: intValue(fields["chunk_index"]),
			Text: firstString(fields, "text"), RawText: firstString(fields, "raw_text"),
			NumTokens: intValue(fields["num_tokens"]), Headings: stringSlice(fields["headings"]),
			Captions: stringSlice(fields["captions"]), DocItems: stringSlice(fields["doc_items"]),
			PageNumbers: intSlice(fields["page_numbers"]),
		}
		chunk.Metadata, _ = fields["metadata"].(map[string]any)
		chunks = append(chunks, chunk)
	}
	return chunks
}

func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func stringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func intSlice(value any) []int {
	items, _ := value.([]any)
	result := make([]int, 0, len(items))
	for _, item := range items {
		result = append(result, intValue(item))
	}
	return result
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
