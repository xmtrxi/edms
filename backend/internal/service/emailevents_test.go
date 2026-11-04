package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var eventsNow = time.Unix(1_800_000_000, 0)

func TestMailgunEvents(t *testing.T) {
	cfg := config.EmailEventsConfig{Provider: "mailgun", Secret: "signing-key"}
	ts := strconv.FormatInt(eventsNow.Unix(), 10)
	mac := hmac.New(sha256.New, []byte("signing-key"))
	mac.Write([]byte(ts + "tok"))
	body := `{"signature":{"timestamp":"` + ts + `","token":"tok","signature":"` + hex.EncodeToString(mac.Sum(nil)) + `"},
		"event-data":{"event":"failed","severity":"permanent","user-variables":{"edms_delivery_id":"email_deliveries:1"},
		"delivery-status":{"description":"mailbox does not exist"},"message":{"headers":{"message-id":"abc@example.com"}}}}`
	req := httptest.NewRequest("POST", "/", nil)

	events, err := ParseEmailEvents(cfg, "mailgun", req, []byte(body), eventsNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Status != "bounced" || events[0].DeliveryID != "email_deliveries:1" || events[0].Detail != "mailbox does not exist" {
		t.Fatalf("unexpected events %+v", events)
	}
	if _, err := ParseEmailEvents(cfg, "mailgun", req, []byte(strings.Replace(body, "tok", "forged", 1)), eventsNow); !errors.Is(err, ErrEmailEventsSignature) {
		t.Fatalf("forged token accepted: %v", err)
	}
	if _, err := ParseEmailEvents(cfg, "mailgun", req, []byte(body), eventsNow.Add(time.Hour)); !errors.Is(err, ErrEmailEventsSignature) {
		t.Fatalf("stale callback accepted: %v", err)
	}
	if _, err := ParseEmailEvents(cfg, "postmark", req, []byte(body), eventsNow); !errors.Is(err, ErrEmailEventsDisabled) {
		t.Fatalf("other providers should be refused, got %v", err)
	}
}

func TestPostmarkEventsNeedBasicAuth(t *testing.T) {
	cfg := config.EmailEventsConfig{Provider: "postmark", User: "edms", Secret: "pw"}
	body := []byte(`{"RecordType":"Bounce","Type":"HardBounce","Description":"Unknown user","Metadata":{"edms_delivery_id":"email_deliveries:9"}}`)
	req := httptest.NewRequest("POST", "/", nil)
	if _, err := ParseEmailEvents(cfg, "postmark", req, body, eventsNow); !errors.Is(err, ErrEmailEventsSignature) {
		t.Fatalf("missing credentials accepted: %v", err)
	}
	req.SetBasicAuth("edms", "pw")
	events, err := ParseEmailEvents(cfg, "postmark", req, body, eventsNow)
	if err != nil || events[0].Status != "bounced" || events[0].DeliveryID != "email_deliveries:9" {
		t.Fatalf("got %+v, %v", events, err)
	}
	soft, _ := ParseEmailEvents(cfg, "postmark", req, []byte(`{"RecordType":"Bounce","Type":"SoftBounce"}`), eventsNow)
	if soft[0].Status != "" {
		t.Fatalf("soft bounces should not change the status, got %q", soft[0].Status)
	}
}

func TestSendGridSignedEvents(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	cfg := config.EmailEventsConfig{Provider: "sendgrid", PublicKey: base64.StdEncoding.EncodeToString(der)}
	body := []byte(`[{"event":"delivered","edms_delivery_id":"email_deliveries:2"},{"event":"bounce","type":"blocked"},{"event":"dropped","reason":"Bounced Address"}]`)
	ts := strconv.FormatInt(eventsNow.Unix(), 10)
	digest := sha256.Sum256(append([]byte(ts), body...))
	sig, _ := ecdsa.SignASN1(rand.Reader, key, digest[:])

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Twilio-Email-Event-Webhook-Signature", base64.StdEncoding.EncodeToString(sig))
	req.Header.Set("X-Twilio-Email-Event-Webhook-Timestamp", ts)
	events, err := ParseEmailEvents(cfg, "sendgrid", req, body, eventsNow)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Status != "delivered" || events[1].Status != "" || events[2].Status != "failed" {
		t.Fatalf("unexpected statuses %+v", events)
	}
	if _, err := ParseEmailEvents(cfg, "sendgrid", req, append(body, ' '), eventsNow); !errors.Is(err, ErrEmailEventsSignature) {
		t.Fatalf("tampered body accepted: %v", err)
	}
}

func TestGenericEvents(t *testing.T) {
	cfg := config.EmailEventsConfig{Provider: "generic", Secret: "s3cret"}
	body := []byte(`{"message_id":"<a@b>","status":"complained"}`)
	ts := strconv.FormatInt(eventsNow.Unix(), 10)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-EDMS-Timestamp", ts)
	req.Header.Set("X-EDMS-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	events, err := ParseEmailEvents(cfg, "generic", req, body, eventsNow)
	if err != nil || events[0].Status != "complained" || events[0].MessageID != "<a@b>" {
		t.Fatalf("got %+v, %v", events, err)
	}
	req.Header.Set("X-EDMS-Signature", "sha256=00")
	if _, err := ParseEmailEvents(cfg, "generic", req, body, eventsNow); !errors.Is(err, ErrEmailEventsSignature) {
		t.Fatalf("bad signature accepted: %v", err)
	}
}

func TestBuildMessageHeaders(t *testing.T) {
	item := &database.EmailDelivery{ID: "email_deliveries:7", Recipient: "ada@example.com", Subject: "Café\r\nBcc: evil@example.com", Body: "line one\nline two"}
	msg := string(buildMessage(item, "EDMS <no-reply@example.com>", "<id@example.com>", "mailgun", eventsNow))
	head, body, _ := strings.Cut(msg, "\r\n\r\n")
	for _, want := range []string{
		"From: EDMS <no-reply@example.com>", "To: ada@example.com", "Message-ID: <id@example.com>",
		"Date: ", "X-EDMS-Delivery-ID: email_deliveries:7", `X-Mailgun-Variables: {"edms_delivery_id":"email_deliveries:7"}`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("missing header %q in\n%s", want, head)
		}
	}
	if strings.Contains(head, "\r\nBcc:") {
		t.Fatal("subject allowed a header injection")
	}
	if !strings.Contains(head, "Subject: =?utf-8?q?Caf=C3=A9") {
		t.Errorf("non-ASCII subject not encoded:\n%s", head)
	}
	if body != "line one\r\nline two" {
		t.Errorf("body line endings not normalised: %q", body)
	}
	if got := envelopeAddress("EDMS <no-reply@example.com>"); got != "no-reply@example.com" {
		t.Errorf("envelope address %q", got)
	}
}
