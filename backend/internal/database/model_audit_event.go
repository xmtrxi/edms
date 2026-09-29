package database

import "time"

type AuditEvent struct {
	ID         RecordID       `json:"id,omitempty"`
	OrgID      string         `json:"org_id,omitempty"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	Resource   string         `json:"resource"`
	ResourceID string         `json:"resource_id"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}
