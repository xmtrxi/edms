package database

import "time"

type EmailDelivery struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id,omitempty"`
	Recipient   string    `json:"recipient"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body,omitempty"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	LastError   string    `json:"last_error,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
}
