package signing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

var ErrInvalidResponse = errors.New("signing provider returned an invalid response")

type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"`
}

type CreateRequest struct {
	Title      string
	FileName   string
	File       io.Reader
	Recipients []Recipient
	ExternalID string
}

type CreateResult struct {
	DocumentID string `json:"document_id"`
	SigningURL string `json:"signing_url,omitempty"`
}

type Provider interface {
	Create(ctx context.Context, request CreateRequest) (*CreateResult, error)
	DownloadCompleted(ctx context.Context, documentID, downloadURL string) (io.ReadCloser, error)
}

type DocumensoClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewDocumensoClient(baseURL, apiKey string, client *http.Client) *DocumensoClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &DocumensoClient{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: client}
}

func (c *DocumensoClient) Create(ctx context.Context, input CreateRequest) (*CreateResult, error) {
	if c.baseURL == "" || c.apiKey == "" {
		return nil, errors.New("documenso is not configured")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", input.Title)
	_ = writer.WriteField("externalId", input.ExternalID)
	recipients, _ := json.Marshal(input.Recipients)
	_ = writer.WriteField("recipients", string(recipients))
	part, err := writer.CreateFormFile("file", input.FileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, input.File); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/documents", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("documenso create failed (%d): %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var payload struct {
		ID          any    `json:"id"`
		DocumentID  any    `json:"documentId"`
		SigningURL  string `json:"signingUrl"`
		RedirectURL string `json:"redirectUrl"`
		Data        *struct {
			ID         any    `json:"id"`
			SigningURL string `json:"signingUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	id := stringifyID(payload.DocumentID)
	if id == "" {
		id = stringifyID(payload.ID)
	}
	signingURL := payload.SigningURL
	if signingURL == "" {
		signingURL = payload.RedirectURL
	}
	if payload.Data != nil {
		if id == "" {
			id = stringifyID(payload.Data.ID)
		}
		if signingURL == "" {
			signingURL = payload.Data.SigningURL
		}
	}
	if id == "" {
		return nil, ErrInvalidResponse
	}
	return &CreateResult{DocumentID: id, SigningURL: signingURL}, nil
}

func (c *DocumensoClient) DownloadCompleted(ctx context.Context, documentID, downloadURL string) (io.ReadCloser, error) {
	target := strings.TrimSpace(downloadURL)
	if target == "" {
		target = c.baseURL + "/api/v1/documents/" + url.PathEscape(documentID) + "/download"
	} else if parsed, err := url.Parse(target); err != nil || !parsed.IsAbs() || parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, errors.New("invalid signed document download URL")
	} else if base, err := url.Parse(c.baseURL); err != nil || !strings.EqualFold(parsed.Host, base.Host) {
		return nil, errors.New("signed document download URL must use the configured provider host")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("documenso download failed (%d): %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return response.Body, nil
}

func stringifyID(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

