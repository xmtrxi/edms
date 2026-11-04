package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var ErrRuleNotFound = errors.New("automation rule not found")

var automationTriggers = map[string]bool{
	database.TriggerDocumentCreated: true, database.TriggerDocumentLabelled: true,
	database.TriggerVersionCreated: true, database.TriggerSignatureCompleted: true,
}

// AutomationService runs an organization's rules when documents are added,
// labelled, revised or signed. Actions run with the permissions of the
// person who created the rule, so a rule can never do more than they could.
type AutomationService struct {
	db      *surrealdb.DB
	docs    *DocumentService
	sharing *SharingService
	email   *OperationsService
	appURL  string
	log     *slog.Logger
}

func NewAutomationService(cfg config.Config, db *surrealdb.DB, docs *DocumentService, email *OperationsService, logger *slog.Logger) *AutomationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AutomationService{
		db: db, docs: docs, sharing: NewSharingService(cfg, db, logger), email: email,
		appURL: strings.TrimRight(cfg.SMTP.AppURL, "/"), log: logger,
	}
}

// automation receives document events; nil turns rules off. It is set once
// at startup.
var automation *AutomationService

// UseAutomation sets the service that runs rules.
func UseAutomation(a *AutomationService) { automation = a }

type automationKey struct{}

// fireAutomation runs matching rules in the background. Changes made by a
// rule never trigger rules again, so rules can't loop.
func fireAutomation(ctx context.Context, trigger string, doc *database.Document) {
	if automation == nil || doc == nil || doc.OrgID == "" || ctx.Value(automationKey{}) != nil {
		return
	}
	detached := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(detached, 2*time.Minute)
		defer cancel()
		automation.Handle(ctx, trigger, string(doc.ID), doc.OrgID)
	}()
}

// Handle runs every enabled rule for trigger that matches the document.
func (s *AutomationService) Handle(ctx context.Context, trigger, documentID, orgID string) {
	rules, err := queryRows[database.AutomationRule](ctx, s.db,
		"SELECT * FROM automation_rules WHERE org_id = $org AND trigger = $trigger AND enabled = true ORDER BY created_at",
		map[string]any{"org": orgID, "trigger": trigger})
	if err != nil {
		s.log.Warn("load automation rules", "error", err)
		return
	}
	for i := range rules {
		rule := &rules[i]
		doc, err := s.document(ctx, documentID)
		if err != nil {
			return
		}
		if !s.inScope(ctx, rule, doc) || !Matches(rule.Conditions, doc) {
			continue
		}
		status, detail := "ok", ""
		if err := s.apply(ctx, rule, doc); err != nil {
			status, detail = "failed", err.Error()
		}
		s.record(ctx, rule, doc, trigger, status, detail)
	}
}

func (s *AutomationService) apply(ctx context.Context, rule *database.AutomationRule, doc *database.Document) error {
	ctx = context.WithValue(WithActor(ctx, Actor{UserID: rule.CreatedBy, OrgID: rule.OrgID}), automationKey{}, rule.ID)
	var done []string
	for _, action := range rule.Actions {
		if err := s.run(ctx, rule, doc, action); err != nil {
			if len(done) > 0 {
				return fmt.Errorf("%s failed after %s: %w", action.Type, strings.Join(done, ", "), err)
			}
			return fmt.Errorf("%s failed: %w", action.Type, err)
		}
		done = append(done, action.Type)
	}
	return nil
}

func (s *AutomationService) run(ctx context.Context, rule *database.AutomationRule, doc *database.Document, action database.AutomationAction) error {
	id := string(doc.ID)
	switch action.Type {
	case database.ActionMoveToFolder:
		folder := action.FolderID
		if doc.FolderID == folder {
			return nil
		}
		_, err := s.docs.Update(ctx, rule.CreatedBy, id, DocumentUpdate{FolderID: &folder})
		return err
	case database.ActionSetSensitivity:
		if doc.Sensitivity == action.Sensitivity {
			return nil
		}
		_, err := s.sharing.UpdateSettings(ctx, rule.CreatedBy, id, nil, action.Sensitivity)
		return err
	case database.ActionGrantAccess:
		shareable, err := s.docs.GetFor(ctx, rule.CreatedBy, "share", id)
		if err != nil {
			return err
		}
		_, err = upsertDocumentGrant(ctx, s.db, shareable, rule.CreatedBy, GrantInput{PrincipalType: action.PrincipalType, PrincipalID: action.PrincipalID, Role: action.Role})
		if err == nil {
			_ = writeAudit(ctx, s.db, rule.CreatedBy, "automation.granted", "document", id, map[string]any{"rule_id": string(rule.ID), "principal": action.PrincipalType + ":" + action.PrincipalID, "role": action.Role})
		}
		return err
	case database.ActionNotify:
		if s.email == nil {
			return nil
		}
		body := fmt.Sprintf("The rule %q matched %q.\n\n", rule.Name, doc.Name)
		if action.Message != "" {
			body = action.Message + "\n\n"
		}
		body += "Open it: " + s.appURL + "/documents/" + id + "\n"
		for _, to := range action.Emails {
			if _, err := s.email.QueueEmail(ctx, rule.CreatedBy, to, rule.Name+": "+doc.Name, body); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown action %q", action.Type)
}

// inScope reports whether the document sits in the rule's folder or below it.
func (s *AutomationService) inScope(ctx context.Context, rule *database.AutomationRule, doc *database.Document) bool {
	if rule.FolderID == "" || doc.FolderID == rule.FolderID {
		return true
	}
	if doc.FolderID == "" {
		return false
	}
	rid, err := database.ParseRecordID(doc.FolderID)
	if err != nil {
		return false
	}
	folder, err := surrealdb.Select[database.Folder](ctx, s.db, rid)
	if err != nil || folder == nil {
		return false
	}
	for _, ancestor := range folder.AncestorIDs {
		if ancestor == rule.FolderID {
			return true
		}
	}
	return false
}

// Matches reports whether a document satisfies every condition.
func Matches(c database.AutomationCondition, doc *database.Document) bool {
	name := strings.ToLower(doc.Name)
	if c.NameContains != "" && !strings.Contains(name, strings.ToLower(c.NameContains)) {
		return false
	}
	if len(c.Extensions) > 0 {
		ext := strings.TrimPrefix(path.Ext(name), ".")
		found := false
		for _, want := range c.Extensions {
			if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(want), "."), ext) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if c.Sensitivity != "" && firstNonEmpty(doc.Sensitivity, database.SensitivityInternal) != c.Sensitivity {
		return false
	}
	if c.Classification != "" {
		raw, _ := json.Marshal(doc.Metadata["ai_classification"])
		if doc.Metadata["ai_classification"] == nil || !strings.Contains(strings.ToLower(string(raw)), strings.ToLower(c.Classification)) {
			return false
		}
	}
	if c.SensitiveData && !hasStrongIdentifiers(doc.Metadata) {
		return false
	}
	return true
}

func hasStrongIdentifiers(metadata map[string]any) bool {
	found, _ := metadata["detected_pii"].(map[string]any)
	for _, kind := range strongIdentifiers {
		// Counts come back from the database as whichever number type the
		// driver chose.
		if count, err := strconv.ParseFloat(fmt.Sprint(found[kind]), 64); err == nil && count > 0 {
			return true
		}
	}
	return false
}

func (s *AutomationService) record(ctx context.Context, rule *database.AutomationRule, doc *database.Document, trigger, status, detail string) {
	row := map[string]any{
		"org_id": rule.OrgID, "rule_id": string(rule.ID), "document_id": string(doc.ID), "trigger": trigger,
		"status": status, "created_at": time.Now().UTC(),
	}
	if detail != "" {
		row["detail"] = truncate(detail, 500)
	}
	if _, err := surrealdb.Create[database.AutomationRun](ctx, s.db, "automation_runs", row); err != nil {
		s.log.Warn("record automation run", "error", err)
	}
	_, _ = surrealdb.Query[any](ctx, s.db, "UPDATE $r SET run_count += 1, last_run_at = time::now()", map[string]any{"r": mustRecordID(string(rule.ID))})
	if status == "failed" {
		s.log.Warn("automation rule failed", "rule_id", string(rule.ID), "document_id", string(doc.ID), "error", detail)
	}
}

func (s *AutomationService) document(ctx context.Context, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrDocumentNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" || doc.Status == database.DocumentStatusTrashed {
		return nil, ErrDocumentNotFound
	}
	return doc, nil
}

// ── Managing rules ──────────────────────────────────────────────────────────

type RuleInput struct {
	Name       string                       `json:"name"`
	Enabled    *bool                        `json:"enabled"`
	Trigger    string                       `json:"trigger"`
	FolderID   string                       `json:"folder_id"`
	Conditions database.AutomationCondition `json:"conditions"`
	Actions    []database.AutomationAction  `json:"actions"`
}

func (s *AutomationService) Rules(ctx context.Context) ([]database.AutomationRule, error) {
	return queryRows[database.AutomationRule](ctx, s.db, "SELECT * FROM automation_rules WHERE org_id = $org ORDER BY created_at", map[string]any{"org": orgOf(ctx)})
}

func (s *AutomationService) CreateRule(ctx context.Context, userID string, in RuleInput) (*database.AutomationRule, error) {
	record, err := s.validate(ctx, in)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record["org_id"], record["created_by"], record["run_count"] = orgOf(ctx), userID, 0
	record["created_at"], record["updated_at"] = now, now
	if _, ok := record["enabled"]; !ok {
		record["enabled"] = true
	}
	rule, err := surrealdb.Create[database.AutomationRule](ctx, s.db, "automation_rules", record)
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "automation.created", "automation_rule", string(rule.ID), map[string]any{"name": rule.Name})
	}
	return rule, err
}

// UpdateRule replaces a rule's definition. Its actions then run with the
// permissions of whoever saved it last.
func (s *AutomationService) UpdateRule(ctx context.Context, userID, id string, in RuleInput) (*database.AutomationRule, error) {
	if _, err := s.rule(ctx, id); err != nil {
		return nil, err
	}
	record, err := s.validate(ctx, in)
	if err != nil {
		return nil, err
	}
	record["created_by"], record["updated_at"] = userID, time.Now().UTC()
	query := "UPDATE $r MERGE $changes"
	if in.FolderID == "" {
		query += "; UPDATE $r SET folder_id = NONE"
	}
	if _, err := surrealdb.Query[any](ctx, s.db, query, map[string]any{"r": mustRecordID(id), "changes": record}); err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, userID, "automation.updated", "automation_rule", id, nil)
	return s.rule(ctx, id)
}

func (s *AutomationService) DeleteRule(ctx context.Context, userID, id string) error {
	if _, err := s.rule(ctx, id); err != nil {
		return err
	}
	if _, err := surrealdb.Query[any](ctx, s.db, "DELETE automation_runs WHERE rule_id = $id; DELETE $r;", map[string]any{"id": id, "r": mustRecordID(id)}); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.db, userID, "automation.deleted", "automation_rule", id, nil)
	return nil
}

// Runs returns a rule's most recent runs.
func (s *AutomationService) Runs(ctx context.Context, id string) ([]database.AutomationRun, error) {
	if _, err := s.rule(ctx, id); err != nil {
		return nil, err
	}
	return queryRows[database.AutomationRun](ctx, s.db, "SELECT * FROM automation_runs WHERE rule_id = $id ORDER BY created_at DESC LIMIT 100", map[string]any{"id": id})
}

func (s *AutomationService) rule(ctx context.Context, id string) (*database.AutomationRule, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrRuleNotFound
	}
	rule, err := surrealdb.Select[database.AutomationRule](ctx, s.db, rid)
	if err != nil || rule == nil || rule.ID == "" || rule.OrgID != orgOf(ctx) {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

func (s *AutomationService) validate(ctx context.Context, in RuleInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("name the rule")
	}
	if !automationTriggers[in.Trigger] {
		return nil, fmt.Errorf("unknown trigger %q", in.Trigger)
	}
	if in.FolderID != "" {
		if err := s.orgFolder(ctx, in.FolderID); err != nil {
			return nil, err
		}
	}
	if len(in.Actions) == 0 || len(in.Actions) > 10 {
		return nil, errors.New("add between 1 and 10 actions")
	}
	c := in.Conditions
	if c.Sensitivity != "" && !database.IsSensitivity(c.Sensitivity) {
		return nil, fmt.Errorf("unknown sensitivity %q", c.Sensitivity)
	}
	for i := range in.Actions {
		a := &in.Actions[i]
		switch a.Type {
		case database.ActionMoveToFolder:
			if a.FolderID == "" {
				return nil, errors.New("choose the folder to move documents to")
			}
			if err := s.orgFolder(ctx, a.FolderID); err != nil {
				return nil, err
			}
			if a.FolderID == in.FolderID && in.FolderID != "" {
				return nil, errors.New("a rule can't move documents into the folder it watches")
			}
		case database.ActionSetSensitivity:
			if !database.IsSensitivity(a.Sensitivity) {
				return nil, fmt.Errorf("unknown sensitivity %q", a.Sensitivity)
			}
		case database.ActionGrantAccess:
			if _, ok := roleCapabilities[a.Role]; !ok {
				return nil, fmt.Errorf("unknown role %q", a.Role)
			}
			normalized, err := normalizeGrant(ctx, s.db, orgOf(ctx), GrantInput{PrincipalType: a.PrincipalType, PrincipalID: a.PrincipalID, Role: a.Role}, false)
			if err != nil {
				return nil, err
			}
			a.PrincipalType, a.PrincipalID = normalized.PrincipalType, normalized.PrincipalID
			a.PrincipalName = truncate(strings.TrimSpace(a.PrincipalName), 120)
		case database.ActionNotify:
			if len(a.Emails) == 0 || len(a.Emails) > 20 {
				return nil, errors.New("add between 1 and 20 email addresses to notify")
			}
			for j, email := range a.Emails {
				addr, err := mail.ParseAddress(strings.TrimSpace(email))
				if err != nil {
					return nil, fmt.Errorf("%q is not a valid email address", email)
				}
				a.Emails[j] = strings.ToLower(addr.Address)
			}
			a.Message = truncate(strings.TrimSpace(a.Message), 1000)
		default:
			return nil, fmt.Errorf("unknown action %q", a.Type)
		}
	}
	record := map[string]any{"name": name, "trigger": in.Trigger, "conditions": in.Conditions, "actions": in.Actions}
	if in.FolderID != "" {
		record["folder_id"] = in.FolderID
	}
	if in.Enabled != nil {
		record["enabled"] = *in.Enabled
	}
	return record, nil
}

func (s *AutomationService) orgFolder(ctx context.Context, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return ErrFolderNotFound
	}
	folder, err := surrealdb.Select[database.Folder](ctx, s.db, rid)
	if err != nil || folder == nil || folder.ID == "" || folder.OrgID != orgOf(ctx) {
		return ErrFolderNotFound
	}
	return nil
}
