package service

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

// maxPassageChars bounds each passage so a prompt stays within model limits.
const maxPassageChars = 1800

// AnswerInput is a question over the drive, a folder or chosen documents.
type AnswerInput struct {
	Question    string   `json:"question"`
	FolderID    string   `json:"folder_id"`
	DocumentIDs []string `json:"document_ids"`
}

// Citation points an answer back to the passage it came from.
type Citation struct {
	Index        int    `json:"index"`
	DocumentID   string `json:"document_id"`
	DocumentName string `json:"document_name"`
	VersionID    string `json:"version_id,omitempty"`
	PageStart    int    `json:"page_start,omitempty"`
	PageEnd      int    `json:"page_end,omitempty"`
	Snippet      string `json:"snippet"`
}

type AnswerResult struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	// Sources are every passage the model was given, cited or not.
	Sources []Citation `json:"sources"`
}

// passage is one retrieved piece of text the user is allowed to read.
type passage struct {
	Citation
	text string
}

// Answer retrieves passages the user can open, asks the model to answer only
// from them, and returns the answer with the passages it cited. Access is
// enforced by retrieval, so the model never sees text the user could not.
func (s *AIService) Answer(ctx context.Context, userID string, in AnswerInput) (*AnswerResult, error) {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return nil, fmt.Errorf("ask a question")
	}
	passages, err := s.retrieve(ctx, userID, question, in, 8)
	if err != nil {
		return nil, err
	}
	result := &AnswerResult{Citations: []Citation{}, Sources: make([]Citation, 0, len(passages))}
	for _, p := range passages {
		result.Sources = append(result.Sources, p.Citation)
	}
	if len(passages) == 0 {
		result.Answer = "I couldn’t find anything about that in the documents you can open."
		return result, nil
	}
	text, err := s.provider.Complete(ctx, answerPrompt(question, passages))
	if err != nil {
		return nil, err
	}
	result.Answer = text
	for _, index := range citedIndexes(text, len(passages)) {
		result.Citations = append(result.Citations, passages[index-1].Citation)
	}
	s.record(ctx, userID, "answer", in, map[string]any{"answer": text, "citations": len(result.Citations)})
	return result, nil
}

func answerPrompt(question string, passages []passage) string {
	var b strings.Builder
	b.WriteString("You answer questions for people at work using only the numbered sources below.\n")
	b.WriteString("Rules: cite every fact with its source number in square brackets, like [2]. ")
	b.WriteString("If the sources do not contain the answer, say you could not find it. ")
	b.WriteString("Be concise and use the same language as the question.\n\n")
	for i, p := range passages {
		fmt.Fprintf(&b, "[%d] %s", i+1, p.DocumentName)
		if p.PageStart > 0 {
			if p.PageEnd > p.PageStart {
				fmt.Fprintf(&b, " (pages %d–%d)", p.PageStart, p.PageEnd)
			} else {
				fmt.Fprintf(&b, " (page %d)", p.PageStart)
			}
		}
		b.WriteString("\n")
		b.WriteString(p.text)
		b.WriteString("\n\n")
	}
	b.WriteString("Question: ")
	b.WriteString(question)
	return b.String()
}

var citationPattern = regexp.MustCompile(`\[(\d{1,2})\]`)

// citedIndexes returns the distinct source numbers in order of first use.
func citedIndexes(text string, count int) []int {
	result := []int{}
	for _, match := range citationPattern.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(match[1])
		if err != nil || n < 1 || n > count || slices.Contains(result, n) {
			continue
		}
		result = append(result, n)
	}
	return result
}

// Retrieve exposes permission-aware passage search to agents and the UI.
func (s *AIService) Retrieve(ctx context.Context, userID string, in AnswerInput, limit int) ([]Citation, []string, error) {
	passages, err := s.retrieve(ctx, userID, in.Question, in, limit)
	if err != nil {
		return nil, nil, err
	}
	citations := make([]Citation, 0, len(passages))
	texts := make([]string, 0, len(passages))
	for _, p := range passages {
		citations = append(citations, p.Citation)
		texts = append(texts, p.text)
	}
	return citations, texts, nil
}

func (s *AIService) retrieve(ctx context.Context, userID, query string, in AnswerInput, limit int) ([]passage, error) {
	if s.search == nil {
		return nil, fmt.Errorf("search is not configured")
	}
	allowedFolders, err := s.folderScope(ctx, userID, in.FolderID)
	if err != nil {
		return nil, err
	}
	inScope := func(doc *database.Document) bool {
		if len(in.DocumentIDs) > 0 && !slices.Contains(in.DocumentIDs, string(doc.ID)) {
			return false
		}
		return allowedFolders == nil || allowedFolders[doc.FolderID]
	}
	search := SearchInput{Query: query, OwnerID: userID, Mode: "passages", Page: 1, PageSize: 40}
	if len(in.DocumentIDs) == 1 {
		search.DocumentID = in.DocumentIDs[0]
	}
	result, err := s.search.Search(ctx, search)
	if err != nil {
		return nil, err
	}
	passages := []passage{}
	for i := range result.Items {
		item := result.Items[i]
		if !inScope(&item.Document) {
			continue
		}
		text := item.Content
		if text == "" {
			// Database search returns whole documents; read the best window.
			text = s.window(ctx, &item.Document, query)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if len(text) > maxPassageChars {
			text = text[:maxPassageChars] + "…"
		}
		passages = append(passages, passage{text: text, Citation: Citation{
			Index: len(passages) + 1, DocumentID: string(item.Document.ID), DocumentName: item.Document.Name,
			VersionID: item.Passage.VersionID, PageStart: item.Passage.PageStart, PageEnd: item.Passage.PageEnd,
			Snippet: firstNonEmpty(item.Snippet, snippetOf(text)),
		}})
		if len(passages) >= limit {
			break
		}
	}
	return passages, nil
}

// folderScope returns the folder and its descendants, or nil for no limit.
// Documents in My files have an empty folder id and are out of scope here.
func (s *AIService) folderScope(ctx context.Context, userID, folderID string) (map[string]bool, error) {
	if folderID == "" {
		return nil, nil
	}
	access := NewAccessService(s.db)
	if err := access.RequireFolder(ctx, userID, "view", folderID); err != nil {
		return nil, ErrForbidden
	}
	folders, err := queryRows[database.Folder](ctx, s.db, "SELECT * FROM folders WHERE org_id = $org AND $folder INSIDE ancestor_ids", map[string]any{"org": orgOf(ctx), "folder": folderID})
	if err != nil {
		return nil, err
	}
	scope := map[string]bool{folderID: true}
	for _, folder := range folders {
		scope[string(folder.ID)] = true
	}
	return scope, nil
}

// window returns the part of a document's text around the first query term.
func (s *AIService) window(ctx context.Context, doc *database.Document, query string) string {
	extractions, err := queryRows[database.DocumentExtraction](ctx, s.db, "SELECT * FROM document_extractions WHERE document_id = $id LIMIT 1", map[string]any{"id": string(doc.ID)})
	if err != nil || len(extractions) == 0 {
		return ""
	}
	text := extractions[0].Text
	lower := strings.ToLower(text)
	at := -1
	for _, word := range strings.Fields(strings.ToLower(query)) {
		if len(word) < 3 {
			continue
		}
		if i := strings.Index(lower, word); i >= 0 {
			at = i
			break
		}
	}
	start := max(0, at-maxPassageChars/3)
	if at < 0 {
		start = 0
	}
	end := min(len(text), start+maxPassageChars)
	return text[start:end]
}

func snippetOf(text string) string {
	if len(text) <= 220 {
		return text
	}
	return text[:220] + "…"
}

// record keeps a history of assistant use for audit.
func (s *AIService) record(ctx context.Context, userID, kind string, in AnswerInput, result map[string]any) {
	now := time.Now().UTC()
	_, _ = surrealdb.Create[database.AIJob](ctx, s.db, "ai_jobs", map[string]any{
		"user_id": userID, "kind": kind, "status": "completed", "document_ids": nonNil(in.DocumentIDs),
		"input": map[string]any{"question": in.Question, "folder_id": in.FolderID}, "result": result,
		"created_at": now, "completed_at": now,
	})
}
