package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/xmtrxi/edms/internal/database"
)

type FormTemplateInput struct {
	DepartmentID string         `json:"department_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Status       string         `json:"status"`
	Definition   map[string]any `json:"definition"`
}
type FormSubmissionInput struct {
	DepartmentID string         `json:"department_id"`
	Values       map[string]any `json:"values"`
	DocumentIDs  []string       `json:"document_ids"`
}
type ApprovalDecisionInput struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}
type FormSubmissionDetail struct {
	Submission database.FormSubmission           `json:"submission"`
	Steps      []database.FormApprovalStep       `json:"steps"`
	Assignees  []database.FormApprovalAssignee   `json:"assignees"`
	Comments   []database.FormComment            `json:"comments"`
	Documents  []database.FormSubmissionDocument `json:"documents"`
}
type FormsService struct{ db *surrealdb.DB }

func NewFormsService(db *surrealdb.DB) *FormsService { return &FormsService{db: db} }
func (s *FormsService) Templates(ctx context.Context) ([]database.FormTemplate, error) {
	r, e := surrealdb.Query[[]database.FormTemplate](ctx, s.db, "SELECT * FROM form_templates WHERE org_id = $org AND status != 'archived' ORDER BY updated_at DESC", map[string]any{"org": orgOf(ctx)})
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *FormsService) CreateTemplate(ctx context.Context, actor string, in FormTemplateInput) (*database.FormTemplate, error) {
	if strings.TrimSpace(in.DepartmentID) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("department and name are required")
	}
	now := time.Now().UTC()
	status := in.Status
	if status == "" {
		status = "draft"
	}
	return surrealdb.Create[database.FormTemplate](ctx, s.db, "form_templates", &database.FormTemplate{OrgID: orgOf(ctx), DepartmentID: in.DepartmentID, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), Status: status, Version: 1, Definition: in.Definition, CreatedBy: actor, CreatedAt: now, UpdatedAt: now})
}
func (s *FormsService) UpdateTemplate(ctx context.Context, actor, id string, in FormTemplateInput) (*database.FormTemplate, error) {
	template, err := s.ownedTemplate(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if template.Status != "draft" {
		return nil, fmt.Errorf("only draft forms can be edited")
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.DepartmentID) == "" {
		return nil, fmt.Errorf("department and name are required")
	}
	return surrealdb.Merge[database.FormTemplate](ctx, s.db, mustRecordID(id), map[string]any{"department_id": in.DepartmentID, "name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "definition": in.Definition, "version": template.Version + 1, "updated_at": time.Now().UTC()})
}
func (s *FormsService) SetTemplateStatus(ctx context.Context, actor, id, status string) (*database.FormTemplate, error) {
	template, err := s.ownedTemplate(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if status == "published" {
		if err = validateTemplateDefinition(template.Definition); err != nil {
			return nil, err
		}
	} else if status != "archived" {
		return nil, fmt.Errorf("invalid form status")
	}
	item, err := surrealdb.Merge[database.FormTemplate](ctx, s.db, mustRecordID(id), map[string]any{"status": status, "updated_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "form."+status, "form_template", id, nil)
	}
	return item, err
}
func (s *FormsService) Submit(ctx context.Context, actor, templateID string, in FormSubmissionInput) (*database.FormSubmission, error) {
	rid, e := database.ParseRecordID(templateID)
	if e != nil {
		return nil, fmt.Errorf("form not found")
	}
	template, e := surrealdb.Select[database.FormTemplate](ctx, s.db, rid)
	if e != nil || template == nil || template.OrgID != orgOf(ctx) {
		return nil, fmt.Errorf("form not found")
	}
	if template.Status != "published" {
		return nil, fmt.Errorf("form is not published")
	}
	if !s.memberOf(ctx, actor, in.DepartmentID) {
		return nil, ErrAccessDenied
	}
	if !allowedDepartment(template.Definition, in.DepartmentID) {
		return nil, ErrAccessDenied
	}
	if e := validateFormValues(template.Definition, in.Values); e != nil {
		return nil, e
	}
	return s.createSubmission(ctx, actor, templateID, in.DepartmentID, in.Values, in.DocumentIDs, template)
}

func (s *FormsService) SubmitVisitor(ctx context.Context, visitorID, templateID, departmentID string, values map[string]any) (*database.FormSubmission, error) {
	rid, err := database.ParseRecordID(templateID)
	if err != nil {
		return nil, fmt.Errorf("form not found")
	}
	template, err := surrealdb.Select[database.FormTemplate](ctx, s.db, rid)
	if err != nil || template == nil || template.Status != "published" || template.DepartmentID != departmentID {
		return nil, fmt.Errorf("form is not available")
	}
	if err = validateFormValues(template.Definition, values); err != nil {
		return nil, err
	}
	return s.createSubmission(ctx, "visitor:"+visitorID, templateID, departmentID, values, nil, template)
}

func (s *FormsService) createSubmission(ctx context.Context, actor, templateID, departmentID string, values map[string]any, documentIDs []string, template *database.FormTemplate) (*database.FormSubmission, error) {
	now := time.Now().UTC()
	submission, e := surrealdb.Create[database.FormSubmission](ctx, s.db, "form_submissions", &database.FormSubmission{OrgID: template.OrgID, TemplateID: templateID, TemplateVersion: template.Version, DepartmentID: departmentID, SubmittedBy: actor, Status: "awaiting_approval", Values: values, Definition: template.Definition, CreatedAt: now, UpdatedAt: now})
	if e != nil {
		return nil, e
	}
	if e = s.createSteps(ctx, string(submission.ID), template.Definition); e != nil {
		return nil, e
	}
	for _, documentID := range documentIDs {
		if strings.TrimSpace(documentID) != "" {
			if err := s.validateDocument(ctx, actor, documentID); err != nil {
				return nil, err
			}
			if _, e := surrealdb.Create[database.FormSubmissionDocument](ctx, s.db, "form_submission_documents", &database.FormSubmissionDocument{SubmissionID: string(submission.ID), DocumentID: documentID, CreatedAt: now}); e != nil {
				return nil, e
			}
		}
	}
	_ = writeAudit(ctx, s.db, actor, "form.submitted", "form_submission", string(submission.ID), map[string]any{"template_id": templateID})
	return submission, nil
}
func (s *FormsService) Submission(ctx context.Context, actor, id string) (*FormSubmissionDetail, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("submission not found")
	}
	submission, err := surrealdb.Select[database.FormSubmission](ctx, s.db, rid)
	if err != nil || submission == nil {
		return nil, fmt.Errorf("submission not found")
	}
	if submission.SubmittedBy != actor && !s.canReview(ctx, actor, id) {
		return nil, ErrAccessDenied
	}
	steps, err := queryRows[database.FormApprovalStep](ctx, s.db, "SELECT * FROM form_approval_steps WHERE submission_id = $id ORDER BY sequence", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	comments, err := s.Comments(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	documents, err := queryRows[database.FormSubmissionDocument](ctx, s.db, "SELECT * FROM form_submission_documents WHERE submission_id = $id ORDER BY created_at", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	assignees := []database.FormApprovalAssignee{}
	for i := range steps {
		rows, queryErr := queryRows[database.FormApprovalAssignee](ctx, s.db, "SELECT * FROM form_approval_assignees WHERE step_id = $id", map[string]any{"id": string(steps[i].ID)})
		if queryErr != nil {
			return nil, queryErr
		}
		assignees = append(assignees, rows...)
	}
	return &FormSubmissionDetail{Submission: *submission, Steps: steps, Assignees: assignees, Comments: comments, Documents: documents}, nil
}
func (s *FormsService) Pending(ctx context.Context, actor string) ([]database.FormSubmission, error) {
	assignees, err := queryRows[database.FormApprovalAssignee](ctx, s.db, "SELECT * FROM form_approval_assignees WHERE user_id = $actor AND status = 'pending'", map[string]any{"actor": actor})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := []database.FormSubmission{}
	for i := range assignees {
		stepID, parseErr := database.ParseRecordID(assignees[i].StepID)
		if parseErr != nil {
			continue
		}
		step, selectErr := surrealdb.Select[database.FormApprovalStep](ctx, s.db, stepID)
		if selectErr != nil || step == nil || step.Status != "active" || seen[step.SubmissionID] {
			continue
		}
		submissionID, parseErr := database.ParseRecordID(step.SubmissionID)
		if parseErr != nil {
			continue
		}
		submission, selectErr := surrealdb.Select[database.FormSubmission](ctx, s.db, submissionID)
		if selectErr == nil && submission != nil {
			items = append(items, *submission)
			seen[step.SubmissionID] = true
		}
	}
	return items, nil
}
func (s *FormsService) Comments(ctx context.Context, actor, submissionID string) ([]database.FormComment, error) {
	rid, err := database.ParseRecordID(submissionID)
	if err != nil {
		return nil, fmt.Errorf("submission not found")
	}
	submission, err := surrealdb.Select[database.FormSubmission](ctx, s.db, rid)
	if err != nil || submission == nil {
		return nil, fmt.Errorf("submission not found")
	}
	if submission.SubmittedBy != actor && !s.canReview(ctx, actor, submissionID) {
		return nil, ErrAccessDenied
	}
	return queryRows[database.FormComment](ctx, s.db, "SELECT * FROM form_comments WHERE submission_id = $id ORDER BY created_at", map[string]any{"id": submissionID})
}
func (s *FormsService) Submissions(ctx context.Context, actor string) ([]database.FormSubmission, error) {
	r, e := surrealdb.Query[[]database.FormSubmission](ctx, s.db, "SELECT * FROM form_submissions WHERE org_id = $org AND submitted_by = $actor ORDER BY created_at DESC", map[string]any{"actor": actor, "org": orgOf(ctx)})
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *FormsService) AddComment(ctx context.Context, actor, submissionID, body string) (*database.FormComment, error) {
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("comment is required")
	}
	if _, err := s.Submission(ctx, actor, submissionID); err != nil {
		return nil, err
	}
	return surrealdb.Create[database.FormComment](ctx, s.db, "form_comments", &database.FormComment{SubmissionID: submissionID, AuthorID: actor, Body: strings.TrimSpace(body), CreatedAt: time.Now().UTC()})
}
func (s *FormsService) Decide(ctx context.Context, actor, submissionID string, in ApprovalDecisionInput) error {
	if in.Action != "approve" && in.Action != "reject" && in.Action != "return" {
		return fmt.Errorf("invalid approval action")
	}
	if in.Action == "return" && strings.TrimSpace(in.Comment) == "" {
		return fmt.Errorf("a return comment is required")
	}
	steps, e := surrealdb.Query[[]database.FormApprovalStep](ctx, s.db, "SELECT * FROM form_approval_steps WHERE submission_id = $submission AND status = 'active' LIMIT 1", map[string]any{"submission": submissionID})
	if e != nil || steps == nil || len(*steps) == 0 || len((*steps)[0].Result) == 0 {
		return fmt.Errorf("no active approval step")
	}
	step := (*steps)[0].Result[0]
	assignees, e := surrealdb.Query[[]database.FormApprovalAssignee](ctx, s.db, "SELECT * FROM form_approval_assignees WHERE step_id = $step AND user_id = $user", map[string]any{"step": string(step.ID), "user": actor})
	if e != nil || assignees == nil || len(*assignees) == 0 || len((*assignees)[0].Result) == 0 {
		return ErrAccessDenied
	}
	now := time.Now().UTC()
	if _, e = surrealdb.Merge[database.FormApprovalAssignee](ctx, s.db, mustRecordID(string((*assignees)[0].Result[0].ID)), map[string]any{"status": in.Action, "decided_at": now}); e != nil {
		return e
	}
	if in.Action == "reject" || in.Action == "return" {
		status := "rejected"
		if in.Action == "return" {
			status = "returned"
		}
		_, e = surrealdb.Merge[database.FormSubmission](ctx, s.db, mustRecordID(submissionID), map[string]any{"status": status, "updated_at": now})
		return e
	}
	complete := step.CompletionRule == "any_one"
	if !complete {
		pending, e := surrealdb.Query[[]database.FormApprovalAssignee](ctx, s.db, "SELECT * FROM form_approval_assignees WHERE step_id = $step AND status = 'pending'", map[string]any{"step": string(step.ID)})
		complete = e == nil && (pending == nil || len(*pending) == 0 || len((*pending)[0].Result) == 0)
	}
	if !complete {
		return nil
	}
	_, e = surrealdb.Merge[database.FormApprovalStep](ctx, s.db, mustRecordID(string(step.ID)), map[string]any{"status": "approved", "completed_at": now})
	if e != nil {
		return e
	}
	next, e := surrealdb.Query[[]database.FormApprovalStep](ctx, s.db, "SELECT * FROM form_approval_steps WHERE submission_id = $submission AND sequence > $sequence ORDER BY sequence LIMIT 1", map[string]any{"submission": submissionID, "sequence": step.Sequence})
	if e != nil {
		return e
	}
	if next == nil || len(*next) == 0 || len((*next)[0].Result) == 0 {
		_, e = surrealdb.Merge[database.FormSubmission](ctx, s.db, mustRecordID(submissionID), map[string]any{"status": "approved", "updated_at": now})
		return e
	}
	_, e = surrealdb.Merge[database.FormApprovalStep](ctx, s.db, mustRecordID(string((*next)[0].Result[0].ID)), map[string]any{"status": "active"})
	return e
}
func (s *FormsService) createSteps(ctx context.Context, sid string, definition map[string]any) error {
	raw, _ := definition["approval_steps"].([]any)
	for i, item := range raw {
		step, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := step["completion_rule"].(string)
		if rule == "" {
			rule = "any_one"
		}
		status := "pending"
		if i == 0 {
			status = "active"
		}
		created, e := surrealdb.Create[database.FormApprovalStep](ctx, s.db, "form_approval_steps", &database.FormApprovalStep{SubmissionID: sid, Sequence: i, Name: stringValue(step["name"], fmt.Sprintf("Approval %d", i+1)), CompletionRule: rule, Status: status, CreatedAt: time.Now().UTC()})
		if e != nil {
			return e
		}
		for _, uid := range stringSlice(step["user_ids"]) {
			if _, e = surrealdb.Create[database.FormApprovalAssignee](ctx, s.db, "form_approval_assignees", &database.FormApprovalAssignee{StepID: string(created.ID), UserID: uid, Status: "pending"}); e != nil {
				return e
			}
		}
		for _, did := range stringSlice(step["department_ids"]) {
			members, e := s.Members(ctx, did)
			if e != nil {
				return e
			}
			for _, m := range members {
				if _, e = surrealdb.Create[database.FormApprovalAssignee](ctx, s.db, "form_approval_assignees", &database.FormApprovalAssignee{StepID: string(created.ID), UserID: m.UserID, DepartmentID: did, Status: "pending"}); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func (s *FormsService) Members(ctx context.Context, did string) ([]database.DepartmentMember, error) {
	r, e := surrealdb.Query[[]database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE department_id = $department", map[string]any{"department": did})
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *FormsService) memberOf(ctx context.Context, uid, did string) bool {
	r, e := surrealdb.Query[[]database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE department_id = $department AND user_id = $user LIMIT 1", map[string]any{"department": did, "user": uid})
	return e == nil && r != nil && len(*r) > 0 && len((*r)[0].Result) > 0
}
func (s *FormsService) ownedTemplate(ctx context.Context, actor, id string) (*database.FormTemplate, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("form not found")
	}
	template, err := surrealdb.Select[database.FormTemplate](ctx, s.db, rid)
	if err != nil || template == nil || template.CreatedBy != actor || template.OrgID != orgOf(ctx) {
		return nil, fmt.Errorf("form not found")
	}
	return template, nil
}
func (s *FormsService) canReview(ctx context.Context, actor, submissionID string) bool {
	steps, err := queryRows[database.FormApprovalStep](ctx, s.db, "SELECT * FROM form_approval_steps WHERE submission_id = $id", map[string]any{"id": submissionID})
	if err != nil {
		return false
	}
	for i := range steps {
		rows, queryErr := queryRows[database.FormApprovalAssignee](ctx, s.db, "SELECT * FROM form_approval_assignees WHERE step_id = $step AND user_id = $actor LIMIT 1", map[string]any{"step": string(steps[i].ID), "actor": actor})
		if queryErr == nil && len(rows) > 0 {
			return true
		}
	}
	return false
}
func (s *FormsService) validateDocument(ctx context.Context, actor, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return fmt.Errorf("attached document not found")
	}
	document, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || document == nil || document.ID == "" || document.Status == database.DocumentStatusArchived {
		return fmt.Errorf("attached document not found")
	}
	if allowed, accessErr := NewAccessService(s.db).CanDocument(ctx, actor, "view", document); accessErr != nil || !allowed {
		return fmt.Errorf("attached document not found")
	}
	return nil
}
func validateTemplateDefinition(definition map[string]any) error {
	fields, _ := definition["fields"].([]any)
	if len(fields) == 0 {
		return fmt.Errorf("add at least one form field before publishing")
	}
	steps, _ := definition["approval_steps"].([]any)
	if len(steps) == 0 {
		return fmt.Errorf("add at least one approval step before publishing")
	}
	return nil
}
func queryRows[T any](ctx context.Context, db *surrealdb.DB, statement string, vars map[string]any) ([]T, error) {
	results, err := surrealdb.Query[[]T](ctx, db, statement, vars)
	if err != nil || results == nil || len(*results) == 0 {
		return nil, err
	}
	return (*results)[0].Result, nil
}
func allowedDepartment(def map[string]any, did string) bool {
	allowed := stringSlice(def["allowed_department_ids"])
	for _, id := range allowed {
		if id == did {
			return true
		}
	}
	return false
}
func validateFormValues(def, values map[string]any) error {
	fields, _ := def["fields"].([]any)
	for _, raw := range fields {
		f, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		required, _ := f["required"].(bool)
		id, _ := f["id"].(string)
		if required && strings.TrimSpace(fmt.Sprint(values[id])) == "" {
			return fmt.Errorf("%s is required", stringValue(f["label"], id))
		}
	}
	return nil
}
func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, v := range items {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
func stringValue(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}
func mustRecordID(id string) models.RecordID { value, _ := database.ParseRecordID(id); return value }
