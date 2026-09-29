package database

import "time"

// VisitorRequest is a short-lived, form-scoped identity for a person who does
// not have an office account. Secrets are stored only as hashes.
type VisitorRequest struct {
	ID               RecordID  `json:"id,omitempty"`
	TemplateID       string    `json:"template_id"`
	DepartmentID     string    `json:"department_id"`
	VisitorName      string    `json:"visitor_name"`
	VisitorEmail     string    `json:"visitor_email"`
	AccessCodeHash   string    `json:"access_code_hash"`
	OTPHash          string    `json:"otp_hash"`
	OTPExpiresAt     time.Time `json:"otp_expires_at"`
	OTPAttempts      int       `json:"otp_attempts"`
	OTPRequestCount  int       `json:"otp_request_count"`
	OTPLastSentAt    time.Time `json:"otp_last_sent_at"`
	SessionTokenHash string    `json:"session_token_hash"`
	SessionExpiresAt time.Time `json:"session_expires_at"`
	SubmissionID     string    `json:"submission_id,omitempty"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type VisitorAttachment struct {
	ID               RecordID  `json:"id,omitempty"`
	VisitorRequestID string    `json:"visitor_request_id"`
	SubmissionID     string    `json:"submission_id,omitempty"`
	Name             string    `json:"name"`
	MimeType         string    `json:"mime_type"`
	Size             int64     `json:"size"`
	StoragePath      string    `json:"storage_path"`
	CreatedAt        time.Time `json:"created_at"`
}
