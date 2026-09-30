package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	applicationai "github.com/xmtrxi/edms/internal/ai"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/storage"
)

// maxLabelText caps how much extracted text is scanned per document.
const maxLabelText = 2 << 20

// LabelService runs after OCR: it detects sensitive data and suggests a
// sensitivity label, and, when AI is configured, classifies and summarizes.
// It only ever suggests raising a label; people decide.
type LabelService struct {
	db       *surrealdb.DB
	storage  storage.Provider
	provider applicationai.Provider
	log      *slog.Logger
}

func NewLabelService(db *surrealdb.DB, provider storage.Provider, ai applicationai.Provider, logger *slog.Logger) *LabelService {
	if logger == nil {
		logger = slog.Default()
	}
	if ai == nil {
		ai = applicationai.DisabledProvider{}
	}
	return &LabelService{db: db, storage: provider, provider: ai, log: logger}
}

// Findings counts sensitive items found in a document's text.
type Findings map[string]int

// piiPatterns find identifiers. Card numbers are confirmed with a Luhn check.
var piiPatterns = map[string]*regexp.Regexp{
	"email":       regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	"phone":       regexp.MustCompile(`(?:\+?254|\b0)[17]\d{8}\b|\+\d{11,14}\b`),
	"kra_pin":     regexp.MustCompile(`\b[AP]\d{9}[A-Z]\b`),
	"national_id": regexp.MustCompile(`(?i)\b(?:national\s+)?id\s*(?:no\.?|number|#)?\s*[:.]?\s*(\d{7,8})\b`),
	"passport":    regexp.MustCompile(`(?i)\bpassport\s*(?:no\.?|number|#)?\s*[:.]?\s*[A-Z]{1,2}\d{6,8}\b`),
	"iban":        regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){3,7}\b`),
	"card_number": regexp.MustCompile(`\b(?:\d[ \-]?){12,18}\d\b`),
}

// strongIdentifiers make a document confidential on their own.
var strongIdentifiers = []string{"card_number", "kra_pin", "national_id", "passport", "iban"}

// DetectSensitiveData scans text for personal and financial identifiers.
func DetectSensitiveData(text string) Findings {
	found := Findings{}
	for kind, pattern := range piiPatterns {
		matches := pattern.FindAllString(text, -1)
		count := 0
		for _, match := range matches {
			if kind == "card_number" && !luhn(match) {
				continue
			}
			count++
		}
		if count > 0 {
			found[kind] = count
		}
	}
	return found
}

func luhn(value string) bool {
	digits := make([]int, 0, len(value))
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if (len(digits)-1-i)%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

var sensitivityRank = map[string]int{
	database.SensitivityPublic: 0, database.SensitivityInternal: 1,
	database.SensitivityConfidential: 2, database.SensitivityRestricted: 3,
}

// SuggestSensitivity returns a stricter label than current when the findings
// call for one, or "" when current is already strict enough.
func SuggestSensitivity(current string, found Findings) string {
	suggested := database.SensitivityInternal
	for _, kind := range strongIdentifiers {
		if found[kind] > 0 {
			suggested = database.SensitivityConfidential
			break
		}
	}
	if current == "" {
		current = database.SensitivityInternal
	}
	if sensitivityRank[suggested] > sensitivityRank[current] {
		return suggested
	}
	return ""
}

// Process labels one document after its text has been extracted.
func (s *LabelService) Process(ctx context.Context, documentID string) error {
	rid, err := database.ParseRecordID(documentID)
	if err != nil {
		return err
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" {
		return ErrDocumentNotFound
	}
	text := s.text(ctx, documentID)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	metadata := doc.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	found := DetectSensitiveData(text)
	metadata["detected_pii"] = found
	if suggestion := SuggestSensitivity(doc.Sensitivity, found); suggestion != "" {
		metadata["suggested_sensitivity"] = suggestion
	} else {
		delete(metadata, "suggested_sensitivity")
	}

	sample := text
	if len(sample) > 12_000 {
		sample = sample[:12_000]
	}
	if classification, err := s.provider.Classify(ctx, applicationai.DocumentContext{DocumentID: documentID, Name: doc.Name, Text: sample}); err == nil {
		metadata["ai_classification"] = classification
	} else if !errors.Is(err, applicationai.ErrDisabled) {
		s.log.Warn("classify document", "document_id", documentID, "error", err)
	}
	if summary, err := s.provider.Complete(ctx, "Summarize this document in at most three sentences for someone deciding whether to open it. Mention dates, amounts and parties when present.\n\nTitle: "+doc.Name+"\n\n"+sample); err == nil {
		metadata["ai_summary"] = strings.TrimSpace(summary)
	} else if !errors.Is(err, applicationai.ErrDisabled) {
		s.log.Warn("summarize document", "document_id", documentID, "error", err)
	}
	metadata["labelled_at"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := surrealdb.Merge[database.Document](ctx, s.db, rid, map[string]any{"metadata": metadata})
	if err != nil {
		return err
	}
	fireAutomation(ctx, database.TriggerDocumentLabelled, updated)
	return nil
}

// text reads the full extracted text, falling back to the stored excerpt.
func (s *LabelService) text(ctx context.Context, documentID string) string {
	extractions, err := queryRows[database.DocumentExtraction](ctx, s.db, "SELECT * FROM document_extractions WHERE document_id = $id LIMIT 1", map[string]any{"id": documentID})
	if err != nil || len(extractions) == 0 {
		return ""
	}
	extraction := extractions[0]
	if path := metadataString(extraction.Metadata, "full_text_path", ""); path != "" && s.storage != nil {
		if reader, err := s.storage.Download(ctx, path); err == nil {
			defer reader.Close()
			if body, err := io.ReadAll(io.LimitReader(reader, maxLabelText)); err == nil && len(body) > 0 {
				return string(body)
			}
		}
	}
	return extraction.Text
}
