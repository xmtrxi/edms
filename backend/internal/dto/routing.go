package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type WorkflowStepResponse struct {
	ID                   string    `json:"id"`
	Sequence             int       `json:"sequence"`
	Name                 string    `json:"name"`
	ActionType           string    `json:"action_type"`
	AssignmentType       string    `json:"assignment_type"`
	AssignedUserID       string    `json:"assigned_user_id,omitempty"`
	AssignedDepartmentID string    `json:"assigned_department_id,omitempty"`
	ExternalEmail        string    `json:"external_email,omitempty"`
	Status               string    `json:"status"`
	RequireComment       bool      `json:"require_comment"`
	RequireNewVersion    bool      `json:"require_new_version"`
	SigningRequestID     string    `json:"signing_request_id,omitempty"`
	ActivatedAt          time.Time `json:"activated_at,omitempty"`
	CompletedAt          time.Time `json:"completed_at,omitempty"`
}

type WorkflowResponse struct {
	ID                string                 `json:"id"`
	DocumentID        string                 `json:"document_id"`
	StartedVersionID  string                 `json:"started_version_id"`
	CurrentVersionID  string                 `json:"current_version_id"`
	Title             string                 `json:"title"`
	Status            string                 `json:"status"`
	Priority          string                 `json:"priority"`
	CurrentStep       int                    `json:"current_step"`
	ReturnedVersionID string                 `json:"returned_version_id,omitempty"`
	DueAt             time.Time              `json:"due_at,omitempty"`
	CompletedAt       time.Time              `json:"completed_at,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	Steps             []WorkflowStepResponse `json:"steps"`
}

func NewWorkflowResponse(workflow *database.WorkflowRequest, steps []database.WorkflowStep) WorkflowResponse {
	response := WorkflowResponse{ID: string(workflow.ID), DocumentID: workflow.DocumentID,
		StartedVersionID: workflow.StartedVersionID, CurrentVersionID: workflow.CurrentVersionID,
		Title: workflow.Title, Status: workflow.Status, Priority: workflow.Priority,
		CurrentStep: workflow.CurrentStep, ReturnedVersionID: workflow.ReturnedVersionID,
		DueAt: workflow.DueAt, CompletedAt: workflow.CompletedAt, CreatedAt: workflow.CreatedAt,
		UpdatedAt: workflow.UpdatedAt, Steps: make([]WorkflowStepResponse, 0, len(steps))}
	for _, step := range steps {
		response.Steps = append(response.Steps, WorkflowStepResponse{ID: string(step.ID), Sequence: step.Sequence,
			Name: step.Name, ActionType: step.ActionType, AssignmentType: step.AssignmentType,
			AssignedUserID: step.AssignedUserID, AssignedDepartmentID: step.AssignedDepartmentID,
			ExternalEmail: step.ExternalEmail, Status: step.Status, RequireComment: step.RequireComment,
			RequireNewVersion: step.RequireNewVersion, SigningRequestID: step.SigningRequestID,
			ActivatedAt: step.ActivatedAt, CompletedAt: step.CompletedAt})
	}
	return response
}
