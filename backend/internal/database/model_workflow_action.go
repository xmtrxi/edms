package database

import "time"

type WorkflowAction struct {
	ID                RecordID  `json:"id,omitempty"`
	WorkflowRequestID string    `json:"workflow_request_id"`
	ActorID           string    `json:"actor_id"`
	Action            string    `json:"action"`
	Note              string    `json:"note"`
	CreatedAt         time.Time `json:"created_at"`
}
