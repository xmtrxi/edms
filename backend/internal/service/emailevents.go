package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/observability"
)

var (
	ErrEmailEventsDisabled  = errors.New("email delivery callbacks are not configured")
	ErrEmailEventsSignature = errors.New("callback signature is invalid")
)

// EmailEvent is one delivery outcome reported by the mail provider.
type EmailEvent struct {
	DeliveryID string
	MessageID  string
	// Status is delivered, bounced, complained or failed. Empty means the
	// event is informational, such as a temporary deferral.
	Status string
	Detail string
}

// emailEventMaxAge rejects replayed callbacks.
const emailEventMaxAge = 15 * time.Minute

// ParseEmailEvents verifies a provider callback and reads its events.
func ParseEmailEvents(cfg config.EmailEventsConfig, provider string, r *http.Request, body []byte, now time.Time) ([]EmailEvent, error) {
	if cfg.Provider == "" || provider != cfg.Provider {
		return nil, ErrEmailEventsDisabled
	}
	switch provider {
	case "mailgun":
		return parseMailgun(cfg.Secret, body, now)
	case "postmark":
		user, password, ok := r.BasicAuth()
		if !ok || cfg.Secret == "" || !constantEqual(user, cfg.User) || !constantEqual(password, cfg.Secret) {
			return nil, ErrEmailEventsSignature
		}
		return parsePostmark(body)
	case "sendgrid":
		if err := verifySendGrid(cfg.PublicKey, r.Header.Get("X-Twilio-Email-Event-Webhook-Signature"), r.Header.Get("X-Twilio-Email-Event-Webhook-Timestamp"), body, now); err != nil {
			return nil, err
		}
		return parseSendGrid(body)
	case "generic":
		if err := verifyGeneric(cfg.Secret, r.Header.Get("X-EDMS-Signature"), r.Header.Get("X-EDMS-Timestamp"), body, now); err != nil {
			return nil, err
		}
		return parseGeneric(body)
	}
	return nil, ErrEmailEventsDisabled
}

func parseMailgun(key string, body []byte, now time.Time) ([]EmailEvent, error) {
	var payload struct {
		Signature struct {
			Timestamp string `json:"timestamp"`
			Token     string `json:"token"`
			Signature string `json:"signature"`
		} `json:"signature"`
		Event struct {
			Event          string            `json:"event"`
			Severity       string            `json:"severity"`
			Reason         string            `json:"reason"`
			UserVariables  map[string]string `json:"user-variables"`
			DeliveryStatus struct {
				Message     string `json:"message"`
				Description string `json:"description"`
			} `json:"delivery-status"`
			Message struct {
				Headers struct {
					MessageID string `json:"message-id"`
				} `json:"headers"`
			} `json:"message"`
		} `json:"event-data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read mailgun event: %w", err)
	}
	sig := payload.Signature
	if key == "" || !freshTimestamp(sig.Timestamp, now) {
		return nil, ErrEmailEventsSignature
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(sig.Timestamp + sig.Token))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(strings.ToLower(sig.Signature))) {
		return nil, ErrEmailEventsSignature
	}
	e := payload.Event
	event := EmailEvent{
		DeliveryID: e.UserVariables["edms_delivery_id"], MessageID: e.Message.Headers.MessageID,
		Detail: firstNonEmpty(e.DeliveryStatus.Description, e.DeliveryStatus.Message, e.Reason),
	}
	switch e.Event {
	case "delivered":
		event.Status = "delivered"
	case "failed":
		if e.Severity == "permanent" {
			event.Status = "bounced"
		}
	case "complained":
		event.Status = "complained"
	case "rejected":
		event.Status = "failed"
	}
	return []EmailEvent{event}, nil
}

func parsePostmark(body []byte) ([]EmailEvent, error) {
	var payload struct {
		RecordType  string            `json:"RecordType"`
		Type        string            `json:"Type"`
		Description string            `json:"Description"`
		Details     string            `json:"Details"`
		Metadata    map[string]string `json:"Metadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read postmark event: %w", err)
	}
	event := EmailEvent{DeliveryID: payload.Metadata["edms_delivery_id"], Detail: firstNonEmpty(payload.Description, payload.Details)}
	switch payload.RecordType {
	case "Delivery":
		event.Status = "delivered"
	case "Bounce":
		// Soft bounces and transient failures are retried by Postmark.
		switch payload.Type {
		case "Transient", "SoftBounce", "DnsError", "AutoResponder", "OpenRelayTest", "Unknown":
		default:
			event.Status = "bounced"
		}
	case "SpamComplaint":
		event.Status = "complained"
	}
	return []EmailEvent{event}, nil
}

func verifySendGrid(publicKey, signature, timestamp string, body []byte, now time.Time) error {
	if publicKey == "" || signature == "" || !freshTimestamp(timestamp, now) {
		return ErrEmailEventsSignature
	}
	der, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return ErrEmailEventsSignature
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return ErrEmailEventsSignature
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return ErrEmailEventsSignature
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return ErrEmailEventsSignature
	}
	digest := sha256.Sum256(append([]byte(timestamp), body...))
	if !ecdsa.VerifyASN1(key, digest[:], sig) {
		return ErrEmailEventsSignature
	}
	return nil
}

func parseSendGrid(body []byte) ([]EmailEvent, error) {
	var payload []struct {
		Event         string `json:"event"`
		Type          string `json:"type"`
		Reason        string `json:"reason"`
		Response      string `json:"response"`
		DeliveryID    string `json:"edms_delivery_id"`
		SMTPMessageID string `json:"smtp-id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read sendgrid events: %w", err)
	}
	events := make([]EmailEvent, 0, len(payload))
	for _, e := range payload {
		event := EmailEvent{DeliveryID: e.DeliveryID, MessageID: e.SMTPMessageID, Detail: firstNonEmpty(e.Reason, e.Response)}
		switch e.Event {
		case "delivered":
			event.Status = "delivered"
		case "bounce":
			// SendGrid marks soft bounces as "blocked"; those are retried.
			if e.Type != "blocked" {
				event.Status = "bounced"
			}
		case "dropped":
			event.Status = "failed"
		case "spamreport":
			event.Status = "complained"
		}
		events = append(events, event)
	}
	return events, nil
}

func verifyGeneric(secret, signature, timestamp string, body []byte, now time.Time) error {
	if secret == "" || !freshTimestamp(timestamp, now) {
		return ErrEmailEventsSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(strings.ToLower(strings.TrimPrefix(signature, "sha256=")))) {
		return ErrEmailEventsSignature
	}
	return nil
}

// parseGeneric reads {"delivery_id"|"message_id", "status", "detail"} or a
// list of them, for relays that translate their own provider's events.
func parseGeneric(body []byte) ([]EmailEvent, error) {
	type item struct {
		DeliveryID string `json:"delivery_id"`
		MessageID  string `json:"message_id"`
		Status     string `json:"status"`
		Detail     string `json:"detail"`
	}
	var list []item
	if err := json.Unmarshal(body, &list); err != nil {
		var one item
		if err := json.Unmarshal(body, &one); err != nil {
			return nil, fmt.Errorf("read email event: %w", err)
		}
		list = []item{one}
	}
	events := make([]EmailEvent, 0, len(list))
	for _, e := range list {
		switch e.Status {
		case "delivered", "bounced", "complained", "failed", "":
		default:
			return nil, fmt.Errorf("unknown email status %q", e.Status)
		}
		events = append(events, EmailEvent{DeliveryID: e.DeliveryID, MessageID: e.MessageID, Status: e.Status, Detail: e.Detail})
	}
	return events, nil
}

// freshTimestamp accepts Unix seconds within emailEventMaxAge of now.
func freshTimestamp(value string, now time.Time) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return false
	}
	return math.Abs(now.Sub(time.Unix(seconds, 0)).Seconds()) <= emailEventMaxAge.Seconds()
}

func constantEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ApplyEmailEvents records outcomes on the matching deliveries and returns
// how many matched.
func (s *OperationsService) ApplyEmailEvents(ctx context.Context, events []EmailEvent) (int, error) {
	matched := 0
	for _, event := range events {
		item, err := s.findDelivery(ctx, event)
		if err != nil {
			return matched, err
		}
		if item == nil {
			continue
		}
		matched++
		now := time.Now().UTC()
		updates := map[string]any{"updated_at": now}
		if event.Detail != "" {
			updates["last_error"] = truncate(event.Detail, 500)
		}
		if event.Status != "" {
			observability.Emails.WithLabelValues(event.Status).Inc()
			updates["status"] = event.Status
			if event.Status == "delivered" {
				updates["delivered_at"] = now
				updates["last_error"] = ""
			}
		}
		if _, err := surrealdb.Merge[database.EmailDelivery](ctx, s.db, mustRecordID(string(item.ID)), updates); err != nil {
			return matched, err
		}
	}
	return matched, nil
}

func (s *OperationsService) findDelivery(ctx context.Context, event EmailEvent) (*database.EmailDelivery, error) {
	if event.DeliveryID != "" {
		rid, err := database.ParseRecordID(event.DeliveryID)
		if err == nil && rid.Table == "email_deliveries" {
			item, err := surrealdb.Select[database.EmailDelivery](ctx, s.db, rid)
			if err == nil && item != nil && item.ID != "" {
				return item, nil
			}
		}
	}
	messageID := strings.TrimSpace(event.MessageID)
	if messageID == "" {
		return nil, nil
	}
	if !strings.HasPrefix(messageID, "<") {
		messageID = "<" + messageID + ">"
	}
	rows, err := queryRows[database.EmailDelivery](ctx, s.db, "SELECT * FROM email_deliveries WHERE message_id = $id LIMIT 1", map[string]any{"id": messageID})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
