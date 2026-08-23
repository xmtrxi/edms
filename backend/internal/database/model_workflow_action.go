package database

import "time"

type WorkflowAction struct {
	ID                RecordID  `json:"id,omitempty"`
	WorkflowRequestID string    `json:"workflow_request_id"`
	StepID            string    `json:"step_id,omitempty"`
	ActorID           string    `json:"actor_id"`
	Action            string    `json:"action"`
	Note              string    `json:"note"`
	DocumentVersionID string    `json:"document_version_id,omitempty"`
	ReturnToStep      int       `json:"return_to_step,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
