package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
)

var (
	ErrWorkflowNotFound   = errors.New("workflow not found")
	ErrInvalidTransition  = errors.New("invalid workflow transition")
	ErrNewVersionRequired = errors.New("a new document version is required before resubmission")
)

type RoutingService struct {
	DB   *surrealdb.DB
	docs *DocumentService
}

func NewRoutingService(db *surrealdb.DB, docs *DocumentService) *RoutingService {
	return &RoutingService{DB: db, docs: docs}
}

type RouteStepInput struct {
	Name                 string `json:"name"`
	ActionType           string `json:"action_type"`
	AssignmentType       string `json:"assignment_type"`
	AssignedUserID       string `json:"assigned_user_id"`
	AssignedDepartmentID string `json:"assigned_department_id"`
	ExternalEmail        string `json:"external_email"`
	RequireComment       bool   `json:"require_comment"`
	RequireNewVersion    bool   `json:"require_new_version"`
}

type CreateRouteInput struct {
	Title    string           `json:"title"`
	Priority string           `json:"priority"`
	DueAt    time.Time        `json:"due_at"`
	Steps    []RouteStepInput `json:"steps"`
}
type RouteDecisionInput struct {
	Action       string `json:"action"`
	Comment      string `json:"comment"`
	ReturnToStep int    `json:"return_to_step"`
}

func (s *RoutingService) Create(ctx context.Context, ownerID, documentID string, in CreateRouteInput) (*dto.WorkflowResponse, error) {
	doc, err := s.docs.Get(ctx, ownerID, documentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" || len(in.Steps) == 0 {
		return nil, fmt.Errorf("title and at least one route step are required")
	}
	if doc.CurrentVersionID == "" {
		return nil, fmt.Errorf("document has no current version")
	}
	now := time.Now().UTC()
	priority := in.Priority
	if priority == "" {
		priority = "medium"
	}
	workflow, err := surrealdb.Create[database.WorkflowRequest](ctx, s.DB, "workflow_requests", &database.WorkflowRequest{
		DocumentID: documentID, StartedVersionID: doc.CurrentVersionID, CurrentVersionID: doc.CurrentVersionID,
		CreatedBy: ownerID, Title: strings.TrimSpace(in.Title), Status: database.WorkflowStatusActive,
		Priority: priority, CurrentStep: 0, DueAt: in.DueAt, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	steps := make([]database.WorkflowStep, 0, len(in.Steps))
	for sequence, input := range in.Steps {
		status := database.WorkflowStepPending
		activated := time.Time{}
		if sequence == 0 {
			status = database.WorkflowStepActive
			activated = now
		}
		step, err := surrealdb.Create[database.WorkflowStep](ctx, s.DB, "workflow_steps", &database.WorkflowStep{
			WorkflowRequestID: string(workflow.ID), Sequence: sequence, Name: strings.TrimSpace(input.Name),
			ActionType: input.ActionType, AssignmentType: input.AssignmentType, AssignedUserID: input.AssignedUserID,
			AssignedDepartmentID: input.AssignedDepartmentID, ExternalEmail: input.ExternalEmail, Status: status,
			RequireComment: input.RequireComment, RequireNewVersion: input.RequireNewVersion, ActivatedAt: activated, CreatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		steps = append(steps, *step)
	}
	_ = s.action(ctx, string(workflow.ID), string(steps[0].ID), ownerID, "submit", "", doc.CurrentVersionID, 0)
	_ = writeAudit(ctx, s.DB, ownerID, "route.started", "document", documentID, map[string]any{
		"workflow_id": string(workflow.ID), "document_version_id": doc.CurrentVersionID, "summary": workflow.Title,
	})
	response := dto.NewWorkflowResponse(workflow, steps)
	return &response, nil
}

func (s *RoutingService) List(ctx context.Context, ownerID, documentID string) ([]dto.WorkflowResponse, error) {
	if _, err := s.docs.Get(ctx, ownerID, documentID); err != nil {
		return nil, err
	}
	results, err := surrealdb.Query[[]database.WorkflowRequest](ctx, s.DB,
		"SELECT * FROM workflow_requests WHERE document_id = $document ORDER BY created_at DESC", map[string]any{"document": documentID})
	if err != nil {
		return nil, err
	}
	response := []dto.WorkflowResponse{}
	if results == nil || len(*results) == 0 {
		return response, nil
	}
	for i := range (*results)[0].Result {
		workflow := (*results)[0].Result[i]
		steps, err := s.steps(ctx, string(workflow.ID))
		if err != nil {
			return nil, err
		}
		response = append(response, dto.NewWorkflowResponse(&workflow, steps))
	}
	return response, nil
}

func (s *RoutingService) Decide(ctx context.Context, ownerID, documentID, workflowID string, in RouteDecisionInput) (*dto.WorkflowResponse, error) {
	doc, err := s.docs.Get(ctx, ownerID, documentID)
	if err != nil {
		return nil, err
	}
	workflow, err := s.get(ctx, documentID, workflowID)
	if err != nil {
		return nil, err
	}
	steps, err := s.steps(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if workflow.CurrentStep < 0 || workflow.CurrentStep >= len(steps) {
		return nil, ErrInvalidTransition
	}
	current := &steps[workflow.CurrentStep]
	if current.Status != database.WorkflowStepActive && workflow.Status != database.WorkflowStatusWaitingForChanges {
		return nil, ErrInvalidTransition
	}
	comment := strings.TrimSpace(in.Comment)
	if current.RequireComment && comment == "" {
		return nil, fmt.Errorf("a comment is required")
	}
	now := time.Now().UTC()
	action := strings.TrimSpace(in.Action)
	switch action {
	case "approve", "acknowledge", "sign", "file":
		current.Status = database.WorkflowStepCompleted
		if action == "approve" {
			current.Status = database.WorkflowStepApproved
		}
		current.CompletedAt = now
		if err := s.mergeStep(ctx, current, map[string]any{"status": current.Status, "completed_at": now}); err != nil {
			return nil, err
		}
		workflow.CurrentVersionID = doc.CurrentVersionID
		if workflow.CurrentStep+1 < len(steps) {
			workflow.CurrentStep++
			next := &steps[workflow.CurrentStep]
			next.Status = database.WorkflowStepActive
			next.ActivatedAt = now
			if err := s.mergeStep(ctx, next, map[string]any{"status": next.Status, "activated_at": now}); err != nil {
				return nil, err
			}
			workflow.Status = database.WorkflowStatusActive
		} else {
			workflow.Status = database.WorkflowStatusCompleted
			workflow.CompletedAt = now
		}
	case "reject":
		current.Status = database.WorkflowStepRejected
		current.CompletedAt = now
		workflow.Status = database.WorkflowStatusRejected
		workflow.CompletedAt = now
		if err := s.mergeStep(ctx, current, map[string]any{"status": current.Status, "completed_at": now}); err != nil {
			return nil, err
		}
	case "return_for_changes":
		if comment == "" {
			return nil, fmt.Errorf("a return reason is required")
		}
		if in.ReturnToStep < 0 || in.ReturnToStep >= len(steps) || in.ReturnToStep >= workflow.CurrentStep {
			return nil, fmt.Errorf("return_to_step must be an earlier step")
		}
		current.Status = database.WorkflowStepReturned
		workflow.Status = database.WorkflowStatusWaitingForChanges
		workflow.ReturnedVersionID = doc.CurrentVersionID
		workflow.CurrentStep = in.ReturnToStep
		if err := s.mergeStep(ctx, current, map[string]any{"status": current.Status, "completed_at": now}); err != nil {
			return nil, err
		}
		target := &steps[in.ReturnToStep]
		target.Status = database.WorkflowStepActive
		target.RequireNewVersion = true
		target.ActivatedAt = now
		if err := s.mergeStep(ctx, target, map[string]any{"status": target.Status, "require_new_version": true, "activated_at": now}); err != nil {
			return nil, err
		}
	case "resubmit":
		if workflow.Status != database.WorkflowStatusWaitingForChanges {
			return nil, ErrInvalidTransition
		}
		if doc.CurrentVersionID == workflow.ReturnedVersionID {
			return nil, ErrNewVersionRequired
		}
		current.Status = database.WorkflowStepCompleted
		current.CompletedAt = now
		workflow.CurrentVersionID = doc.CurrentVersionID
		if err := s.mergeStep(ctx, current, map[string]any{"status": current.Status, "completed_at": now}); err != nil {
			return nil, err
		}
		// Continue at the step immediately following the revision target.
		if workflow.CurrentStep+1 < len(steps) {
			workflow.CurrentStep++
			next := &steps[workflow.CurrentStep]
			next.Status = database.WorkflowStepActive
			next.ActivatedAt = now
			_ = s.mergeStep(ctx, next, map[string]any{"status": next.Status, "activated_at": now})
			workflow.Status = database.WorkflowStatusActive
		} else {
			workflow.Status = database.WorkflowStatusCompleted
			workflow.CompletedAt = now
		}
	default:
		return nil, ErrInvalidTransition
	}
	rid, _ := database.ParseRecordID(workflowID)
	merged, err := surrealdb.Merge[database.WorkflowRequest](ctx, s.DB, rid, map[string]any{
		"status": workflow.Status, "current_step": workflow.CurrentStep, "current_version_id": workflow.CurrentVersionID,
		"returned_version_id": workflow.ReturnedVersionID, "completed_at": workflow.CompletedAt, "updated_at": now,
	})
	if err != nil {
		return nil, err
	}
	workflow = merged
	_ = s.action(ctx, workflowID, string(current.ID), ownerID, action, comment, doc.CurrentVersionID, in.ReturnToStep)
	_ = writeAudit(ctx, s.DB, ownerID, "route."+action, "document", documentID, map[string]any{
		"workflow_id": workflowID, "step_id": string(current.ID), "document_version_id": doc.CurrentVersionID, "summary": comment,
	})
	steps, _ = s.steps(ctx, workflowID)
	response := dto.NewWorkflowResponse(workflow, steps)
	return &response, nil
}

func (s *RoutingService) get(ctx context.Context, documentID, workflowID string) (*database.WorkflowRequest, error) {
	rid, err := database.ParseRecordID(workflowID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}
	workflow, err := surrealdb.Select[database.WorkflowRequest](ctx, s.DB, rid)
	if err != nil {
		return nil, err
	}
	if workflow == nil || workflow.DocumentID != documentID {
		return nil, ErrWorkflowNotFound
	}
	return workflow, nil
}
func (s *RoutingService) steps(ctx context.Context, workflowID string) ([]database.WorkflowStep, error) {
	results, err := surrealdb.Query[[]database.WorkflowStep](ctx, s.DB, "SELECT * FROM workflow_steps WHERE workflow_request_id = $workflow ORDER BY sequence", map[string]any{"workflow": workflowID})
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) == 0 {
		return []database.WorkflowStep{}, nil
	}
	return (*results)[0].Result, nil
}
func (s *RoutingService) mergeStep(ctx context.Context, step *database.WorkflowStep, values map[string]any) error {
	rid, err := database.ParseRecordID(string(step.ID))
	if err != nil {
		return err
	}
	_, err = surrealdb.Merge[database.WorkflowStep](ctx, s.DB, rid, values)
	return err
}
func (s *RoutingService) action(ctx context.Context, workflowID, stepID, actorID, action, note, versionID string, returnTo int) error {
	_, err := surrealdb.Create[database.WorkflowAction](ctx, s.DB, "workflow_actions", &database.WorkflowAction{WorkflowRequestID: workflowID, StepID: stepID, ActorID: actorID, Action: action, Note: note, DocumentVersionID: versionID, ReturnToStep: returnTo, CreatedAt: time.Now().UTC()})
	return err
}
