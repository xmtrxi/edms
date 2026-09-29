package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/queue"
	"github.com/xmtrxi/edms/internal/storage"
)

const (
	visitorOTPTTL     = 10 * time.Minute
	visitorSessionTTL = 30 * time.Minute
	visitorMaxTries   = 5
	visitorOTPDelay   = time.Minute
	visitorOTPMax     = 5
)

var ErrVisitorAccess = errors.New("the visitor credentials are invalid or expired")

type CreateVisitorRequestInput struct {
	TemplateID   string    `json:"template_id"`
	VisitorName  string    `json:"visitor_name"`
	VisitorEmail string    `json:"visitor_email"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type VisitorRequestCreated struct {
	Request    VisitorRequestView `json:"request"`
	AccessCode string             `json:"access_code"`
	PublicURL  string             `json:"public_url"`
}

type VisitorRequestView struct {
	ID           string    `json:"id"`
	TemplateID   string    `json:"template_id"`
	DepartmentID string    `json:"department_id"`
	VisitorName  string    `json:"visitor_name"`
	VisitorEmail string    `json:"visitor_email"`
	SubmissionID string    `json:"submission_id,omitempty"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type VisitorAccessInput struct {
	Email      string `json:"email"`
	AccessCode string `json:"access_code"`
}

type VisitorVerifyInput struct {
	Email      string `json:"email"`
	AccessCode string `json:"access_code"`
	OTP        string `json:"otp"`
}

type VisitorSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type VisitorPortal struct {
	Request    VisitorRequestView          `json:"request"`
	Template   database.FormTemplate       `json:"template"`
	Submission *database.FormSubmission    `json:"submission,omitempty"`
	Steps      []database.FormApprovalStep `json:"steps"`
}

type VisitorOTPDelivery struct {
	DevelopmentOTP string `json:"development_otp,omitempty"`
}

type VisitorService struct {
	db      *surrealdb.DB
	cfg     config.Config
	forms   *FormsService
	storage storage.Provider
	email   *OperationsService
}

func NewVisitorService(cfg config.Config, db *surrealdb.DB, provider storage.Provider, queueClient *queue.Client) *VisitorService {
	return &VisitorService{db: db, cfg: cfg, forms: NewFormsService(db), storage: provider, email: NewOperationsService(db, cfg.SMTP, queueClient)}
}

func (s *VisitorService) List(ctx context.Context, actor string) ([]VisitorRequestView, error) {
	results, err := surrealdb.Query[[]database.VisitorRequest](ctx, s.db, "SELECT * FROM visitor_requests WHERE created_by = $actor ORDER BY created_at DESC", map[string]any{"actor": actor})
	if err != nil || results == nil || len(*results) == 0 {
		return nil, err
	}
	items := make([]VisitorRequestView, 0, len((*results)[0].Result))
	for i := range (*results)[0].Result {
		items = append(items, visitorView(&(*results)[0].Result[i]))
	}
	return items, nil
}

func (s *VisitorService) Get(ctx context.Context, actor, id string) (*VisitorPortal, error) {
	request, err := s.staffRequest(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	return s.portalForRequest(ctx, request)
}

func (s *VisitorService) Revoke(ctx context.Context, actor, id string) error {
	request, err := s.staffRequest(ctx, actor, id)
	if err != nil {
		return err
	}
	_, err = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(string(request.ID)), map[string]any{"status": "revoked", "session_token_hash": "", "updated_at": time.Now().UTC()})
	if err == nil {
		_ = writeAudit(ctx, s.db, actor, "visitor.request_revoked", "visitor_request", id, nil)
	}
	return err
}

func (s *VisitorService) Resend(ctx context.Context, actor, id string) (*VisitorRequestCreated, error) {
	request, err := s.staffRequest(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if request.Status == "revoked" || request.ExpiresAt.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("visitor request is no longer active")
	}
	code, err := randomCode(8, "ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	if err != nil {
		return nil, err
	}
	_, err = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(id), map[string]any{"access_code_hash": secretHash(code), "otp_hash": "", "session_token_hash": "", "otp_request_count": 0, "updated_at": time.Now().UTC()})
	if err != nil {
		return nil, err
	}
	_ = s.sendAccessCode(ctx, actor, request.VisitorEmail, request.VisitorName, code)
	return &VisitorRequestCreated{Request: visitorView(request), AccessCode: code, PublicURL: strings.TrimRight(s.cfg.SMTP.AppURL, "/") + "/visit"}, nil
}

func (s *VisitorService) Create(ctx context.Context, actor string, in CreateVisitorRequestInput) (*VisitorRequestCreated, error) {
	name := strings.TrimSpace(in.VisitorName)
	email := strings.ToLower(strings.TrimSpace(in.VisitorEmail))
	if name == "" || email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("visitor name and a valid email are required")
	}
	rid, err := database.ParseRecordID(in.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("published form not found")
	}
	template, err := surrealdb.Select[database.FormTemplate](ctx, s.db, rid)
	if err != nil || template == nil || template.Status != "published" {
		return nil, fmt.Errorf("published form not found")
	}
	expiresAt := in.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(72 * time.Hour)
	}
	if expiresAt.Before(time.Now().UTC().Add(15 * time.Minute)) {
		return nil, fmt.Errorf("expiry must be at least 15 minutes from now")
	}
	accessCode, err := randomCode(8, "ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	request, err := surrealdb.Create[database.VisitorRequest](ctx, s.db, "visitor_requests", &database.VisitorRequest{
		TemplateID: in.TemplateID, DepartmentID: template.DepartmentID, VisitorName: name,
		VisitorEmail: email, AccessCodeHash: secretHash(accessCode), Status: "awaiting_visitor",
		ExpiresAt: expiresAt, CreatedBy: actor, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.db, actor, "visitor.request_created", "visitor_request", string(request.ID), map[string]any{"template_id": in.TemplateID})
	return &VisitorRequestCreated{Request: visitorView(request), AccessCode: accessCode, PublicURL: strings.TrimRight(s.cfg.SMTP.AppURL, "/") + "/visit"}, nil
}

func (s *VisitorService) RequestOTP(ctx context.Context, in VisitorAccessInput) (*VisitorOTPDelivery, error) {
	request, err := s.findByCredentials(ctx, in.Email, in.AccessCode)
	if err != nil {
		return nil, ErrVisitorAccess
	}
	now := time.Now().UTC()
	if !request.OTPLastSentAt.IsZero() && now.Sub(request.OTPLastSentAt) < visitorOTPDelay {
		return nil, fmt.Errorf("please wait before requesting another code")
	}
	if request.OTPRequestCount >= visitorOTPMax {
		return nil, fmt.Errorf("verification code request limit reached")
	}
	otp, err := randomCode(6, "0123456789")
	if err != nil {
		return nil, err
	}
	if _, err = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(string(request.ID)), map[string]any{
		"otp_hash": secretHash(otp), "otp_expires_at": now.Add(visitorOTPTTL), "otp_attempts": 0, "otp_request_count": request.OTPRequestCount + 1, "otp_last_sent_at": now, "updated_at": now,
	}); err != nil {
		return nil, err
	}
	if err = s.sendOTP(ctx, string(request.ID), request.VisitorEmail, request.VisitorName, otp); err != nil {
		return nil, err
	}
	result := &VisitorOTPDelivery{}
	if s.cfg.App.Env != "production" && s.cfg.App.Env != "prod" {
		result.DevelopmentOTP = otp
	}
	return result, nil
}

func (s *VisitorService) Verify(ctx context.Context, in VisitorVerifyInput) (*VisitorSession, error) {
	request, err := s.findByCredentials(ctx, in.Email, in.AccessCode)
	if err != nil || request.OTPExpiresAt.Before(time.Now().UTC()) || request.OTPAttempts >= visitorMaxTries {
		return nil, ErrVisitorAccess
	}
	if !secretMatches(request.OTPHash, strings.TrimSpace(in.OTP)) {
		_, _ = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(string(request.ID)), map[string]any{"otp_attempts": request.OTPAttempts + 1})
		return nil, ErrVisitorAccess
	}
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("create visitor session: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().UTC().Add(visitorSessionTTL)
	if request.ExpiresAt.Before(expiresAt) {
		expiresAt = request.ExpiresAt
	}
	_, err = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(string(request.ID)), map[string]any{
		"session_token_hash": secretHash(token), "session_expires_at": expiresAt, "otp_hash": "", "updated_at": time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	return &VisitorSession{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *VisitorService) Portal(ctx context.Context, token string) (*VisitorPortal, error) {
	request, err := s.findBySession(ctx, token)
	if err != nil {
		return nil, ErrVisitorAccess
	}
	return s.portalForRequest(ctx, request)
}

func (s *VisitorService) portalForRequest(ctx context.Context, request *database.VisitorRequest) (*VisitorPortal, error) {
	templateID, err := database.ParseRecordID(request.TemplateID)
	if err != nil {
		return nil, ErrVisitorAccess
	}
	template, err := surrealdb.Select[database.FormTemplate](ctx, s.db, templateID)
	if err != nil || template == nil {
		return nil, ErrVisitorAccess
	}
	portal := &VisitorPortal{Request: visitorView(request), Template: *template, Steps: []database.FormApprovalStep{}}
	if request.SubmissionID != "" {
		submissionID, parseErr := database.ParseRecordID(request.SubmissionID)
		if parseErr == nil {
			portal.Submission, _ = surrealdb.Select[database.FormSubmission](ctx, s.db, submissionID)
		}
		results, queryErr := surrealdb.Query[[]database.FormApprovalStep](ctx, s.db, "SELECT * FROM form_approval_steps WHERE submission_id = $submission ORDER BY sequence", map[string]any{"submission": request.SubmissionID})
		if queryErr == nil && results != nil && len(*results) > 0 {
			portal.Steps = (*results)[0].Result
		}
	}
	return portal, nil
}

func (s *VisitorService) AddAttachment(ctx context.Context, token, name, mime string, size int64, file io.Reader) (*database.VisitorAttachment, error) {
	request, err := s.findBySession(ctx, token)
	if err != nil || s.storage == nil {
		return nil, ErrVisitorAccess
	}
	cleanName := filepath.Base(strings.TrimSpace(name))
	if cleanName == "" || cleanName == "." || size <= 0 || size > 20*1024*1024 {
		return nil, fmt.Errorf("attachment must be between 1 byte and 20 MB")
	}
	key, err := randomCode(24, "abcdefghijklmnopqrstuvwxyz0123456789")
	if err != nil {
		return nil, err
	}
	path := "visitor-attachments/" + key + "-" + cleanName
	if err = s.storage.Upload(ctx, path, file); err != nil {
		return nil, err
	}
	attachment, err := surrealdb.Create[database.VisitorAttachment](ctx, s.db, "visitor_attachments", &database.VisitorAttachment{VisitorRequestID: string(request.ID), SubmissionID: request.SubmissionID, Name: cleanName, MimeType: mime, Size: size, StoragePath: path, CreatedAt: time.Now().UTC()})
	if err != nil {
		_ = s.storage.Delete(ctx, path)
		return nil, err
	}
	return attachment, nil
}

func (s *VisitorService) Attachments(ctx context.Context, token string) ([]database.VisitorAttachment, error) {
	request, err := s.findBySession(ctx, token)
	if err != nil {
		return nil, ErrVisitorAccess
	}
	return s.attachmentsFor(ctx, string(request.ID))
}

func (s *VisitorService) DeleteAttachment(ctx context.Context, token, id string) error {
	request, err := s.findBySession(ctx, token)
	if err != nil {
		return ErrVisitorAccess
	}
	attachment, err := s.attachment(ctx, id)
	if err != nil || attachment.VisitorRequestID != string(request.ID) {
		return ErrVisitorAccess
	}
	if err = s.storage.Delete(ctx, attachment.StoragePath); err != nil {
		return err
	}
	_, err = surrealdb.Delete[database.VisitorAttachment](ctx, s.db, mustRecordID(id))
	return err
}

func (s *VisitorService) DownloadAttachment(ctx context.Context, actor, requestID, attachmentID string) (*database.VisitorAttachment, io.ReadCloser, error) {
	if _, err := s.staffRequest(ctx, actor, requestID); err != nil {
		return nil, nil, err
	}
	attachment, err := s.attachment(ctx, attachmentID)
	if err != nil || attachment.VisitorRequestID != requestID {
		return nil, nil, fmt.Errorf("attachment not found")
	}
	reader, err := s.storage.Download(ctx, attachment.StoragePath)
	return attachment, reader, err
}

func (s *VisitorService) attachmentsFor(ctx context.Context, requestID string) ([]database.VisitorAttachment, error) {
	r, err := surrealdb.Query[[]database.VisitorAttachment](ctx, s.db, "SELECT * FROM visitor_attachments WHERE visitor_request_id = $request ORDER BY created_at DESC", map[string]any{"request": requestID})
	if err != nil || r == nil || len(*r) == 0 {
		return nil, err
	}
	return (*r)[0].Result, nil
}

func (s *VisitorService) attachment(ctx context.Context, id string) (*database.VisitorAttachment, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("attachment not found")
	}
	item, err := surrealdb.Select[database.VisitorAttachment](ctx, s.db, rid)
	if err != nil || item == nil {
		return nil, fmt.Errorf("attachment not found")
	}
	return item, nil
}

func (s *VisitorService) staffRequest(ctx context.Context, actor, id string) (*database.VisitorRequest, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("visitor request not found")
	}
	request, err := surrealdb.Select[database.VisitorRequest](ctx, s.db, rid)
	if err != nil || request == nil || request.CreatedBy != actor {
		return nil, fmt.Errorf("visitor request not found")
	}
	return request, nil
}

func (s *VisitorService) Submit(ctx context.Context, token string, values map[string]any) (*database.FormSubmission, error) {
	request, err := s.findBySession(ctx, token)
	if err != nil || request.SubmissionID != "" {
		return nil, ErrVisitorAccess
	}
	submission, err := s.forms.SubmitVisitor(ctx, string(request.ID), request.TemplateID, request.DepartmentID, values)
	if err != nil {
		return nil, err
	}
	_, err = surrealdb.Merge[database.VisitorRequest](ctx, s.db, mustRecordID(string(request.ID)), map[string]any{
		"submission_id": string(submission.ID), "status": "submitted", "updated_at": time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	return submission, nil
}

func (s *VisitorService) findByCredentials(ctx context.Context, email, accessCode string) (*database.VisitorRequest, error) {
	results, err := surrealdb.Query[[]database.VisitorRequest](ctx, s.db, "SELECT * FROM visitor_requests WHERE visitor_email = $email AND status != 'revoked' ORDER BY created_at DESC", map[string]any{"email": strings.ToLower(strings.TrimSpace(email))})
	if err != nil || results == nil || len(*results) == 0 {
		return nil, ErrVisitorAccess
	}
	for _, request := range (*results)[0].Result {
		if request.ExpiresAt.After(time.Now().UTC()) && secretMatches(request.AccessCodeHash, strings.ToUpper(strings.TrimSpace(accessCode))) {
			return &request, nil
		}
	}
	return nil, ErrVisitorAccess
}

func (s *VisitorService) findBySession(ctx context.Context, token string) (*database.VisitorRequest, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrVisitorAccess
	}
	results, err := surrealdb.Query[[]database.VisitorRequest](ctx, s.db, "SELECT * FROM visitor_requests WHERE session_token_hash = $hash LIMIT 1", map[string]any{"hash": secretHash(token)})
	if err != nil || results == nil || len(*results) == 0 || len((*results)[0].Result) == 0 {
		return nil, ErrVisitorAccess
	}
	request := (*results)[0].Result[0]
	if request.ExpiresAt.Before(time.Now().UTC()) || request.SessionExpiresAt.Before(time.Now().UTC()) || request.Status == "revoked" {
		return nil, ErrVisitorAccess
	}
	return &request, nil
}

func (s *VisitorService) sendOTP(ctx context.Context, actor, email, name, otp string) error {
	if s.cfg.SMTP.Host == "" {
		if s.cfg.App.Env == "production" || s.cfg.App.Env == "prod" {
			return fmt.Errorf("visitor email delivery is not configured")
		}
		return nil
	}
	_, err := s.email.QueueEmail(ctx, actor, email, "Your EDMS verification code", "Hello "+name+",\n\nYour verification code is "+otp+". It expires in 10 minutes.\n")
	return err
}

func (s *VisitorService) sendAccessCode(ctx context.Context, actor, email, name, code string) error {
	if s.cfg.SMTP.Host == "" {
		return nil
	}
	_, err := s.email.QueueEmail(ctx, actor, email, "Your EDMS visitor request", "Hello "+name+",\n\nOpen "+strings.TrimRight(s.cfg.SMTP.AppURL, "/")+"/visit and enter access code "+code+".\n")
	return err
}

func secretHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func secretMatches(hash, value string) bool {
	want, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	got, _ := hex.DecodeString(secretHash(value))
	return len(want) == len(got) && subtle.ConstantTimeCompare(want, got) == 1
}

func randomCode(length int, alphabet string) (string, error) {
	if length < 1 || alphabet == "" {
		return "", fmt.Errorf("invalid code configuration")
	}
	result := make([]byte, length)
	for i := range result {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", fmt.Errorf("generate secure code: %w", err)
		}
		result[i] = alphabet[value.Int64()]
	}
	return string(result), nil
}

func visitorView(request *database.VisitorRequest) VisitorRequestView {
	return VisitorRequestView{
		ID: string(request.ID), TemplateID: request.TemplateID, DepartmentID: request.DepartmentID,
		VisitorName: request.VisitorName, VisitorEmail: request.VisitorEmail, SubmissionID: request.SubmissionID,
		Status: request.Status, ExpiresAt: request.ExpiresAt, CreatedAt: request.CreatedAt,
	}
}
