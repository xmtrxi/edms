package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	applicationai "github.com/xmtrxi/edms/internal/ai"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

type AIInput struct {
	Question    string   `json:"question"`
	Text        string   `json:"text"`
	DocumentIDs []string `json:"document_ids"`
}
type AIService struct {
	db       *surrealdb.DB
	provider applicationai.Provider
}

func NewAIService(db *surrealdb.DB, cfg config.AIConfig) *AIService {
	var provider applicationai.Provider = applicationai.DisabledProvider{}
	if cfg.Enabled && cfg.BaseURL != "" {
		provider = applicationai.NewHTTPCompatibleProvider(cfg.BaseURL, cfg.APIKey, cfg.ChatModel, cfg.EmbeddingModel, nil)
	}
	return &AIService{db: db, provider: provider}
}

func (s *AIService) Run(ctx context.Context, actor, kind string, input AIInput) (*database.AIJob, error) {
	if len(input.DocumentIDs) > 10 {
		return nil, fmt.Errorf("select no more than 10 documents")
	}
	if kind == "ask" && strings.TrimSpace(input.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}
	job, err := surrealdb.Create[database.AIJob](ctx, s.db, "ai_jobs", &database.AIJob{UserID: actor, Kind: kind, Status: "running", DocumentIDs: input.DocumentIDs, Input: map[string]any{"question": input.Question, "text": input.Text}, CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, err
	}
	contexts, err := s.contexts(ctx, actor, input)
	if err != nil {
		return s.fail(ctx, job, err)
	}
	result := map[string]any{}
	switch kind {
	case "classify":
		if len(contexts) == 0 {
			err = fmt.Errorf("a document or text is required")
		} else {
			var classification applicationai.Classification
			classification, err = s.provider.Classify(ctx, contexts[0])
			result = map[string]any{"classification": classification}
		}
	case "ask":
		var answer applicationai.Answer
		answer, err = s.provider.Ask(ctx, input.Question, contexts)
		result = map[string]any{"answer": answer.Text, "citations": answer.Citations}
	case "summarize":
		var answer applicationai.Answer
		answer, err = s.provider.Ask(ctx, "Summarize these documents for an office worker. Include decisions, dates, owners, and next actions.", contexts)
		result = map[string]any{"summary": answer.Text, "citations": answer.Citations}
	case "extract":
		var answer applicationai.Answer
		answer, err = s.provider.Ask(ctx, "Extract key entities, dates, amounts, references, obligations, and action items. Return clear JSON.", contexts)
		result = map[string]any{"extraction": answer.Text, "citations": answer.Citations}
	default:
		err = fmt.Errorf("unsupported AI operation")
	}
	if err != nil {
		return s.fail(ctx, job, err)
	}
	completed := time.Now().UTC()
	merged, err := surrealdb.Merge[database.AIJob](ctx, s.db, mustRecordID(string(job.ID)), map[string]any{"status": "completed", "result": result, "completed_at": completed})
	if err != nil {
		return nil, err
	}
	return merged, nil
}
func (s *AIService) Get(ctx context.Context, actor, id string) (*database.AIJob, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("AI job not found")
	}
	job, err := surrealdb.Select[database.AIJob](ctx, s.db, rid)
	if err != nil || job == nil || job.UserID != actor {
		return nil, fmt.Errorf("AI job not found")
	}
	return job, nil
}
func (s *AIService) contexts(ctx context.Context, actor string, input AIInput) ([]applicationai.DocumentContext, error) {
	items := []applicationai.DocumentContext{}
	if input.Text != "" {
		text := input.Text
		if len(text) > 40000 {
			text = text[:40000]
		}
		items = append(items, applicationai.DocumentContext{Name: "Provided text", Text: text})
	}
	for _, id := range input.DocumentIDs {
		rid, err := database.ParseRecordID(id)
		if err != nil {
			return nil, fmt.Errorf("document not found")
		}
		doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
		if err != nil || doc == nil || doc.ID == "" {
			return nil, fmt.Errorf("document not found")
		}
		// The assistant only reads documents the person could open themselves.
		if allowed, accessErr := NewAccessService(s.db).CanDocument(ctx, actor, "view", doc); accessErr != nil || !allowed {
			return nil, fmt.Errorf("document not found")
		}
		extractions, err := queryRows[database.DocumentExtraction](ctx, s.db, "SELECT * FROM document_extractions WHERE document_id = $id ORDER BY updated_at DESC LIMIT 1", map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		text := ""
		if len(extractions) > 0 {
			text = extractions[0].Text
		}
		if len(text) > 40000 {
			text = text[:40000]
		}
		items = append(items, applicationai.DocumentContext{DocumentID: id, Name: doc.Name, Text: text, Metadata: doc.Metadata})
	}
	return items, nil
}
func (s *AIService) fail(ctx context.Context, job *database.AIJob, err error) (*database.AIJob, error) {
	_, _ = surrealdb.Merge[database.AIJob](ctx, s.db, mustRecordID(string(job.ID)), map[string]any{"status": "failed", "error": err.Error(), "completed_at": time.Now().UTC()})
	if errors.Is(err, applicationai.ErrDisabled) {
		return nil, applicationai.ErrDisabled
	}
	return nil, err
}
