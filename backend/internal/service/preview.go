package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/gotenberg"
	"github.com/xmtrxi/edms/internal/storage"
)

var ErrNoPreview = errors.New("no preview is available for this file type")

// Preview kinds the viewer understands.
const (
	PreviewPDF   = "pdf"
	PreviewImage = "image"
	PreviewText  = "text"
	PreviewVideo = "video"
	PreviewAudio = "audio"
)

// officeExtensions are converted to PDF for viewing.
var officeExtensions = map[string]bool{
	".doc": true, ".docx": true, ".odt": true, ".rtf": true,
	".xls": true, ".xlsx": true, ".ods": true, ".csv": false,
	".ppt": true, ".pptx": true, ".odp": true,
}

// PreviewService serves a browser-viewable rendition of a document. Office
// files are converted once per version through Gotenberg and cached next to
// the original.
type PreviewService struct {
	docs    *DocumentService
	storage storage.Provider
	office  *gotenberg.Client
}

func NewPreviewService(docs *DocumentService, provider storage.Provider, office *gotenberg.Client) *PreviewService {
	return &PreviewService{docs: docs, storage: provider, office: office}
}

// Preview is an opened rendition ready to stream.
type Preview struct {
	Kind     string
	MimeType string
	Name     string
	Body     io.ReadCloser
}

// PreviewKind classifies a mime type and file name for the viewer.
func PreviewKind(mime, name string) (kind string, convert bool) {
	mime = strings.ToLower(mime)
	ext := strings.ToLower(path.Ext(name))
	switch {
	case mime == "application/pdf" || ext == ".pdf":
		return PreviewPDF, false
	case strings.HasPrefix(mime, "image/"):
		return PreviewImage, false
	case strings.HasPrefix(mime, "video/"):
		return PreviewVideo, false
	case strings.HasPrefix(mime, "audio/"):
		return PreviewAudio, false
	case strings.HasPrefix(mime, "text/") || ext == ".csv" || ext == ".md" || ext == ".json" || ext == ".txt":
		return PreviewText, false
	case officeExtensions[ext]:
		return PreviewPDF, true
	}
	return "", false
}

// Open returns the rendition of the document's current version.
func (s *PreviewService) Open(ctx context.Context, userID, documentID string) (*Preview, error) {
	doc, err := s.docs.GetFor(ctx, userID, "view", documentID)
	if err != nil {
		return nil, err
	}
	return s.Rendition(ctx, doc)
}

// Rendition opens the viewable form of doc's current version. Callers check
// access first.
func (s *PreviewService) Rendition(ctx context.Context, doc *database.Document) (*Preview, error) {
	kind, convert := PreviewKind(doc.MimeType, doc.Name)
	if kind == "" {
		return nil, ErrNoPreview
	}
	if !convert {
		body, err := s.storage.Download(ctx, doc.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		mime := doc.MimeType
		if kind == PreviewText {
			mime = "text/plain; charset=utf-8"
		}
		return &Preview{Kind: kind, MimeType: mime, Name: doc.Name, Body: body}, nil
	}

	version := doc.CurrentVersionID
	if version == "" {
		version = doc.Checksum
	}
	_, key, _ := strings.Cut(version, ":")
	if key == "" {
		key = version
	}
	cachePath := path.Join(path.Dir(doc.StoragePath), "preview-"+key+".pdf")
	if exists, _ := s.storage.Exists(ctx, cachePath); exists {
		body, err := s.storage.Download(ctx, cachePath)
		if err == nil {
			return &Preview{Kind: PreviewPDF, MimeType: "application/pdf", Name: doc.Name + ".pdf", Body: body}, nil
		}
	}
	if s.office == nil {
		return nil, ErrNoPreview
	}
	original, err := s.storage.Download(ctx, doc.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer original.Close()
	pdf, err := s.office.ConvertOfficeToPDF(ctx, doc.Name, original)
	if err != nil {
		return nil, fmt.Errorf("convert for preview: %w", err)
	}
	// A failed cache write only costs a later re-conversion.
	_ = s.storage.Upload(ctx, cachePath, bytes.NewReader(pdf))
	return &Preview{Kind: PreviewPDF, MimeType: "application/pdf", Name: doc.Name + ".pdf", Body: io.NopCloser(bytes.NewReader(pdf))}, nil
}
