package database

import "time"

type Department struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	ParentID    string    `json:"parent_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DepartmentMember struct {
	ID           RecordID  `json:"id,omitempty"`
	OrgID        string    `json:"org_id"`
	DepartmentID string    `json:"department_id"`
	UserID       string    `json:"user_id"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
