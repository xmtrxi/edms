package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

var (
	_ Provider = (*HTTPCompatibleProvider)(nil)
	_ Provider = DisabledProvider{}
)

type recordedRequest struct {
	Method, Path, Auth, ContentType string
	Body                            map[string]any
}

// fakeServer answers every request with status and body, recording what it received.
func fakeServer(t *testing.T, status int, body string) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := recordedRequest{Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"), ContentType: r.Header.Get("Content-Type")}
		if err := json.NewDecoder(r.Body).Decode(&request.Body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		requests = append(requests, request)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func chatReply(content string) string {
	payload, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}}})
	return string(payload)
}

func TestCompleteSendsChatRequest(t *testing.T) {
	server, requests := fakeServer(t, http.StatusOK, chatReply("  Hello there \n"))
	provider := NewHTTPCompatibleProvider(server.URL+"/v1/", "sk-test", "chat-model", "embed-model", server.Client())

	answer, err := provider.Complete(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if answer != "Hello there" {
		t.Fatalf("answer should be trimmed, got %q", answer)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one request, got %d", len(*requests))
	}
	got := (*requests)[0]
	if got.Method != http.MethodPost || got.Path != "/v1/chat/completions" {
		t.Fatalf("unexpected request %s %s", got.Method, got.Path)
	}
	if got.Auth != "Bearer sk-test" || got.ContentType != "application/json" {
		t.Fatalf("unexpected headers auth=%q content-type=%q", got.Auth, got.ContentType)
	}
	if got.Body["model"] != "chat-model" || got.Body["temperature"] != 0.1 {
		t.Fatalf("unexpected body %v", got.Body)
	}
	messages, _ := got.Body["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %v", got.Body["messages"])
	}
	message, _ := messages[0].(map[string]any)
	if message["role"] != "user" || message["content"] != "Say hello" {
		t.Fatalf("unexpected message %v", message)
	}
}

func TestNoAuthorizationHeaderWithoutAPIKey(t *testing.T) {
	server, requests := fakeServer(t, http.StatusOK, chatReply("ok"))
	if _, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Complete(context.Background(), "x"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if auth := (*requests)[0].Auth; auth != "" {
		t.Fatalf("expected no Authorization header for a local server, got %q", auth)
	}
}

func TestEmbed(t *testing.T) {
	server, requests := fakeServer(t, http.StatusOK, `{"data":[{"index":0,"embedding":[0.1,0.2]},{"index":1,"embedding":[0.3]}]}`)
	provider := NewHTTPCompatibleProvider(server.URL, "key", "chat-model", "embed-model", server.Client())

	vectors, err := provider.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vectors) != 2 || !slices.Equal(vectors[0], []float32{0.1, 0.2}) || !slices.Equal(vectors[1], []float32{0.3}) {
		t.Fatalf("unexpected vectors %v", vectors)
	}
	got := (*requests)[0]
	if got.Path != "/embeddings" || got.Body["model"] != "embed-model" {
		t.Fatalf("unexpected request %s %v", got.Path, got.Body)
	}
	input, _ := got.Body["input"].([]any)
	if len(input) != 2 || input[0] != "first" || input[1] != "second" {
		t.Fatalf("unexpected input %v", got.Body["input"])
	}
}

func TestEmbedEmptyResponse(t *testing.T) {
	server, _ := fakeServer(t, http.StatusOK, `{"data":[]}`)
	vectors, err := NewHTTPCompatibleProvider(server.URL, "", "", "", nil).Embed(context.Background(), []string{"x"})
	if err != nil || vectors == nil || len(vectors) != 0 {
		t.Fatalf("expected empty non-nil result, got %v, %v", vectors, err)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		want    Classification
		wantErr bool
	}{
		{
			name:  "plain JSON",
			reply: `{"category":"invoice","confidence":0.92,"fields":{"total":"120.00"},"reasoning":"has totals"}`,
			want:  Classification{Category: "invoice", Confidence: 0.92, Fields: map[string]any{"total": "120.00"}, Reasoning: "has totals"},
		},
		{
			name:  "json code fence",
			reply: "```json\n{\"category\":\"contract\",\"confidence\":0.5}\n```",
			want:  Classification{Category: "contract", Confidence: 0.5},
		},
		{name: "prose", reply: "This looks like an invoice.", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, requests := fakeServer(t, http.StatusOK, chatReply(tc.reply))
			provider := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil)
			got, err := provider.Classify(context.Background(), DocumentContext{DocumentID: "document:1", Name: "inv.pdf", Text: "Total due 120.00"})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "invalid classification JSON") {
					t.Fatalf("expected invalid JSON error, got %+v, %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got.Category != tc.want.Category || got.Confidence != tc.want.Confidence || got.Reasoning != tc.want.Reasoning || len(got.Fields) != len(tc.want.Fields) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for key, value := range tc.want.Fields {
				if got.Fields[key] != value {
					t.Fatalf("field %q = %v, want %v", key, got.Fields[key], value)
				}
			}
			prompt := (*requests)[0].Body["messages"].([]any)[0].(map[string]any)["content"].(string)
			for _, want := range []string{"Return only JSON", "Name: inv.pdf", "Total due 120.00"} {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt missing %q:\n%s", want, prompt)
				}
			}
		})
	}
}

func TestClassifyAcceptsBareCodeFence(t *testing.T) {
	server, _ := fakeServer(t, http.StatusOK, chatReply("```\n{\"category\":\"memo\",\"confidence\":0.7}\n```"))
	got, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Classify(context.Background(), DocumentContext{Name: "a"})
	if err != nil || got.Category != "memo" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestAskBuildsPromptAndCitations(t *testing.T) {
	server, requests := fakeServer(t, http.StatusOK, chatReply("The budget is 5M [document:2]."))
	provider := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil)
	docs := []DocumentContext{
		{DocumentID: "document:1", Name: "Minutes.docx", Text: "Meeting notes"},
		{DocumentID: "document:2", Name: "Budget.xlsx", Text: "Total 5M"},
	}
	answer, err := provider.Ask(context.Background(), "What is the budget?", docs)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer.Text != "The budget is 5M [document:2]." {
		t.Fatalf("unexpected text %q", answer.Text)
	}
	if !slices.Equal(answer.Citations, []string{"document:2"}) {
		t.Fatalf("unexpected citations %v", answer.Citations)
	}
	prompt := (*requests)[0].Body["messages"].([]any)[0].(map[string]any)["content"].(string)
	for _, want := range []string{"Question: What is the budget?", "Document document:1 — Minutes.docx\nMeeting notes", "Document document:2 — Budget.xlsx\nTotal 5M"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestAskWithoutCitationsReturnsEmptySlice(t *testing.T) {
	server, _ := fakeServer(t, http.StatusOK, chatReply("Not in the documents."))
	answer, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Ask(context.Background(), "q", []DocumentContext{{DocumentID: "document:1"}})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer.Citations == nil || len(answer.Citations) != 0 {
		t.Fatalf("expected empty non-nil citations, got %#v", answer.Citations)
	}
}

func TestAskCitationPrefixCollision(t *testing.T) {
	server, _ := fakeServer(t, http.StatusOK, chatReply("See [document:10]."))
	answer, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Ask(context.Background(), "q",
		[]DocumentContext{{DocumentID: "document:1"}, {DocumentID: "document:10"}})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if !slices.Equal(answer.Citations, []string{"document:10"}) {
		t.Fatalf("unexpected citations %v", answer.Citations)
	}
}

func TestHTTPErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"server error", http.StatusInternalServerError, "  upstream exploded \n", "AI request failed (500): upstream exploded"},
		{"unauthorized", http.StatusUnauthorized, `{"error":"bad key"}`, `AI request failed (401): {"error":"bad key"}`},
		{"no choices", http.StatusOK, `{"choices":[]}`, "AI returned no answer"},
		{"malformed JSON", http.StatusOK, `not json`, "invalid character"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := fakeServer(t, tc.status, tc.body)
			_, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Complete(context.Background(), "x")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestErrorBodyIsLimited(t *testing.T) {
	server, _ := fakeServer(t, http.StatusBadGateway, strings.Repeat("x", 10000))
	_, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Complete(context.Background(), "x")
	if err == nil || len(err.Error()) > 4096+64 {
		t.Fatalf("expected error body capped at 4096 bytes, got %d bytes", len(err.Error()))
	}
}

func TestUnconfiguredProviderIsDisabled(t *testing.T) {
	provider := NewHTTPCompatibleProvider("", "key", "m", "e", nil)
	ctx := context.Background()
	if _, err := provider.Complete(ctx, "x"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := provider.Embed(ctx, []string{"x"}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Embed: %v", err)
	}
	if _, err := provider.Classify(ctx, DocumentContext{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Classify: %v", err)
	}
	if _, err := provider.Ask(ctx, "q", nil); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Ask: %v", err)
	}
}

func TestCancelledContext(t *testing.T) {
	server, _ := fakeServer(t, http.StatusOK, chatReply("ok"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewHTTPCompatibleProvider(server.URL, "", "m", "e", nil).Complete(ctx, "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDisabledProvider(t *testing.T) {
	var provider DisabledProvider
	ctx := context.Background()
	if _, err := provider.Classify(ctx, DocumentContext{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Classify: %v", err)
	}
	if _, err := provider.Ask(ctx, "q", nil); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Ask: %v", err)
	}
	if _, err := provider.Embed(ctx, nil); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Embed: %v", err)
	}
	if _, err := provider.Complete(ctx, "x"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Complete: %v", err)
	}
}
