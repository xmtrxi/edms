// Package pdftools runs PDF operations on a Stirling-PDF server.
package pdftools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Operations the app offers. Each maps to one Stirling-PDF endpoint.
const (
	OpMerge       = "merge"
	OpCompress    = "compress"
	OpRotate      = "rotate"
	OpWatermark   = "watermark"
	OpPageNumbers = "page_numbers"
)

// IsOperation reports whether op is supported.
func IsOperation(op string) bool {
	switch op {
	case OpMerge, OpCompress, OpRotate, OpWatermark, OpPageNumbers:
		return true
	}
	return false
}

// Input is one PDF sent to Stirling.
type Input struct {
	Name string
	Body io.Reader
}

// Options tune an operation; unused fields are ignored.
type Options struct {
	// Level is the compression level, 1 (light) to 9 (strongest).
	Level int `json:"level,omitempty"`
	// Angle rotates every page clockwise by 90, 180 or 270 degrees.
	Angle int `json:"angle,omitempty"`
	// Text is the watermark text.
	Text string `json:"text,omitempty"`
	// Opacity is the watermark opacity from 0.05 to 1.
	Opacity float64 `json:"opacity,omitempty"`
	// Position places page numbers: 1-9 as on a phone keypad, 8 is bottom centre.
	Position int `json:"position,omitempty"`
}

// Validate checks options for the operation and fills defaults.
func (o *Options) Validate(op string, inputs int) error {
	switch op {
	case OpMerge:
		if inputs < 2 {
			return fmt.Errorf("choose at least two PDFs to merge")
		}
		return nil
	}
	if inputs != 1 {
		return fmt.Errorf("this tool works on one document at a time")
	}
	switch op {
	case OpCompress:
		if o.Level == 0 {
			o.Level = 5
		}
		if o.Level < 1 || o.Level > 9 {
			return fmt.Errorf("compression level must be between 1 and 9")
		}
	case OpRotate:
		if o.Angle != 90 && o.Angle != 180 && o.Angle != 270 {
			return fmt.Errorf("rotate by 90, 180 or 270 degrees")
		}
	case OpWatermark:
		o.Text = strings.TrimSpace(o.Text)
		if o.Text == "" || len(o.Text) > 200 {
			return fmt.Errorf("watermark text must be between 1 and 200 characters")
		}
		if o.Opacity == 0 {
			o.Opacity = 0.3
		}
		if o.Opacity < 0.05 || o.Opacity > 1 {
			return fmt.Errorf("opacity must be between 0.05 and 1")
		}
	case OpPageNumbers:
		if o.Position == 0 {
			o.Position = 8
		}
		if o.Position < 1 || o.Position > 9 {
			return fmt.Errorf("page number position must be between 1 and 9")
		}
	default:
		return fmt.Errorf("unknown PDF tool %q", op)
	}
	return nil
}

// Client calls the Stirling-PDF REST API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: &http.Client{Timeout: timeout}}
}

// Run performs op and returns the resulting PDF.
func (c *Client) Run(ctx context.Context, op string, inputs []Input, opts Options) ([]byte, error) {
	if err := opts.Validate(op, len(inputs)); err != nil {
		return nil, err
	}
	var path string
	fields := map[string]string{}
	switch op {
	case OpMerge:
		path = "/api/v1/general/merge-pdfs"
		fields["sortType"] = "orderProvided"
		fields["removeCertSign"] = "false"
	case OpCompress:
		path = "/api/v1/misc/compress-pdf"
		fields["optimizeLevel"] = strconv.Itoa(opts.Level)
	case OpRotate:
		path = "/api/v1/general/rotate-pdf"
		fields["angle"] = strconv.Itoa(opts.Angle)
	case OpWatermark:
		path = "/api/v1/security/add-watermark"
		fields["watermarkType"] = "text"
		fields["watermarkText"] = opts.Text
		fields["alphabet"] = "roman"
		fields["fontSize"] = "30"
		fields["rotation"] = "45"
		fields["opacity"] = strconv.FormatFloat(opts.Opacity, 'f', 2, 64)
		fields["widthSpacer"] = "50"
		fields["heightSpacer"] = "50"
	case OpPageNumbers:
		path = "/api/v1/misc/add-page-numbers"
		fields["position"] = strconv.Itoa(opts.Position)
		fields["startingNumber"] = "1"
		fields["customMargin"] = "medium"
		fields["pagesToNumber"] = ""
		fields["customText"] = "{n}"
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for _, in := range inputs {
		part, err := form.CreateFormFile("fileInput", in.Name)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, in.Body); err != nil {
			return nil, err
		}
	}
	for key, value := range fields {
		if err := form.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if err := form.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Accept", "application/pdf")
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stirling-pdf %s: %w", op, err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<30))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(out[:min(len(out), 300)]))}
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		return nil, fmt.Errorf("stirling-pdf %s returned something other than a PDF", op)
	}
	return out, nil
}

// Ping checks the server answers.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/info/status", nil)
	if err != nil {
		return err
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("stirling-pdf status %d", resp.StatusCode)
	}
	return nil
}

// HTTPError is a refused request. Client errors (4xx) are not worth retrying.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("stirling-pdf returned %d: %s", e.Status, e.Body)
}

// Retryable reports whether trying again might succeed.
func (e *HTTPError) Retryable() bool {
	return e.Status >= 500 || e.Status == http.StatusTooManyRequests
}
