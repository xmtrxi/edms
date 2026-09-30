package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/mail"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/esign"
	"github.com/xmtrxi/edms/internal/storage"
)

var (
	ErrEnvelopeNotFound = errors.New("signature request not found")
	ErrEnvelopeClosed   = errors.New("this signature request is no longer open")
	ErrNotYourTurn      = errors.New("it is not your turn to sign yet")
	ErrCodeRequired     = errors.New("confirm the code sent to your email first")
	ErrCodeInvalid      = errors.New("that code is not correct or has expired")
	ErrCodeTooSoon      = errors.New("a code was sent a moment ago; check your email")
)

const (
	maxSignSource      = 100 << 20
	maxSignatureImage  = 1 << 20
	otpLifetime        = 10 * time.Minute
	otpMaxAttempts     = 5
	verificationWindow = 2 * time.Hour
)

// ESignService runs the built-in signing flow: it freezes a PDF, collects
// signatures from each recipient, then stamps them, appends a certificate of
// completion, seals the file and stores it as a new document version.
type ESignService struct {
	db       *surrealdb.DB
	storage  storage.Provider
	docs     *DocumentService
	versions *VersionService
	previews *PreviewService
	email    *OperationsService
	cfg      config.SigningConfig
	appURL   string
	secret   []byte
	log      *slog.Logger

	sealerMu sync.Mutex
	sealer   *esign.Sealer
}

func NewESignService(cfg config.Config, db *surrealdb.DB, provider storage.Provider, docs *DocumentService, previews *PreviewService, email *OperationsService, logger *slog.Logger) *ESignService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ESignService{
		db: db, storage: provider, docs: docs, versions: NewVersionService(db, provider, docs), previews: previews, email: email,
		cfg: cfg.Signing, appURL: strings.TrimRight(cfg.SMTP.AppURL, "/"), secret: []byte("esign:" + cfg.JWT.Secret), log: logger,
	}
}

// ── Inputs and views ────────────────────────────────────────────────────────

type RecipientInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type FieldInput struct {
	// Recipient is the index into EnvelopeInput.Recipients.
	Recipient int     `json:"recipient"`
	Kind      string  `json:"kind"`
	Page      int     `json:"page"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	W         float64 `json:"w"`
	H         float64 `json:"h"`
	Required  *bool   `json:"required"`
	Label     string  `json:"label"`
}

type EnvelopeInput struct {
	Title      string           `json:"title"`
	Message    string           `json:"message"`
	Sequential bool             `json:"sequential"`
	RequireOTP *bool            `json:"require_otp"`
	ExpiresIn  int              `json:"expires_in_days"`
	Recipients []RecipientInput `json:"recipients"`
	Fields     []FieldInput     `json:"fields"`
}

type RecipientView struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	Position      int       `json:"position"`
	Status        string    `json:"status"`
	ViewedAt      time.Time `json:"viewed_at,omitzero"`
	SignedAt      time.Time `json:"signed_at,omitzero"`
	DeclinedAt    time.Time `json:"declined_at,omitzero"`
	DeclineReason string    `json:"decline_reason,omitempty"`
	// Link is only included for the envelope's sender.
	Link string `json:"link,omitempty"`
}

type FieldView struct {
	ID          string  `json:"id"`
	RecipientID string  `json:"recipient_id"`
	Kind        string  `json:"kind"`
	Page        int     `json:"page"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	W           float64 `json:"w"`
	H           float64 `json:"h"`
	Required    bool    `json:"required"`
	Label       string  `json:"label,omitempty"`
	Filled      bool    `json:"filled"`
	Value       string  `json:"value,omitempty"`
}

type EventView struct {
	Action    string    `json:"action"`
	Actor     string    `json:"actor,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type EnvelopeView struct {
	ID                 string          `json:"id"`
	DocumentID         string          `json:"document_id"`
	DocumentName       string          `json:"document_name,omitempty"`
	Title              string          `json:"title"`
	Message            string          `json:"message,omitempty"`
	Status             string          `json:"status"`
	Sequential         bool            `json:"sequential"`
	RequireOTP         bool            `json:"require_otp"`
	Pages              int             `json:"pages"`
	CreatedBy          string          `json:"created_by"`
	SenderName         string          `json:"sender_name,omitempty"`
	CompletedVersionID string          `json:"completed_version_id,omitempty"`
	CompletedAt        time.Time       `json:"completed_at,omitzero"`
	VoidReason         string          `json:"void_reason,omitempty"`
	ExpiresAt          time.Time       `json:"expires_at"`
	CreatedAt          time.Time       `json:"created_at"`
	Recipients         []RecipientView `json:"recipients"`
	Fields             []FieldView     `json:"fields,omitempty"`
	Events             []EventView     `json:"events,omitempty"`
	// SigningLink is set in the inbox, for the signed-in recipient.
	SigningLink string `json:"signing_link,omitempty"`
}

// ── Sending ─────────────────────────────────────────────────────────────────

var fieldKinds = map[string]bool{
	database.FieldSignature: true, database.FieldInitials: true, database.FieldDate: true,
	database.FieldName: true, database.FieldText: true, database.FieldCheckbox: true,
}

func validateEnvelope(in *EnvelopeInput) error {
	in.Title = strings.TrimSpace(in.Title)
	if len(in.Recipients) == 0 || len(in.Recipients) > 25 {
		return errors.New("add between 1 and 25 recipients")
	}
	seen := map[string]bool{}
	signers := 0
	for i := range in.Recipients {
		r := &in.Recipients[i]
		r.Name, r.Email = strings.TrimSpace(r.Name), strings.ToLower(strings.TrimSpace(r.Email))
		if addr, err := mail.ParseAddress(r.Email); err != nil || addr.Address != r.Email || !strings.Contains(r.Email[strings.LastIndex(r.Email, "@"):], ".") {
			return fmt.Errorf("%q is not a valid email address", r.Email)
		}
		if seen[r.Email] {
			return fmt.Errorf("%s is listed twice", r.Email)
		}
		seen[r.Email] = true
		if r.Name == "" {
			r.Name = strings.Split(r.Email, "@")[0]
		}
		if r.Role == "" {
			r.Role = database.RecipientRoleSigner
		}
		if r.Role != database.RecipientRoleSigner && r.Role != database.RecipientRoleViewer {
			return fmt.Errorf("unknown recipient role %q", r.Role)
		}
		if r.Role == database.RecipientRoleSigner {
			signers++
		}
	}
	if signers == 0 {
		return errors.New("add at least one signer")
	}
	if len(in.Fields) > 500 {
		return errors.New("too many fields")
	}
	hasField := map[int]bool{}
	for _, f := range in.Fields {
		if f.Recipient < 0 || f.Recipient >= len(in.Recipients) || in.Recipients[f.Recipient].Role != database.RecipientRoleSigner {
			return errors.New("every field must belong to a signer")
		}
		if !fieldKinds[f.Kind] {
			return fmt.Errorf("unknown field type %q", f.Kind)
		}
		if f.W <= 0 || f.H <= 0 || f.X < 0 || f.Y < 0 || f.X+f.W > 1.0001 || f.Y+f.H > 1.0001 {
			return errors.New("a field is outside the page")
		}
		hasField[f.Recipient] = true
	}
	for i, r := range in.Recipients {
		if r.Role == database.RecipientRoleSigner && !hasField[i] {
			return fmt.Errorf("place at least one field for %s", r.Name)
		}
	}
	return nil
}

// Create freezes the document as a PDF and sends it to the first signers.
// It needs the sign capability on the document.
func (s *ESignService) Create(ctx context.Context, userID, documentID string, in EnvelopeInput) (*EnvelopeView, error) {
	doc, err := s.docs.GetFor(ctx, userID, "sign", documentID)
	if err != nil {
		return nil, err
	}
	if err := validateEnvelope(&in); err != nil {
		return nil, err
	}
	rendition, err := s.previews.Rendition(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("this file can't be prepared for signing: %w", err)
	}
	defer rendition.Body.Close()
	if rendition.Kind != PreviewPDF {
		return nil, errors.New("only PDFs and Office documents can be signed")
	}
	source, err := io.ReadAll(io.LimitReader(rendition.Body, maxSignSource+1))
	if err != nil {
		return nil, err
	}
	if len(source) > maxSignSource {
		return nil, errors.New("the file is too large to sign")
	}
	pages, err := esign.PageCount(ctx, source)
	if err != nil {
		return nil, errors.New("the PDF could not be read")
	}
	for _, f := range in.Fields {
		if f.Page < 1 || f.Page > pages {
			return nil, fmt.Errorf("a field is on page %d, but the document has %d", f.Page, pages)
		}
	}

	orgID := firstNonEmpty(doc.OrgID, orgOf(ctx))
	folder := randomKey()
	sourcePath := path.Join("esign", recordKey(orgID), folder, "source.pdf")
	if err := s.storage.Upload(ctx, sourcePath, bytes.NewReader(source)); err != nil {
		return nil, fmt.Errorf("store the document to sign: %w", err)
	}
	now := time.Now().UTC()
	days := in.ExpiresIn
	if days <= 0 || days > 365 {
		days = 30
	}
	requireOTP := in.RequireOTP == nil || *in.RequireOTP
	title := firstNonEmpty(in.Title, doc.Name)
	record := map[string]any{
		"org_id": orgID, "document_id": string(doc.ID), "source_path": sourcePath, "source_hash": esign.Hash(source),
		"pages": pages, "title": title, "message": strings.TrimSpace(in.Message), "status": database.EnvelopeStatusSent,
		"sequential": in.Sequential, "require_otp": requireOTP, "created_by": userID,
		"expires_at": now.AddDate(0, 0, days), "created_at": now, "updated_at": now,
	}
	if doc.CurrentVersionID != "" {
		record["source_version_id"] = doc.CurrentVersionID
	}
	envelope, err := surrealdb.Create[database.SignatureEnvelope](ctx, s.db, "signature_envelopes", record)
	if err != nil {
		_ = s.storage.Delete(ctx, sourcePath)
		return nil, err
	}
	envelopeID := string(envelope.ID)

	recipientIDs := make([]string, len(in.Recipients))
	for i, r := range in.Recipients {
		row := map[string]any{
			"org_id": orgID, "envelope_id": envelopeID, "name": r.Name, "email": r.Email, "role": r.Role,
			"position": i + 1, "status": database.RecipientStatusWaiting, "nonce": randomKey(), "token_hash": "pending:" + randomKey(),
			"otp_attempts": 0, "created_at": now, "updated_at": now,
		}
		if user, err := s.memberByEmail(ctx, orgID, r.Email); err == nil && user != "" {
			row["user_id"] = user
		}
		created, err := surrealdb.Create[database.SignatureRecipient](ctx, s.db, "signature_recipients", row)
		if err != nil {
			return nil, err
		}
		recipientIDs[i] = string(created.ID)
		// The token depends on the record ID, so its hash is set afterwards.
		token := s.token(recipientIDs[i], created.Nonce)
		if _, err := surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(created.ID)), map[string]any{"token_hash": secretHash(token)}); err != nil {
			return nil, err
		}
	}
	for _, f := range in.Fields {
		required := f.Required == nil || *f.Required
		if f.Kind == database.FieldCheckbox && f.Required == nil {
			required = false
		}
		if _, err := surrealdb.Create[database.SignatureField](ctx, s.db, "signature_fields", map[string]any{
			"org_id": orgID, "envelope_id": envelopeID, "recipient_id": recipientIDs[f.Recipient], "kind": f.Kind,
			"page": f.Page, "x": f.X, "y": f.Y, "w": f.W, "h": f.H, "required": required,
			"label": strings.TrimSpace(f.Label), "created_at": now,
		}); err != nil {
			return nil, err
		}
	}
	s.event(ctx, envelope, "", "created", fmt.Sprintf("Sent %q for signature", title), "", "")
	_ = writeAudit(ctx, s.db, userID, "signature.requested", "document", string(doc.ID), map[string]any{"envelope_id": envelopeID, "recipients": len(in.Recipients)})
	if err := s.advance(ctx, envelope); err != nil {
		return nil, err
	}
	return s.view(ctx, envelope, true)
}

// advance emails the recipients whose turn it is. In sequential envelopes
// that is the next unsigned signer; otherwise every signer at once.
func (s *ESignService) advance(ctx context.Context, envelope *database.SignatureEnvelope) error {
	recipients, err := s.recipients(ctx, string(envelope.ID))
	if err != nil {
		return err
	}
	for _, r := range s.due(envelope, recipients) {
		if r.Status != database.RecipientStatusWaiting {
			continue
		}
		if _, err := surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(r.ID)), map[string]any{"status": database.RecipientStatusSent, "updated_at": time.Now().UTC()}); err != nil {
			return err
		}
		s.invite(ctx, envelope, &r)
	}
	return nil
}

// due returns the signers who may sign now.
func (s *ESignService) due(envelope *database.SignatureEnvelope, recipients []database.SignatureRecipient) []database.SignatureRecipient {
	var pending []database.SignatureRecipient
	for _, r := range recipients {
		if r.Role == database.RecipientRoleSigner && r.Status != database.RecipientStatusSigned {
			pending = append(pending, r)
		}
	}
	if envelope.Sequential && len(pending) > 1 {
		return pending[:1]
	}
	return pending
}

func (s *ESignService) invite(ctx context.Context, envelope *database.SignatureEnvelope, r *database.SignatureRecipient) {
	sender := s.userName(ctx, envelope.CreatedBy)
	body := fmt.Sprintf("Hello %s,\n\n%s has asked you to sign %q.\n\n", r.Name, sender, envelope.Title)
	if envelope.Message != "" {
		body += envelope.Message + "\n\n"
	}
	body += "Review and sign: " + s.link(r) + "\n\nThis link is personal to you. Don't forward it.\n"
	s.mail(ctx, envelope, r.Email, "Please sign: "+envelope.Title, body)
	s.event(ctx, envelope, string(r.ID), "sent", "Emailed "+r.Email, "", "")
}

// ── Signing (public, by link) ──────────────────────────────────────────────

// SigningSession is a recipient's view of an envelope opened by its link.
type SigningSession struct {
	Envelope     *EnvelopeView `json:"envelope"`
	Recipient    RecipientView `json:"recipient"`
	Organization string        `json:"organization,omitempty"`
	Verified     bool          `json:"verified"`
	CanSign      bool          `json:"can_sign"`
	WaitingOn    string        `json:"waiting_on,omitempty"`
	Fields       []FieldView   `json:"fields"`
}

// Open resolves a signing link. The first open is recorded as viewed.
func (s *ESignService) Open(ctx context.Context, token, ip, agent string) (context.Context, *SigningSession, error) {
	ctx, envelope, recipient, err := s.byToken(ctx, token)
	if err != nil {
		return ctx, nil, err
	}
	if recipient.ViewedAt.IsZero() && envelope.Status == database.EnvelopeStatusSent {
		now := time.Now().UTC()
		updates := map[string]any{"viewed_at": now, "updated_at": now}
		if recipient.Status == database.RecipientStatusSent {
			updates["status"] = database.RecipientStatusViewed
			recipient.Status = database.RecipientStatusViewed
		}
		recipient.ViewedAt = now
		_, _ = surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(recipient.ID)), updates)
		s.event(ctx, envelope, string(recipient.ID), "viewed", "Opened the document", ip, agent)
	}
	session, err := s.session(ctx, envelope, recipient)
	return ctx, session, err
}

func (s *ESignService) session(ctx context.Context, envelope *database.SignatureEnvelope, recipient *database.SignatureRecipient) (*SigningSession, error) {
	view, err := s.view(ctx, envelope, false)
	if err != nil {
		return nil, err
	}
	fields, err := s.fields(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	session := &SigningSession{
		Envelope: view, Recipient: recipientView(recipient),
		Verified: !envelope.RequireOTP || s.verified(recipient),
	}
	for _, f := range fields {
		if f.RecipientID == string(recipient.ID) {
			session.Fields = append(session.Fields, fieldView(&f, true))
		}
	}
	if orgs, err := queryRows[database.Organization](ctx, s.db, "SELECT * FROM organizations WHERE <string> id = $org LIMIT 1", map[string]any{"org": envelope.OrgID}); err == nil && len(orgs) > 0 {
		session.Organization = orgs[0].Name
	}
	if envelope.Status == database.EnvelopeStatusSent && recipient.Role == database.RecipientRoleSigner &&
		recipient.Status != database.RecipientStatusSigned && recipient.Status != database.RecipientStatusDeclined {
		recipients, err := s.recipients(ctx, string(envelope.ID))
		if err != nil {
			return nil, err
		}
		for _, r := range s.due(envelope, recipients) {
			if r.ID == recipient.ID {
				session.CanSign = true
			}
		}
		if !session.CanSign {
			if due := s.due(envelope, recipients); len(due) > 0 {
				session.WaitingOn = due[0].Name
			}
		}
	}
	return session, nil
}

func (s *ESignService) verified(r *database.SignatureRecipient) bool {
	return !r.VerifiedAt.IsZero() && time.Since(r.VerifiedAt) < verificationWindow
}

// SourcePDF streams the document a recipient signs, or the completed file
// once everyone has signed.
func (s *ESignService) SourcePDF(ctx context.Context, token string) (io.ReadCloser, string, error) {
	ctx, envelope, _, err := s.byToken(ctx, token)
	if err != nil {
		return nil, "", err
	}
	if envelope.Status == database.EnvelopeStatusCompleted && envelope.CompletedVersionID != "" {
		if version, err := s.versions.get(ctx, envelope.DocumentID, envelope.CompletedVersionID); err == nil {
			body, err := s.storage.Download(ctx, version.StoragePath)
			return body, envelope.Title + " (signed).pdf", err
		}
	}
	body, err := s.storage.Download(ctx, envelope.SourcePath)
	return body, envelope.Title + ".pdf", err
}

// SendCode emails a one-time code that confirms the recipient controls the
// address the link was sent to.
func (s *ESignService) SendCode(ctx context.Context, token, ip, agent string) error {
	ctx, envelope, recipient, err := s.byToken(ctx, token)
	if err != nil {
		return err
	}
	if envelope.Status != database.EnvelopeStatusSent {
		return ErrEnvelopeClosed
	}
	if !recipient.OTPExpiresAt.IsZero() && time.Until(recipient.OTPExpiresAt) > otpLifetime-30*time.Second {
		return ErrCodeTooSoon
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	if _, err := surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(recipient.ID)), map[string]any{
		"otp_hash": secretHash(string(recipient.ID) + ":" + code), "otp_expires_at": time.Now().UTC().Add(otpLifetime),
		"otp_attempts": 0, "updated_at": time.Now().UTC(),
	}); err != nil {
		return err
	}
	s.mail(ctx, envelope, recipient.Email, "Your signing code: "+code,
		fmt.Sprintf("Your code to sign %q is %s.\n\nIt expires in 10 minutes. If you didn't ask for it, ignore this email.\n", envelope.Title, code))
	s.event(ctx, envelope, string(recipient.ID), "code_sent", "Sent a verification code to "+recipient.Email, ip, agent)
	return nil
}

// VerifyCode checks the one-time code.
func (s *ESignService) VerifyCode(ctx context.Context, token, code, ip, agent string) error {
	ctx, envelope, recipient, err := s.byToken(ctx, token)
	if err != nil {
		return err
	}
	if recipient.OTPHash == "" || time.Now().After(recipient.OTPExpiresAt) || recipient.OTPAttempts >= otpMaxAttempts {
		return ErrCodeInvalid
	}
	if !secretMatches(recipient.OTPHash, string(recipient.ID)+":"+strings.TrimSpace(code)) {
		_, _ = surrealdb.Query[any](ctx, s.db, "UPDATE $r SET otp_attempts += 1", map[string]any{"r": mustRecordID(string(recipient.ID))})
		return ErrCodeInvalid
	}
	now := time.Now().UTC()
	if _, err := surrealdb.Query[any](ctx, s.db, "UPDATE $r SET verified_at = $now, otp_hash = NONE, otp_expires_at = NONE, updated_at = $now",
		map[string]any{"r": mustRecordID(string(recipient.ID)), "now": now}); err != nil {
		return err
	}
	s.event(ctx, envelope, string(recipient.ID), "verified", "Confirmed their email address with a code", ip, agent)
	return nil
}

// SignInput carries the recipient's field values by field ID. Signatures and
// initials are PNG data URLs; checkboxes are "true" or "false".
type SignInput struct {
	Values map[string]string `json:"values"`
	// Consent is the recipient agreeing to sign electronically.
	Consent bool `json:"consent"`
	// TimeZone is the signer's IANA zone, so date fields show their local date.
	TimeZone string `json:"time_zone"`
}

// Sign records a recipient's values. When they are the last signer, the
// document is completed.
func (s *ESignService) Sign(ctx context.Context, token string, in SignInput, ip, agent string) (*SigningSession, error) {
	ctx, envelope, recipient, err := s.byToken(ctx, token)
	if err != nil {
		return nil, err
	}
	session, err := s.session(ctx, envelope, recipient)
	if err != nil {
		return nil, err
	}
	if envelope.Status != database.EnvelopeStatusSent || recipient.Status == database.RecipientStatusSigned {
		return nil, ErrEnvelopeClosed
	}
	if !session.CanSign {
		return nil, ErrNotYourTurn
	}
	if !session.Verified {
		return nil, ErrCodeRequired
	}
	if !in.Consent {
		return nil, errors.New("agree to sign electronically to continue")
	}
	fields, err := s.fields(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	local := now
	if zone, err := time.LoadLocation(in.TimeZone); err == nil && in.TimeZone != "" && in.TimeZone != "Local" {
		local = now.In(zone)
	}
	type update struct {
		field  *database.SignatureField
		values map[string]any
	}
	var updates []update
	for i := range fields {
		f := &fields[i]
		if f.RecipientID != string(recipient.ID) {
			continue
		}
		value := strings.TrimSpace(in.Values[string(f.ID)])
		values := map[string]any{"filled_at": now}
		switch f.Kind {
		case database.FieldDate:
			value = local.Format("2 January 2006")
		case database.FieldName:
			if value == "" {
				value = recipient.Name
			}
		case database.FieldCheckbox:
			if value != "true" {
				value = ""
			}
		case database.FieldSignature, database.FieldInitials:
			if value == "" {
				break
			}
			image, err := decodeSignatureImage(value)
			if err != nil {
				return nil, err
			}
			imagePath := path.Join(path.Dir(envelope.SourcePath), "fields", recordKey(string(f.ID))+".png")
			if err := s.storage.Upload(ctx, imagePath, bytes.NewReader(image)); err != nil {
				return nil, fmt.Errorf("store signature: %w", err)
			}
			values["image_path"] = imagePath
			value = "signed"
		case database.FieldText:
			if len(value) > 500 {
				value = value[:500]
			}
		}
		if value == "" && f.Required {
			return nil, fmt.Errorf("complete every required field (%s on page %d)", fieldLabel(f), f.Page)
		}
		if value == "" {
			continue
		}
		values["value"] = value
		updates = append(updates, update{f, values})
	}
	for _, u := range updates {
		if _, err := surrealdb.Merge[database.SignatureField](ctx, s.db, mustRecordID(string(u.field.ID)), u.values); err != nil {
			return nil, err
		}
	}
	if _, err := surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(recipient.ID)), map[string]any{
		"status": database.RecipientStatusSigned, "signed_at": now, "ip": ip, "user_agent": truncate(agent, 300), "updated_at": now,
	}); err != nil {
		return nil, err
	}
	recipient.Status, recipient.SignedAt = database.RecipientStatusSigned, now
	s.event(ctx, envelope, string(recipient.ID), "signed", "Signed", ip, agent)

	recipients, err := s.recipients(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	if len(s.due(envelope, recipients)) == 0 {
		if err := s.complete(ctx, envelope); err != nil {
			s.log.Error("complete signature request", "envelope_id", string(envelope.ID), "error", err)
		}
	} else if err := s.advance(ctx, envelope); err != nil {
		return nil, err
	}
	fresh, err := s.envelope(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	return s.session(ctx, fresh, recipient)
}

// Decline ends the envelope; the sender is told why.
func (s *ESignService) Decline(ctx context.Context, token, reason, ip, agent string) error {
	ctx, envelope, recipient, err := s.byToken(ctx, token)
	if err != nil {
		return err
	}
	if envelope.Status != database.EnvelopeStatusSent || recipient.Status == database.RecipientStatusSigned {
		return ErrEnvelopeClosed
	}
	now := time.Now().UTC()
	reason = truncate(strings.TrimSpace(reason), 1000)
	if _, err := surrealdb.Merge[database.SignatureRecipient](ctx, s.db, mustRecordID(string(recipient.ID)), map[string]any{
		"status": database.RecipientStatusDeclined, "declined_at": now, "decline_reason": reason, "ip": ip, "updated_at": now,
	}); err != nil {
		return err
	}
	if _, err := surrealdb.Merge[database.SignatureEnvelope](ctx, s.db, mustRecordID(string(envelope.ID)), map[string]any{"status": database.EnvelopeStatusDeclined, "updated_at": now}); err != nil {
		return err
	}
	s.event(ctx, envelope, string(recipient.ID), "declined", firstNonEmpty(reason, "Declined to sign"), ip, agent)
	if sender := s.userEmail(ctx, envelope.CreatedBy); sender != "" {
		body := fmt.Sprintf("%s declined to sign %q.\n", recipient.Name, envelope.Title)
		if reason != "" {
			body += "\nTheir reason: " + reason + "\n"
		}
		s.mail(ctx, envelope, sender, "Declined: "+envelope.Title, body)
	}
	return nil
}

// ── Completion ──────────────────────────────────────────────────────────────

// complete stamps every value, appends the certificate, seals the PDF and
// stores it as the document's next version. Only one caller wins the claim.
func (s *ESignService) complete(ctx context.Context, envelope *database.SignatureEnvelope) error {
	claimed, err := queryRows[database.SignatureEnvelope](ctx, s.db,
		"UPDATE $e SET status = 'completing', updated_at = time::now() WHERE status = 'sent' RETURN AFTER", map[string]any{"e": mustRecordID(string(envelope.ID))})
	if err != nil || len(claimed) == 0 {
		return err
	}
	release := func(cause error) error {
		_, _ = surrealdb.Merge[database.SignatureEnvelope](ctx, s.db, mustRecordID(string(envelope.ID)), map[string]any{"status": database.EnvelopeStatusSent})
		return cause
	}
	sealed, err := s.render(ctx, envelope)
	if err != nil {
		return release(err)
	}
	doc, err := s.document(ctx, envelope.DocumentID)
	if err != nil {
		return release(err)
	}
	recipients, _ := s.recipients(ctx, string(envelope.ID))
	var names []string
	for _, r := range recipients {
		if r.Role == database.RecipientRoleSigner {
			names = append(names, r.Name)
		}
	}
	fileName := strings.TrimSuffix(doc.Name, path.Ext(doc.Name)) + ".pdf"
	version, _, err := s.versions.AddVersion(ctx, envelope.CreatedBy, envelope.DocumentID, NewVersionInput{
		File: bytes.NewReader(sealed), FileName: fileName, MimeType: "application/pdf",
		Source: database.VersionSourceSignature, Summary: "Signed by " + strings.Join(names, ", "), SkipAccessCheck: true,
	})
	if err != nil {
		return release(fmt.Errorf("store signed version: %w", err))
	}
	now := time.Now().UTC()
	if fileName != doc.Name {
		_, _ = surrealdb.Merge[database.Document](ctx, s.db, mustRecordID(string(doc.ID)), map[string]any{"name": fileName})
	}
	if _, err := surrealdb.Merge[database.SignatureEnvelope](ctx, s.db, mustRecordID(string(envelope.ID)), map[string]any{
		"status": database.EnvelopeStatusCompleted, "completed_version_id": string(version.ID), "completed_hash": esign.Hash(sealed),
		"completed_at": now, "updated_at": now,
	}); err != nil {
		return err
	}
	envelope.Status = database.EnvelopeStatusCompleted
	s.event(ctx, envelope, "", "completed", "All signers signed; the sealed copy was saved as a new version", "", "")
	_ = writeAudit(ctx, s.db, envelope.CreatedBy, "signature.completed", "document", envelope.DocumentID, map[string]any{"envelope_id": string(envelope.ID), "version_id": string(version.ID)})

	body := fmt.Sprintf("Everyone has signed %q.\n\nDownload the signed copy: %%s\n", envelope.Title)
	for _, r := range recipients {
		s.mail(ctx, envelope, r.Email, "Completed: "+envelope.Title, fmt.Sprintf(body, s.link(&r)))
	}
	if sender := s.userEmail(ctx, envelope.CreatedBy); sender != "" {
		s.mail(ctx, envelope, sender, "Completed: "+envelope.Title, fmt.Sprintf(body, s.appURL+"/documents/"+envelope.DocumentID))
	}
	return nil
}

// render builds the final sealed PDF.
func (s *ESignService) render(ctx context.Context, envelope *database.SignatureEnvelope) ([]byte, error) {
	source, err := s.read(ctx, envelope.SourcePath, maxSignSource)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	fields, err := s.fields(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	marks := make([]esign.Mark, 0, len(fields))
	for _, f := range fields {
		mark := esign.Mark{Page: f.Page, X: f.X, Y: f.Y, W: f.W, H: f.H}
		switch {
		case f.ImagePath != "":
			if mark.Image, err = s.read(ctx, f.ImagePath, maxSignatureImage); err != nil {
				return nil, fmt.Errorf("read signature: %w", err)
			}
		case f.Kind == database.FieldCheckbox && f.Value == "true":
			mark.Text = "X"
		case f.Kind != database.FieldCheckbox:
			mark.Text = f.Value
		}
		marks = append(marks, mark)
	}
	stamped, err := esign.Stamp(ctx, source, marks)
	if err != nil {
		return nil, err
	}
	sealer, sealErr := s.loadSealer(ctx)
	certificate, err := s.certificate(ctx, envelope, sealer)
	if err != nil {
		return nil, err
	}
	full, err := esign.Append(ctx, stamped, certificate)
	if err != nil {
		return nil, err
	}
	if sealErr != nil {
		s.log.Warn("signing seal unavailable; saving without a digital seal", "error", sealErr)
		return full, nil
	}
	sealed, err := sealer.Seal(full, esign.SealInfo{Name: s.cfg.SealName, Reason: "Signed: " + envelope.Title, Contact: s.appURL})
	if err != nil {
		s.log.Warn("seal signed document", "envelope_id", string(envelope.ID), "error", err)
		return full, nil
	}
	return sealed, nil
}

func (s *ESignService) certificate(ctx context.Context, envelope *database.SignatureEnvelope, sealer *esign.Sealer) ([]byte, error) {
	recipients, err := s.recipients(ctx, string(envelope.ID))
	if err != nil {
		return nil, err
	}
	events, err := queryRows[database.SignatureEvent](ctx, s.db, "SELECT * FROM signature_events WHERE envelope_id = $e ORDER BY created_at", map[string]any{"e": string(envelope.ID)})
	if err != nil {
		return nil, err
	}
	stamp := func(t time.Time) string { return t.UTC().Format("2 Jan 2006, 15:04:05 UTC") }
	lines := []esign.Line{
		{Text: "Certificate of completion", Size: 20, Bold: true},
		{Text: envelope.Title, Size: 12, Gap: 4},
		{Text: "Reference " + string(envelope.ID), Muted: true},
		{Text: "Summary", Size: 12, Bold: true, Gap: 14},
		{Text: "Sent by " + s.userName(ctx, envelope.CreatedBy) + " on " + stamp(envelope.CreatedAt)},
		{Text: "Completed on " + stamp(time.Now())},
		{Text: fmt.Sprintf("Pages in the original: %d", envelope.Pages)},
		{Text: "SHA-256 of the original: " + envelope.SourceHash, Muted: true},
	}
	if envelope.RequireOTP {
		lines = append(lines, esign.Line{Text: "Each signer confirmed their email address with a one-time code before signing."})
	}
	lines = append(lines, esign.Line{Text: "Recipients", Size: 12, Bold: true, Gap: 14})
	for _, r := range recipients {
		lines = append(lines, esign.Line{Text: fmt.Sprintf("%s <%s>", r.Name, r.Email), Bold: true, Gap: 6})
		switch {
		case r.Role == database.RecipientRoleViewer:
			lines = append(lines, esign.Line{Text: "Received a copy"})
		case !r.SignedAt.IsZero():
			lines = append(lines, esign.Line{Text: "Signed " + stamp(r.SignedAt) + firstNonEmpty(prefixed(" from ", r.IP), "")})
			if !r.VerifiedAt.IsZero() {
				lines = append(lines, esign.Line{Text: "Email verified " + stamp(r.VerifiedAt)})
			}
			if r.UserAgent != "" {
				lines = append(lines, esign.Line{Text: r.UserAgent, Muted: true, Size: 8})
			}
		}
	}
	lines = append(lines, esign.Line{Text: "Activity", Size: 12, Bold: true, Gap: 14})
	names := map[string]string{}
	for _, r := range recipients {
		names[string(r.ID)] = r.Name
	}
	for _, e := range events {
		text := stamp(e.CreatedAt) + "  " + firstNonEmpty(names[e.RecipientID], "System") + ": " + firstNonEmpty(e.Detail, e.Action)
		if e.IP != "" {
			text += " (" + e.IP + ")"
		}
		lines = append(lines, esign.Line{Text: text, Size: 9})
	}
	footer := "The pages before this certificate were signed electronically."
	if sealer != nil {
		footer += " The file carries a digital seal; any later change invalidates it. Seal certificate SHA-256: " + sealer.Fingerprint()
	}
	lines = append(lines, esign.Line{Text: footer, Muted: true, Size: 8, Gap: 18})
	return esign.TextPDF(lines), nil
}

// loadSealer reads the configured seal, or creates and keeps a self-signed
// one in storage.
func (s *ESignService) loadSealer(ctx context.Context) (*esign.Sealer, error) {
	s.sealerMu.Lock()
	defer s.sealerMu.Unlock()
	if s.sealer != nil {
		return s.sealer, nil
	}
	var certPEM, keyPEM []byte
	var err error
	if s.cfg.SealCertFile != "" && s.cfg.SealKeyFile != "" {
		if certPEM, err = os.ReadFile(s.cfg.SealCertFile); err != nil {
			return nil, err
		}
		if keyPEM, err = os.ReadFile(s.cfg.SealKeyFile); err != nil {
			return nil, err
		}
	} else {
		const certPath, keyPath = "system/esign/seal-cert.pem", "system/esign/seal-key.pem"
		certPEM, _ = s.read(ctx, certPath, 1<<16)
		keyPEM, _ = s.read(ctx, keyPath, 1<<16)
		if len(certPEM) == 0 || len(keyPEM) == 0 {
			if certPEM, keyPEM, err = esign.SelfSigned(s.cfg.SealName); err != nil {
				return nil, err
			}
			if err := s.storage.Upload(ctx, keyPath, bytes.NewReader(keyPEM)); err != nil {
				return nil, err
			}
			if err := s.storage.Upload(ctx, certPath, bytes.NewReader(certPEM)); err != nil {
				return nil, err
			}
			s.log.Info("created a self-signed seal for signed documents; set ESIGN_SEAL_CERT_FILE and ESIGN_SEAL_KEY_FILE to use your own")
		}
	}
	sealer, err := esign.NewSealer(certPEM, keyPEM, s.cfg.TSAURL)
	if err != nil {
		return nil, err
	}
	s.sealer = sealer
	return sealer, nil
}

// ── Sender actions ──────────────────────────────────────────────────────────

// ForDocument lists a document's signature requests.
func (s *ESignService) ForDocument(ctx context.Context, userID, documentID string) ([]EnvelopeView, error) {
	doc, err := s.docs.GetFor(ctx, userID, "view", documentID)
	if err != nil {
		return nil, err
	}
	envelopes, err := queryRows[database.SignatureEnvelope](ctx, s.db,
		"SELECT * FROM signature_envelopes WHERE document_id = $doc ORDER BY created_at DESC", map[string]any{"doc": string(doc.ID)})
	if err != nil {
		return nil, err
	}
	canSign, _ := s.docs.access.CanDocument(ctx, userID, "sign", doc)
	return s.views(ctx, envelopes, canSign)
}

// Sent lists requests the user sent; Inbox lists those waiting on them.
func (s *ESignService) Sent(ctx context.Context, userID string) ([]EnvelopeView, error) {
	envelopes, err := queryRows[database.SignatureEnvelope](ctx, s.db,
		"SELECT * FROM signature_envelopes WHERE org_id = $org AND created_by = $user ORDER BY created_at DESC LIMIT 200",
		map[string]any{"org": orgOf(ctx), "user": userID})
	if err != nil {
		return nil, err
	}
	return s.views(ctx, envelopes, true)
}

func (s *ESignService) Inbox(ctx context.Context, userID string) ([]EnvelopeView, error) {
	actor, _ := ActorFrom(ctx)
	rows, err := queryRows[database.SignatureRecipient](ctx, s.db,
		"SELECT * FROM signature_recipients WHERE (user_id = $user OR email = $email) AND status IN ['sent', 'viewed'] ORDER BY created_at DESC LIMIT 200",
		map[string]any{"user": userID, "email": strings.ToLower(actor.Email)})
	if err != nil {
		return nil, err
	}
	var result []EnvelopeView
	for i := range rows {
		envelope, err := s.envelope(ctx, rows[i].EnvelopeID)
		if err != nil || envelope.Status != database.EnvelopeStatusSent {
			continue
		}
		view, err := s.view(ctx, envelope, false)
		if err != nil {
			continue
		}
		view.SigningLink = s.link(&rows[i])
		result = append(result, *view)
	}
	return result, nil
}

// Get returns one request with its fields and activity.
func (s *ESignService) Get(ctx context.Context, userID, id string) (*EnvelopeView, error) {
	envelope, manage, err := s.authorized(ctx, userID, id, false)
	if err != nil {
		return nil, err
	}
	view, err := s.view(ctx, envelope, manage)
	if err != nil {
		return nil, err
	}
	fields, err := s.fields(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		view.Fields = append(view.Fields, fieldView(&f, false))
	}
	events, err := queryRows[database.SignatureEvent](ctx, s.db, "SELECT * FROM signature_events WHERE envelope_id = $e ORDER BY created_at", map[string]any{"e": id})
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, r := range view.Recipients {
		names[r.ID] = r.Name
	}
	for _, e := range events {
		view.Events = append(view.Events, EventView{Action: e.Action, Actor: names[e.RecipientID], Detail: e.Detail, IP: e.IP, CreatedAt: e.CreatedAt})
	}
	return view, nil
}

// Void cancels an open request.
func (s *ESignService) Void(ctx context.Context, userID, id, reason string) error {
	envelope, _, err := s.authorized(ctx, userID, id, true)
	if err != nil {
		return err
	}
	if envelope.Status != database.EnvelopeStatusSent {
		return ErrEnvelopeClosed
	}
	now := time.Now().UTC()
	reason = truncate(strings.TrimSpace(reason), 500)
	if _, err := surrealdb.Merge[database.SignatureEnvelope](ctx, s.db, mustRecordID(string(envelope.ID)), map[string]any{
		"status": database.EnvelopeStatusVoided, "voided_at": now, "void_reason": reason, "updated_at": now,
	}); err != nil {
		return err
	}
	s.event(ctx, envelope, "", "voided", firstNonEmpty(reason, "Cancelled by the sender"), "", "")
	recipients, _ := s.recipients(ctx, id)
	for _, r := range recipients {
		if r.Status == database.RecipientStatusSent || r.Status == database.RecipientStatusViewed {
			body := fmt.Sprintf("The request to sign %q was cancelled; you don't need to do anything.\n", envelope.Title)
			if reason != "" {
				body += "\nReason: " + reason + "\n"
			}
			s.mail(ctx, envelope, r.Email, "Cancelled: "+envelope.Title, body)
		}
	}
	return nil
}

// Remind emails the signers whose turn it is again.
func (s *ESignService) Remind(ctx context.Context, userID, id string) (int, error) {
	envelope, _, err := s.authorized(ctx, userID, id, true)
	if err != nil {
		return 0, err
	}
	if envelope.Status != database.EnvelopeStatusSent {
		return 0, ErrEnvelopeClosed
	}
	recipients, err := s.recipients(ctx, id)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range s.due(envelope, recipients) {
		if r.Status == database.RecipientStatusSent || r.Status == database.RecipientStatusViewed {
			s.invite(ctx, envelope, &r)
			count++
		}
	}
	return count, nil
}

// Retry completes a request whose completion failed part-way.
func (s *ESignService) Retry(ctx context.Context, userID, id string) error {
	envelope, _, err := s.authorized(ctx, userID, id, true)
	if err != nil {
		return err
	}
	recipients, err := s.recipients(ctx, id)
	if err != nil {
		return err
	}
	if envelope.Status != database.EnvelopeStatusSent || len(s.due(envelope, recipients)) > 0 {
		return ErrEnvelopeClosed
	}
	return s.complete(ctx, envelope)
}

// authorized loads an envelope for its sender or someone who may sign the
// document; manage reports whether they may act on it.
func (s *ESignService) authorized(ctx context.Context, userID, id string, requireManage bool) (*database.SignatureEnvelope, bool, error) {
	envelope, err := s.envelope(ctx, id)
	if err != nil || envelope.OrgID != orgOf(ctx) {
		return nil, false, ErrEnvelopeNotFound
	}
	if envelope.CreatedBy == userID {
		return envelope, true, nil
	}
	doc, err := s.docs.GetFor(ctx, userID, "view", envelope.DocumentID)
	if err != nil {
		return nil, false, ErrEnvelopeNotFound
	}
	manage, _ := s.docs.access.CanDocument(ctx, userID, "sign", doc)
	if requireManage && !manage {
		return nil, false, ErrForbidden
	}
	return envelope, manage, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *ESignService) byToken(ctx context.Context, token string) (context.Context, *database.SignatureEnvelope, *database.SignatureRecipient, error) {
	if len(token) < 20 {
		return ctx, nil, nil, ErrEnvelopeNotFound
	}
	rows, err := queryRows[database.SignatureRecipient](ctx, s.db, "SELECT * FROM signature_recipients WHERE token_hash = $hash LIMIT 1", map[string]any{"hash": secretHash(token)})
	if err != nil {
		return ctx, nil, nil, err
	}
	if len(rows) == 0 {
		return ctx, nil, nil, ErrEnvelopeNotFound
	}
	recipient := &rows[0]
	envelope, err := s.envelope(ctx, recipient.EnvelopeID)
	if err != nil {
		return ctx, nil, nil, err
	}
	// Public requests act on behalf of the sender's organization.
	ctx = WithActor(ctx, Actor{UserID: envelope.CreatedBy, OrgID: envelope.OrgID})
	if envelope.Status == database.EnvelopeStatusSent && time.Now().After(envelope.ExpiresAt) {
		return ctx, nil, nil, ErrEnvelopeClosed
	}
	return ctx, envelope, recipient, nil
}

func (s *ESignService) envelope(ctx context.Context, id string) (*database.SignatureEnvelope, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrEnvelopeNotFound
	}
	envelope, err := surrealdb.Select[database.SignatureEnvelope](ctx, s.db, rid)
	if err != nil || envelope == nil || envelope.ID == "" {
		return nil, ErrEnvelopeNotFound
	}
	return envelope, nil
}

func (s *ESignService) recipients(ctx context.Context, envelopeID string) ([]database.SignatureRecipient, error) {
	rows, err := queryRows[database.SignatureRecipient](ctx, s.db, "SELECT * FROM signature_recipients WHERE envelope_id = $e", map[string]any{"e": envelopeID})
	sort.Slice(rows, func(i, j int) bool { return rows[i].Position < rows[j].Position })
	return rows, err
}

func (s *ESignService) fields(ctx context.Context, envelopeID string) ([]database.SignatureField, error) {
	return queryRows[database.SignatureField](ctx, s.db, "SELECT * FROM signature_fields WHERE envelope_id = $e ORDER BY page, y, x", map[string]any{"e": envelopeID})
}

func (s *ESignService) views(ctx context.Context, envelopes []database.SignatureEnvelope, withLinks bool) ([]EnvelopeView, error) {
	result := make([]EnvelopeView, 0, len(envelopes))
	for i := range envelopes {
		view, err := s.view(ctx, &envelopes[i], withLinks)
		if err != nil {
			return nil, err
		}
		result = append(result, *view)
	}
	return result, nil
}

func (s *ESignService) view(ctx context.Context, e *database.SignatureEnvelope, withLinks bool) (*EnvelopeView, error) {
	view := &EnvelopeView{
		ID: string(e.ID), DocumentID: e.DocumentID, Title: e.Title, Message: e.Message, Status: e.Status,
		Sequential: e.Sequential, RequireOTP: e.RequireOTP, Pages: e.Pages, CreatedBy: e.CreatedBy,
		SenderName: s.userName(ctx, e.CreatedBy), CompletedVersionID: e.CompletedVersionID, CompletedAt: e.CompletedAt,
		VoidReason: e.VoidReason, ExpiresAt: e.ExpiresAt, CreatedAt: e.CreatedAt, Recipients: []RecipientView{},
	}
	if view.Status == "completing" {
		view.Status = database.EnvelopeStatusSent
	}
	if doc, err := s.document(ctx, e.DocumentID); err == nil {
		view.DocumentName = doc.Name
	}
	recipients, err := s.recipients(ctx, string(e.ID))
	if err != nil {
		return nil, err
	}
	for i := range recipients {
		rv := recipientView(&recipients[i])
		if withLinks && e.Status == database.EnvelopeStatusSent {
			rv.Link = s.link(&recipients[i])
		}
		view.Recipients = append(view.Recipients, rv)
	}
	return view, nil
}

func recipientView(r *database.SignatureRecipient) RecipientView {
	return RecipientView{
		ID: string(r.ID), Name: r.Name, Email: r.Email, Role: r.Role, Position: r.Position, Status: r.Status,
		ViewedAt: r.ViewedAt, SignedAt: r.SignedAt, DeclinedAt: r.DeclinedAt, DeclineReason: r.DeclineReason,
	}
}

func fieldView(f *database.SignatureField, withValue bool) FieldView {
	view := FieldView{
		ID: string(f.ID), RecipientID: f.RecipientID, Kind: f.Kind, Page: f.Page, X: f.X, Y: f.Y, W: f.W, H: f.H,
		Required: f.Required, Label: f.Label, Filled: !f.FilledAt.IsZero(),
	}
	if withValue && f.Kind != database.FieldSignature && f.Kind != database.FieldInitials {
		view.Value = f.Value
	}
	return view
}

func fieldLabel(f *database.SignatureField) string {
	if f.Label != "" {
		return f.Label
	}
	return f.Kind
}

// token derives a recipient's link secret, so it can be shown again to its
// sender or a signed-in recipient without being stored.
func (s *ESignService) token(recipientID, nonce string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(recipientID + ":" + nonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *ESignService) link(r *database.SignatureRecipient) string {
	return s.appURL + "/sign/" + s.token(string(r.ID), r.Nonce)
}

func (s *ESignService) event(ctx context.Context, envelope *database.SignatureEnvelope, recipientID, action, detail, ip, agent string) {
	row := map[string]any{"org_id": envelope.OrgID, "envelope_id": string(envelope.ID), "action": action, "detail": detail}
	if recipientID != "" {
		row["recipient_id"] = recipientID
	}
	if ip != "" {
		row["ip"] = ip
	}
	if agent != "" {
		row["user_agent"] = truncate(agent, 300)
	}
	// The database stamps the time: events often land within one second and
	// the trail must keep their order.
	if _, err := surrealdb.Query[any](ctx, s.db, "CREATE signature_events CONTENT object::extend($row, { created_at: time::now() }) RETURN NONE", map[string]any{"row": row}); err != nil {
		s.log.Warn("record signature event", "envelope_id", string(envelope.ID), "error", err)
	}
}

func (s *ESignService) mail(ctx context.Context, envelope *database.SignatureEnvelope, to, subject, body string) {
	if s.email == nil {
		return
	}
	if orgOf(ctx) == "" {
		ctx = WithActor(ctx, Actor{UserID: envelope.CreatedBy, OrgID: envelope.OrgID})
	}
	if _, err := s.email.QueueEmail(ctx, envelope.CreatedBy, to, subject, body); err != nil {
		s.log.Warn("queue signing email", "to", to, "error", err)
	}
}

func (s *ESignService) read(ctx context.Context, storagePath string, limit int64) ([]byte, error) {
	body, err := s.storage.Download(ctx, storagePath)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(io.LimitReader(body, limit))
}

func (s *ESignService) userName(ctx context.Context, userID string) string {
	if user, err := (&OrganizationService{db: s.db}).userByID(ctx, userID); err == nil && user != nil {
		return firstNonEmpty(user.Name, user.Email)
	}
	return "Someone"
}

func (s *ESignService) userEmail(ctx context.Context, userID string) string {
	if user, err := (&OrganizationService{db: s.db}).userByID(ctx, userID); err == nil && user != nil {
		return user.Email
	}
	return ""
}

// memberByEmail links a recipient to an account in the organization, so the
// request shows up in their inbox.
func (s *ESignService) memberByEmail(ctx context.Context, orgID, email string) (string, error) {
	users, err := queryRows[database.User](ctx, s.db, "SELECT * FROM users WHERE email = $email LIMIT 1", map[string]any{"email": email})
	if err != nil || len(users) == 0 {
		return "", err
	}
	members, err := queryRows[database.Membership](ctx, s.db, "SELECT * FROM memberships WHERE org_id = $org AND user_id = $user AND status = 'active' LIMIT 1",
		map[string]any{"org": orgID, "user": string(users[0].ID)})
	if err != nil || len(members) == 0 {
		return "", err
	}
	return string(users[0].ID), nil
}

func decodeSignatureImage(value string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) {
		return nil, errors.New("draw or type your signature")
	}
	data, err := base64.StdEncoding.DecodeString(value[len(prefix):])
	if err != nil || len(data) == 0 {
		return nil, errors.New("the signature image could not be read")
	}
	if len(data) > maxSignatureImage {
		return nil, errors.New("the signature image is too large")
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return nil, errors.New("the signature must be a PNG image")
	}
	return data, nil
}

func randomKey() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// recordKey is the part of a record ID after the table name.
func recordKey(id string) string {
	if _, key, ok := strings.Cut(id, ":"); ok {
		return strings.Trim(key, "⟨⟩`")
	}
	return id
}

func prefixed(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

// document loads a document without an access check.
func (s *ESignService) document(ctx context.Context, id string) (*database.Document, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, ErrDocumentNotFound
	}
	doc, err := surrealdb.Select[database.Document](ctx, s.db, rid)
	if err != nil || doc == nil || doc.ID == "" {
		return nil, ErrDocumentNotFound
	}
	return doc, nil
}
