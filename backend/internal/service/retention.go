package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/storage"
)

var ErrPolicyNotFound = errors.New("retention policy not found")

// retentionBatch caps how many documents one policy handles per run.
const retentionBatch = 500

// RetentionService applies retention policies. A policy acts with its
// creator's permissions and never touches documents on legal hold.
type RetentionService struct {
	db        *surrealdb.DB
	access    *AccessService
	workspace *WorkspaceService
	log       *slog.Logger
}

func NewRetentionService(cfg config.Config, db *surrealdb.DB, provider storage.Provider, logger *slog.Logger) *RetentionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RetentionService{db: db, access: NewAccessService(db), workspace: NewWorkspaceService(db, cfg, provider), log: logger}
}

type PolicyInput struct {
	Name        string `json:"name"`
	Enabled     *bool  `json:"enabled"`
	FolderID    string `json:"folder_id"`
	Sensitivity string `json:"sensitivity"`
	AgeBasis    string `json:"age_basis"`
	KeepDays    int    `json:"keep_days"`
	Action      string `json:"action"`
}

func (s *RetentionService) Policies(ctx context.Context) ([]database.RetentionPolicy, error) {
	return queryRows[database.RetentionPolicy](ctx, s.db, "SELECT * FROM retention_policies WHERE org_id = $org ORDER BY created_at", map[string]any{"org": orgOf(ctx)})
}

func (s *RetentionService) Create(ctx context.Context, userID string, in PolicyInput) (*database.RetentionPolicy, error) {
	record, err := s.validate(ctx, in)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record["org_id"], record["created_by"], record["last_count"] = orgOf(ctx), userID, 0
	record["created_at"], record["updated_at"] = now, now
	if _, ok := record["enabled"]; !ok {
		record["enabled"] = true
	}
	policy, err := surrealdb.Create[database.RetentionPolicy](ctx, s.db, "retention_policies", record)
	if err == nil {
		_ = writeAudit(ctx, s.db, userID, "retention.created", "retention_policy", string(policy.ID), map[string]any{"name": policy.Name, "action": policy.Action, "keep_days": policy.KeepDays})
	}
	return policy, err
}

func (s *RetentionService) Update(ctx context.Context, userID, id string, in PolicyInput) (*database.RetentionPolicy, error) {
	if _, err := s.policy(ctx, id); err != nil {
		return nil, err
	}
	record, err := s.validate(ctx, in)
	if err != nil {
		return nil, err
	}
	record["created_by"], record["updated_at"] = userID, time.Now().UTC()
	query := "UPDATE $p MERGE $changes"
	if in.FolderID == "" {
		query += "; UPDATE $p SET folder_id = NONE"
	}
	if in.Sensitivity == "" {
		query += "; UPDATE $p SET sensitivity = NONE"
	}
	if _, err := surrealdb.Query[any](ctx, s.db, query, map[string]any{"p": mustRecordID(id), "changes": record}); err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, userID, "retention.updated", "retention_policy", id, nil)
	return s.policy(ctx, id)
}

func (s *RetentionService) Delete(ctx context.Context, userID, id string) error {
	if _, err := s.policy(ctx, id); err != nil {
		return err
	}
	if _, err := surrealdb.Delete[database.RetentionPolicy](ctx, s.db, mustRecordID(id)); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.db, userID, "retention.deleted", "retention_policy", id, nil)
	return nil
}

// RetentionPreview shows what a policy would do if it ran now.
type RetentionPreview struct {
	Count  int                 `json:"count"`
	Held   int                 `json:"held"`
	Sample []database.Document `json:"sample"`
}

func (s *RetentionService) Preview(ctx context.Context, id string) (*RetentionPreview, error) {
	policy, err := s.policy(ctx, id)
	if err != nil {
		return nil, err
	}
	docs, held, err := s.candidates(ctx, policy)
	if err != nil {
		return nil, err
	}
	sample := docs
	if len(sample) > 20 {
		sample = sample[:20]
	}
	return &RetentionPreview{Count: len(docs), Held: held, Sample: sample}, nil
}

// RunNow applies one policy immediately.
func (s *RetentionService) RunNow(ctx context.Context, id string) (int, error) {
	policy, err := s.policy(ctx, id)
	if err != nil {
		return 0, err
	}
	return s.apply(ctx, policy)
}

// RunAll applies every enabled policy in every organization.
func (s *RetentionService) RunAll(ctx context.Context) (int, error) {
	policies, err := queryRows[database.RetentionPolicy](ctx, s.db, "SELECT * FROM retention_policies WHERE enabled = true", nil)
	if err != nil {
		return 0, err
	}
	total := 0
	for i := range policies {
		count, err := s.apply(ctx, &policies[i])
		if err != nil {
			s.log.Warn("retention policy failed", "policy_id", string(policies[i].ID), "error", err)
		}
		total += count
	}
	return total, nil
}

func (s *RetentionService) apply(ctx context.Context, policy *database.RetentionPolicy) (int, error) {
	ctx = WithActor(ctx, Actor{UserID: policy.CreatedBy, OrgID: policy.OrgID})
	docs, _, err := s.candidates(ctx, policy)
	if err != nil {
		return 0, err
	}
	count := 0
	now := time.Now().UTC()
	for i := range docs {
		doc := &docs[i]
		id := string(doc.ID)
		if ok, _ := s.access.CanDocument(ctx, policy.CreatedBy, "delete", doc); !ok {
			continue
		}
		var err error
		switch policy.Action {
		case database.RetentionArchive:
			_, err = surrealdb.Merge[database.Document](ctx, s.db, mustRecordID(id), map[string]any{"status": database.DocumentStatusArchived, "updated_at": now})
		case database.RetentionTrash:
			_, err = surrealdb.Merge[database.Document](ctx, s.db, mustRecordID(id), map[string]any{
				"status": database.DocumentStatusTrashed, "deleted_from_status": doc.Status, "deleted_at": now, "updated_at": now,
			})
		case database.RetentionPurge:
			err = s.workspace.PermanentlyDelete(ctx, policy.CreatedBy, id)
		}
		if err != nil {
			s.log.Warn("apply retention", "policy_id", string(policy.ID), "document_id", id, "error", err)
			continue
		}
		_ = writeAudit(ctx, s.db, policy.CreatedBy, "retention."+policy.Action, "document", id, map[string]any{"policy_id": string(policy.ID), "policy": policy.Name})
		count++
	}
	_, _ = surrealdb.Merge[database.RetentionPolicy](ctx, s.db, mustRecordID(string(policy.ID)), map[string]any{"last_run_at": now, "last_count": count})
	return count, nil
}

// candidates lists documents old enough for the policy, minus those on
// legal hold, which it counts separately.
func (s *RetentionService) candidates(ctx context.Context, policy *database.RetentionPolicy) ([]database.Document, int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -policy.KeepDays)
	vars := map[string]any{"org": policy.OrgID, "cutoff": cutoff, "limit": retentionBatch}
	where := []string{"org_id = $org"}
	switch policy.Action {
	case database.RetentionPurge:
		where = append(where, "status = 'trashed'", "deleted_at < $cutoff")
	case database.RetentionArchive:
		where = append(where, "status NOT IN ['archived', 'trashed']")
	case database.RetentionTrash:
		where = append(where, "status != 'trashed'")
	}
	if policy.Action != database.RetentionPurge {
		field := "created_at"
		if policy.AgeBasis == database.AgeFromModified {
			field = "updated_at"
		}
		where = append(where, field+" < $cutoff")
	}
	if policy.FolderID != "" {
		scope, err := s.folderScope(ctx, policy.OrgID, policy.FolderID)
		if err != nil {
			return nil, 0, err
		}
		vars["folders"] = scope
		where = append(where, "folder_id IN $folders")
	}
	if policy.Sensitivity != "" {
		vars["sensitivity"] = policy.Sensitivity
		clause := "sensitivity = $sensitivity"
		if policy.Sensitivity == database.SensitivityInternal {
			clause = "(sensitivity = $sensitivity OR sensitivity = NONE OR sensitivity = '')"
		}
		where = append(where, clause)
	}
	docs, err := queryRows[database.Document](ctx, s.db, "SELECT * FROM documents WHERE "+strings.Join(where, " AND ")+" ORDER BY created_at LIMIT $limit", vars)
	if err != nil {
		return nil, 0, err
	}
	holds, err := queryRows[database.LegalHold](ctx, s.db, "SELECT * FROM legal_holds WHERE org_id = $org AND active = true", map[string]any{"org": policy.OrgID})
	if err != nil {
		return nil, 0, err
	}
	onHold := map[string]bool{}
	for _, hold := range holds {
		for _, id := range hold.DocumentIDs {
			onHold[id] = true
		}
	}
	kept := docs[:0]
	held := 0
	for _, doc := range docs {
		if onHold[string(doc.ID)] {
			held++
			continue
		}
		kept = append(kept, doc)
	}
	return kept, held, nil
}

func (s *RetentionService) folderScope(ctx context.Context, orgID, folderID string) ([]string, error) {
	folders, err := queryRows[database.Folder](ctx, s.db, "SELECT * FROM folders WHERE org_id = $org AND $folder INSIDE ancestor_ids", map[string]any{"org": orgID, "folder": folderID})
	if err != nil {
		return nil, err
	}
	scope := []string{folderID}
	for _, folder := range folders {
		scope = append(scope, string(folder.ID))
	}
	return scope, nil
}

func (s *RetentionService) policy(ctx context.Context, id string) (*database.RetentionPolicy, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrPolicyNotFound
	}
	policy, err := surrealdb.Select[database.RetentionPolicy](ctx, s.db, rid)
	if err != nil || policy == nil || policy.ID == "" || policy.OrgID != orgOf(ctx) {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

func (s *RetentionService) validate(ctx context.Context, in PolicyInput) (map[string]any, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("name the policy")
	}
	switch in.Action {
	case database.RetentionArchive, database.RetentionTrash, database.RetentionPurge:
	default:
		return nil, fmt.Errorf("unknown action %q", in.Action)
	}
	if in.AgeBasis == "" {
		in.AgeBasis = database.AgeFromCreated
	}
	if in.AgeBasis != database.AgeFromCreated && in.AgeBasis != database.AgeFromModified {
		return nil, fmt.Errorf("unknown age basis %q", in.AgeBasis)
	}
	// Purging deletes for good; insist on a grace period in the trash.
	minimum := 1
	if in.Action == database.RetentionPurge {
		minimum = 7
	}
	if in.KeepDays < minimum || in.KeepDays > 36500 {
		return nil, fmt.Errorf("keep documents between %d and 36500 days", minimum)
	}
	if in.Sensitivity != "" && !database.IsSensitivity(in.Sensitivity) {
		return nil, fmt.Errorf("unknown sensitivity %q", in.Sensitivity)
	}
	record := map[string]any{"name": name, "age_basis": in.AgeBasis, "keep_days": in.KeepDays, "action": in.Action}
	if in.FolderID != "" {
		rid, err := database.ParseRecordID(in.FolderID)
		if err != nil {
			return nil, ErrFolderNotFound
		}
		folder, err := surrealdb.Select[database.Folder](ctx, s.db, rid)
		if err != nil || folder == nil || folder.OrgID != orgOf(ctx) {
			return nil, ErrFolderNotFound
		}
		record["folder_id"] = in.FolderID
	}
	if in.Sensitivity != "" {
		record["sensitivity"] = in.Sensitivity
	}
	if in.Enabled != nil {
		record["enabled"] = *in.Enabled
	}
	return record, nil
}
