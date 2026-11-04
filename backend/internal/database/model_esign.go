package database

import "time"

// SignatureEnvelope is a document sent out for signature. The PDF that
// recipients sign is frozen when the envelope is created.
type SignatureEnvelope struct {
	ID                 RecordID  `json:"id,omitempty"`
	OrgID              string    `json:"org_id"`
	DocumentID         string    `json:"document_id"`
	SourceVersionID    string    `json:"source_version_id,omitempty"`
	SourcePath         string    `json:"source_path"`
	SourceHash         string    `json:"source_hash"`
	Pages              int       `json:"pages"`
	Title              string    `json:"title"`
	Message            string    `json:"message,omitempty"`
	Status             string    `json:"status"`
	Sequential         bool      `json:"sequential"`
	RequireOTP         bool      `json:"require_otp"`
	CreatedBy          string    `json:"created_by"`
	CompletedVersionID string    `json:"completed_version_id,omitempty"`
	CompletedHash      string    `json:"completed_hash,omitempty"`
	CompletedAt        time.Time `json:"completed_at,omitempty"`
	VoidedAt           time.Time `json:"voided_at,omitempty"`
	VoidReason         string    `json:"void_reason,omitempty"`
	ExpiresAt          time.Time `json:"expires_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

const (
	EnvelopeStatusSent      = "sent"
	EnvelopeStatusCompleted = "completed"
	EnvelopeStatusDeclined  = "declined"
	EnvelopeStatusVoided    = "voided"
)

// SignatureRecipient is one person asked to sign or receive a copy.
type SignatureRecipient struct {
	ID            RecordID  `json:"id,omitempty"`
	OrgID         string    `json:"org_id"`
	EnvelopeID    string    `json:"envelope_id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	UserID        string    `json:"user_id,omitempty"`
	Role          string    `json:"role"`
	Position      int       `json:"position"`
	Status        string    `json:"status"`
	Nonce         string    `json:"nonce,omitempty"`
	TokenHash     string    `json:"token_hash,omitempty"`
	OTPHash       string    `json:"otp_hash,omitempty"`
	OTPExpiresAt  time.Time `json:"otp_expires_at,omitempty"`
	OTPAttempts   int       `json:"otp_attempts"`
	VerifiedAt    time.Time `json:"verified_at,omitempty"`
	ViewedAt      time.Time `json:"viewed_at,omitempty"`
	SignedAt      time.Time `json:"signed_at,omitempty"`
	DeclinedAt    time.Time `json:"declined_at,omitempty"`
	DeclineReason string    `json:"decline_reason,omitempty"`
	IP            string    `json:"ip,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const (
	RecipientRoleSigner = "signer"
	RecipientRoleViewer = "viewer"

	RecipientStatusWaiting  = "waiting"
	RecipientStatusSent     = "sent"
	RecipientStatusViewed   = "viewed"
	RecipientStatusSigned   = "signed"
	RecipientStatusDeclined = "declined"
)

// SignatureField is a box a recipient fills. Position and size are
// fractions of the page, measured from its top-left corner.
type SignatureField struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	EnvelopeID  string    `json:"envelope_id"`
	RecipientID string    `json:"recipient_id"`
	Kind        string    `json:"kind"`
	Page        int       `json:"page"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	W           float64   `json:"w"`
	H           float64   `json:"h"`
	Required    bool      `json:"required"`
	Label       string    `json:"label,omitempty"`
	Value       string    `json:"value,omitempty"`
	ImagePath   string    `json:"image_path,omitempty"`
	FilledAt    time.Time `json:"filled_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	FieldSignature = "signature"
	FieldInitials  = "initials"
	FieldDate      = "date"
	FieldName      = "name"
	FieldText      = "text"
	FieldCheckbox  = "checkbox"
)

// SignatureEvent is one step in an envelope's audit trail, printed on the
// certificate of completion.
type SignatureEvent struct {
	ID          RecordID  `json:"id,omitempty"`
	OrgID       string    `json:"org_id"`
	EnvelopeID  string    `json:"envelope_id"`
	RecipientID string    `json:"recipient_id,omitempty"`
	Action      string    `json:"action"`
	Detail      string    `json:"detail,omitempty"`
	IP          string    `json:"ip,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
