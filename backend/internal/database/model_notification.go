package database

import "time"

// Notification tells one person something happened that needs their
// attention: a share, a mention, a step assigned to them.
type Notification struct {
	ID         RecordID   `json:"id,omitempty"`
	OrgID      string     `json:"org_id,omitempty"`
	UserID     string     `json:"user_id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Body       string     `json:"body,omitempty"`
	Link       string     `json:"link,omitempty"`
	ActorID    string     `json:"actor_id,omitempty"`
	Resource   string     `json:"resource,omitempty"`
	ResourceID string     `json:"resource_id,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

const (
	NotificationShare          = "share"
	NotificationMention        = "mention"
	NotificationAccessRequest  = "access_request"
	NotificationAccessDecision = "access_decision"
	NotificationRouteStep      = "route_step"
	NotificationRouteOutcome   = "route_outcome"
	NotificationFormApproval   = "form_approval"
	NotificationFormOutcome    = "form_outcome"
	NotificationSignature      = "signature"
	NotificationFileReceived   = "file_received"
)
