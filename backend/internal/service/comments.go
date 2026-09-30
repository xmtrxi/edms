package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

var ErrCommentNotFound = errors.New("comment not found")

const maxCommentLength = 5000

// CommentService keeps discussion threads on documents. Reading needs view
// access, writing needs comment access; everyone with the document open sees
// changes live.
type CommentService struct {
	db     *surrealdb.DB
	docs   *DocumentService
	email  *OperationsService
	appURL string
}

func NewCommentService(db *surrealdb.DB, docs *DocumentService, email *OperationsService, appURL string) *CommentService {
	return &CommentService{db: db, docs: docs, email: email, appURL: strings.TrimRight(appURL, "/")}
}

type CommentInput struct {
	Body     string   `json:"body"`
	ParentID string   `json:"parent_id"`
	Page     int      `json:"page"`
	Quote    string   `json:"quote"`
	Mentions []string `json:"mentions"`
}

type CommentView struct {
	ID         string       `json:"id"`
	DocumentID string       `json:"document_id"`
	ParentID   string       `json:"parent_id,omitempty"`
	Author     PersonView   `json:"author"`
	Body       string       `json:"body"`
	Page       int          `json:"page,omitempty"`
	Quote      string       `json:"quote,omitempty"`
	Mentions   []PersonView `json:"mentions,omitempty"`
	Edited     bool         `json:"edited"`
	Resolved   bool         `json:"resolved"`
	ResolvedBy *PersonView  `json:"resolved_by,omitempty"`
	ResolvedAt time.Time    `json:"resolved_at,omitzero"`
	CreatedAt  time.Time    `json:"created_at"`
	CanEdit    bool         `json:"can_edit"`
	CanResolve bool         `json:"can_resolve"`
}

type PersonView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// List returns every comment on a document, oldest first. Replies carry
// their thread's ID in parent_id.
func (s *CommentService) List(ctx context.Context, userID, documentID string) ([]CommentView, error) {
	doc, err := s.docs.GetFor(ctx, userID, "view", documentID)
	if err != nil {
		return nil, err
	}
	comments, err := queryRows[database.Comment](ctx, s.db, "SELECT * FROM comments WHERE document_id = $doc ORDER BY created_at", map[string]any{"doc": string(doc.ID)})
	if err != nil {
		return nil, err
	}
	canModerate, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
	people := newPeople(s.db)
	views := make([]CommentView, 0, len(comments))
	for i := range comments {
		views = append(views, s.view(ctx, people, &comments[i], userID, canModerate))
	}
	return views, nil
}

// Create adds a comment or a reply and emails anyone mentioned.
func (s *CommentService) Create(ctx context.Context, userID, documentID string, in CommentInput) (*CommentView, error) {
	doc, err := s.docs.GetFor(ctx, userID, "comment", documentID)
	if err != nil {
		return nil, err
	}
	body, err := cleanCommentBody(in.Body)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record := map[string]any{
		"org_id": firstNonEmpty(doc.OrgID, orgOf(ctx)), "document_id": string(doc.ID), "author_id": userID,
		"body": body, "created_at": now, "updated_at": now,
	}
	if in.ParentID != "" {
		parent, err := s.get(ctx, in.ParentID)
		if err != nil || parent.DocumentID != string(doc.ID) {
			return nil, ErrCommentNotFound
		}
		// Threads are one level deep: a reply to a reply joins the thread.
		record["parent_id"] = firstNonEmpty(parent.ParentID, string(parent.ID))
	} else {
		if in.Page > 0 {
			record["page"] = in.Page
		}
		if quote := strings.TrimSpace(in.Quote); quote != "" {
			record["quote"] = truncate(quote, 500)
		}
	}
	mentions := s.validMentions(ctx, doc, userID, in.Mentions)
	if len(mentions) > 0 {
		record["mentions"] = mentions
	}
	created, err := surrealdb.Create[database.Comment](ctx, s.db, "comments", record)
	if err != nil {
		return nil, err
	}
	canModerate, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
	view := s.view(ctx, newPeople(s.db), created, userID, canModerate)
	_ = writeAudit(ctx, s.db, userID, "comment.created", "document", string(doc.ID), map[string]any{"comment_id": string(created.ID)})
	publishDocument(ctx, string(doc.ID), "comment.created", view)
	s.notify(ctx, doc, &view, mentions)
	return &view, nil
}

// Update changes a comment's text. Only its author may.
func (s *CommentService) Update(ctx context.Context, userID, id, body string) (*CommentView, error) {
	comment, doc, err := s.load(ctx, userID, id, "comment")
	if err != nil {
		return nil, err
	}
	if comment.AuthorID != userID {
		return nil, ErrForbidden
	}
	clean, err := cleanCommentBody(body)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	updated, err := surrealdb.Merge[database.Comment](ctx, s.db, mustRecordID(id), map[string]any{"body": clean, "edited_at": now, "updated_at": now})
	if err != nil {
		return nil, err
	}
	canModerate, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
	view := s.view(ctx, newPeople(s.db), updated, userID, canModerate)
	publishDocument(ctx, comment.DocumentID, "comment.updated", view)
	return &view, nil
}

// Resolve closes or reopens a thread. Its author or an editor may.
func (s *CommentService) Resolve(ctx context.Context, userID, id string, resolved bool) (*CommentView, error) {
	comment, doc, err := s.load(ctx, userID, id, "comment")
	if err != nil {
		return nil, err
	}
	if comment.ParentID != "" {
		return nil, errors.New("resolve the thread, not a reply")
	}
	canModerate, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
	if comment.AuthorID != userID && !canModerate {
		return nil, ErrForbidden
	}
	query := "UPDATE $c SET resolved_at = time::now(), resolved_by = $user, updated_at = time::now()"
	if !resolved {
		query = "UPDATE $c SET resolved_at = NONE, resolved_by = NONE, updated_at = time::now()"
	}
	if _, err := surrealdb.Query[any](ctx, s.db, query, map[string]any{"c": mustRecordID(id), "user": userID}); err != nil {
		return nil, err
	}
	fresh, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	view := s.view(ctx, newPeople(s.db), fresh, userID, canModerate)
	publishDocument(ctx, comment.DocumentID, "comment.updated", view)
	return &view, nil
}

// Delete removes a comment, and its replies when it starts a thread. Its
// author or an editor may.
func (s *CommentService) Delete(ctx context.Context, userID, id string) error {
	comment, doc, err := s.load(ctx, userID, id, "view")
	if err != nil {
		return err
	}
	canModerate, _ := s.docs.access.CanDocument(ctx, userID, "edit", doc)
	if comment.AuthorID != userID && !canModerate {
		return ErrForbidden
	}
	if _, err := surrealdb.Query[any](ctx, s.db, "DELETE comments WHERE parent_id = $id; DELETE $c;", map[string]any{"id": id, "c": mustRecordID(id)}); err != nil {
		return err
	}
	publishDocument(ctx, comment.DocumentID, "comment.deleted", map[string]string{"id": id})
	return nil
}

func (s *CommentService) load(ctx context.Context, userID, id, capability string) (*database.Comment, *database.Document, error) {
	comment, err := s.get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	doc, err := s.docs.GetFor(ctx, userID, capability, comment.DocumentID)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			return nil, nil, ErrForbidden
		}
		return nil, nil, ErrCommentNotFound
	}
	return comment, doc, nil
}

func (s *CommentService) get(ctx context.Context, id string) (*database.Comment, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrCommentNotFound
	}
	comment, err := surrealdb.Select[database.Comment](ctx, s.db, rid)
	if err != nil || comment == nil || comment.ID == "" {
		return nil, ErrCommentNotFound
	}
	return comment, nil
}

// validMentions keeps people who can open the document, so a mention never
// reveals it to someone without access.
func (s *CommentService) validMentions(ctx context.Context, doc *database.Document, authorID string, ids []string) []string {
	var result []string
	seen := map[string]bool{authorID: true}
	for _, id := range ids {
		if seen[id] || len(result) >= 20 {
			continue
		}
		seen[id] = true
		if ok, err := s.docs.access.CanDocument(ctx, id, "view", doc); err == nil && ok {
			result = append(result, id)
		}
	}
	return result
}

func (s *CommentService) notify(ctx context.Context, doc *database.Document, comment *CommentView, mentions []string) {
	link := s.appURL + "/documents/" + string(doc.ID) + "#comments"
	for _, id := range mentions {
		publishUser(ctx, id, "mention", map[string]any{"document_id": string(doc.ID), "document_name": doc.Name, "comment": comment})
		if s.email == nil {
			continue
		}
		person := newPeople(s.db).get(ctx, id)
		if person.Email == "" {
			continue
		}
		body := fmt.Sprintf("%s mentioned you on %q:\n\n%s\n\nReply: %s\n", comment.Author.Name, doc.Name, comment.Body, link)
		_, _ = s.email.QueueEmail(ctx, comment.Author.ID, person.Email, comment.Author.Name+" mentioned you on "+doc.Name, body)
	}
}

func (s *CommentService) view(ctx context.Context, people *peopleCache, c *database.Comment, viewerID string, canModerate bool) CommentView {
	view := CommentView{
		ID: string(c.ID), DocumentID: c.DocumentID, ParentID: c.ParentID, Author: people.get(ctx, c.AuthorID),
		Body: c.Body, Page: c.Page, Quote: c.Quote, Edited: !c.EditedAt.IsZero(), Resolved: !c.ResolvedAt.IsZero(),
		ResolvedAt: c.ResolvedAt, CreatedAt: c.CreatedAt,
		CanEdit:    c.AuthorID == viewerID,
		CanResolve: c.ParentID == "" && (c.AuthorID == viewerID || canModerate),
	}
	if c.ResolvedBy != "" {
		person := people.get(ctx, c.ResolvedBy)
		view.ResolvedBy = &person
	}
	for _, id := range c.Mentions {
		view.Mentions = append(view.Mentions, people.get(ctx, id))
	}
	return view
}

func cleanCommentBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", errors.New("write something first")
	}
	if utf8.RuneCountInString(body) > maxCommentLength {
		return "", fmt.Errorf("comments are limited to %d characters", maxCommentLength)
	}
	return body, nil
}

// peopleCache looks up names once per request.
type peopleCache struct {
	db    *surrealdb.DB
	cache map[string]PersonView
}

func newPeople(db *surrealdb.DB) *peopleCache {
	return &peopleCache{db: db, cache: map[string]PersonView{}}
}

func (p *peopleCache) get(ctx context.Context, id string) PersonView {
	if person, ok := p.cache[id]; ok {
		return person
	}
	person := PersonView{ID: id, Name: "Someone"}
	if user, err := (&OrganizationService{db: p.db}).userByID(ctx, id); err == nil && user != nil {
		person.Name, person.Email = firstNonEmpty(user.Name, user.Email), user.Email
	}
	p.cache[id] = person
	return person
}
