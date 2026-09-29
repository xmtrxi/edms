package docling

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/xmtrxi/edms/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func jsonResponse(payload string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(payload))}
}

func TestChunkFileAsyncSendsHybridChunkOptions(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chunk/hybrid/file/async" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		for key, want := range map[string]string{
			"convert_do_ocr": "true", "convert_table_mode": "accurate",
			"convert_image_export_mode": "placeholder", "chunking_max_tokens": "512",
			"chunking_merge_peers": "true", "chunking_use_markdown_tables": "true",
			"chunking_use_markdown_images": "false", "chunking_include_raw_text": "true",
			"include_converted_doc": "true",
		} {
			if got := r.FormValue(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		return jsonResponse(`{"task_id":"chunk-task"}`), nil
	})

	client := New(config.DoclingConfig{BaseURL: "http://docling.test"})
	client.http.Transport = transport
	taskID, err := client.ChunkFileAsync(context.Background(), "scan.pdf", strings.NewReader("pdf"), ChunkOptions{
		Options:   Options{DoOCR: true, TableMode: "accurate", ImageExportMode: "placeholder"},
		MaxTokens: 512, MergePeers: true, UseMarkdownTables: true, IncludeRawText: true, IncludeConvertedDoc: true,
	})
	if err != nil {
		t.Fatalf("ChunkFileAsync: %v", err)
	}
	if taskID != "chunk-task" {
		t.Fatalf("task id = %q", taskID)
	}
}

func TestResultDecodesChunkMetadataAndConvertedDocument(t *testing.T) {
	transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(`{
          "chunks":[{"filename":"scan","chunk_index":2,"text":"Context","raw_text":"Raw","num_tokens":9,"headings":["Terms"],"page_numbers":[2,3]}],
          "documents":[{"content":{"json_content":{"page_count":3}}}],
          "status":"success"
        }`), nil
	})

	client := New(config.DoclingConfig{BaseURL: "http://docling.test"})
	client.http.Transport = transport
	result, err := client.Result(context.Background(), "task")
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if len(result.Chunks) != 1 || result.Chunks[0].ChunkIndex != 2 || result.Chunks[0].PageNumbers[1] != 3 {
		t.Fatalf("unexpected chunks: %#v", result.Chunks)
	}
	if result.TextContent != "Raw" || result.Pages != 3 {
		t.Fatalf("text = %q, pages = %d", result.TextContent, result.Pages)
	}
}
