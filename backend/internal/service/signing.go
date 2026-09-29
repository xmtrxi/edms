package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/signing"
	"github.com/xmtrxi/edms/internal/storage"
)

type SigningService struct {
	db       *surrealdb.DB
	storage  storage.Provider
	provider signing.Provider
}

func NewSigningService(db *surrealdb.DB, storageProvider storage.Provider, provider signing.Provider) *SigningService {
	return &SigningService{db: db, storage: storageProvider, provider: provider}
}

func (s *SigningService) Complete(ctx context.Context, providerDocumentID, downloadURL string) (*database.SigningRequest, error) {
	requests, err := queryRows[database.SigningRequest](ctx, s.db, "SELECT * FROM signing_requests WHERE provider_document_id = $provider_document LIMIT 1", map[string]any{"provider_document": providerDocumentID})
	if err != nil || len(requests) == 0 {
		return nil, fmt.Errorf("signing request not found")
	}
	request := &requests[0]
	if request.Status == database.SigningStatusCompleted && request.CompletedVersionID != "" {
		return request, nil
	}
	documentID, err := database.ParseRecordID(request.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("signed document not found")
	}
	document, err := surrealdb.Select[database.Document](ctx, s.db, documentID)
	if err != nil || document == nil {
		return nil, fmt.Errorf("signed document not found")
	}

	reader, err := s.provider.DownloadCompleted(ctx, providerDocumentID, downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download completed document: %w", err)
	}
	defer reader.Close()
	temp, err := os.CreateTemp("", "edms-signed-*")
	if err != nil {
		return nil, fmt.Errorf("buffer signed document: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), reader)
	if err != nil {
		return nil, fmt.Errorf("read completed document: %w", err)
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	next := document.VersionCount + 1
	if next < 2 {
		next = 2
	}
	path := VersionStoragePath(request.DocumentID, next, document.Name)
	if err := s.storage.Upload(ctx, path, temp); err != nil {
		return nil, fmt.Errorf("store completed document: %w", err)
	}
	now := time.Now().UTC()
	checksum := hex.EncodeToString(hash.Sum(nil))
	version, err := surrealdb.Create[database.DocumentVersion](ctx, s.db, "document_versions", &database.DocumentVersion{
		DocumentID: request.DocumentID, ParentVersionID: document.CurrentVersionID, Version: next,
		StorageProvider: s.storage.Name(), StoragePath: path, FileName: document.Name, MimeType: document.MimeType,
		Checksum: checksum, Size: size, CreatedBy: request.RequestedBy, Source: database.VersionSourceSignature,
		ChangeSummary: "Completed electronic signature", ChangeManifest: map[string]any{"kind": database.VersionSourceSignature, "provider_document_id": providerDocumentID},
		WorkflowID: request.WorkflowRequestID, WorkflowStepID: request.WorkflowStepID, SigningRequestID: string(request.ID), CreatedAt: now,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, path)
		return nil, err
	}
	if _, err := surrealdb.Create[database.DocumentVersionContributor](ctx, s.db, "document_version_contributors", &database.DocumentVersionContributor{VersionID: string(version.ID), UserID: request.RequestedBy, Role: "signing_requester", CreatedAt: now}); err != nil {
		return nil, err
	}
	if _, err := surrealdb.Merge[database.Document](ctx, s.db, documentID, map[string]any{
		"current_version_id": string(version.ID), "version_count": next, "storage_path": path,
		"size": size, "checksum": checksum, "status": database.DocumentStatusReady, "updated_at": now,
	}); err != nil {
		return nil, err
	}
	requestID, _ := database.ParseRecordID(string(request.ID))
	request, err = surrealdb.Merge[database.SigningRequest](ctx, s.db, requestID, map[string]any{"status": database.SigningStatusCompleted, "completed_version_id": string(version.ID), "updated_at": now})
	if err != nil {
		return nil, err
	}
	if err := s.completeWorkflowStep(ctx, request, string(version.ID), now); err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, request.RequestedBy, "signature.completed", "document", request.DocumentID, map[string]any{"signing_request_id": string(request.ID), "document_version_id": string(version.ID)})
	return request, nil
}

func (s *SigningService) completeWorkflowStep(ctx context.Context, request *database.SigningRequest, versionID string, now time.Time) error {
	if request.WorkflowRequestID == "" || request.WorkflowStepID == "" {
		return nil
	}
	stepID, err := database.ParseRecordID(request.WorkflowStepID)
	if err != nil {
		return err
	}
	if _, err := surrealdb.Merge[database.WorkflowStep](ctx, s.db, stepID, map[string]any{"status": database.WorkflowStepCompleted, "completed_at": now}); err != nil {
		return err
	}
	workflowID, err := database.ParseRecordID(request.WorkflowRequestID)
	if err != nil {
		return err
	}
	workflow, err := surrealdb.Select[database.WorkflowRequest](ctx, s.db, workflowID)
	if err != nil || workflow == nil {
		return fmt.Errorf("workflow not found")
	}
	steps, err := queryRows[database.WorkflowStep](ctx, s.db, "SELECT * FROM workflow_steps WHERE workflow_request_id = $workflow ORDER BY sequence ASC", map[string]any{"workflow": request.WorkflowRequestID})
	if err != nil {
		return err
	}
	updates := map[string]any{"current_version_id": versionID, "updated_at": now}
	if workflow.CurrentStep+1 < len(steps) {
		next := steps[workflow.CurrentStep+1]
		nextID, _ := database.ParseRecordID(string(next.ID))
		if _, err := surrealdb.Merge[database.WorkflowStep](ctx, s.db, nextID, map[string]any{"status": database.WorkflowStepActive, "activated_at": now}); err != nil {
			return err
		}
		updates["current_step"] = workflow.CurrentStep + 1
		updates["status"] = database.WorkflowStatusActive
	} else {
		updates["status"] = database.WorkflowStatusCompleted
		updates["completed_at"] = now
	}
	_, err = surrealdb.Merge[database.WorkflowRequest](ctx, s.db, workflowID, updates)
	return err
}
