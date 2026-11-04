package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidWebhookSignature(t *testing.T) {
	body := []byte(`{"event":"document.completed"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !validWebhookSignature(body, signature, "secret") {
		t.Fatal("expected valid signature")
	}
	if validWebhookSignature([]byte(`{"event":"changed"}`), signature, "secret") {
		t.Fatal("expected changed body to fail verification")
	}
}

func TestFirstWebhookStringReadsNestedPayload(t *testing.T) {
	payload := map[string]any{"payload": map[string]any{"documentId": float64(42)}}
	if got := firstWebhookString(payload, "documentId"); got != "42" {
		t.Fatalf("expected nested document id, got %q", got)
	}
}
