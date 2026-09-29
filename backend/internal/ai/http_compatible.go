package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPCompatibleProvider speaks a small, widely implemented JSON protocol.
// Hosted gateways and local inference servers can both expose it, so the core
// application never imports a vendor SDK.
type HTTPCompatibleProvider struct {
	baseURL, apiKey, chatModel, embeddingModel string
	client                                     *http.Client
}

func NewHTTPCompatibleProvider(baseURL, apiKey, chatModel, embeddingModel string, client *http.Client) *HTTPCompatibleProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPCompatibleProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, chatModel: chatModel, embeddingModel: embeddingModel, client: client}
}

func (p *HTTPCompatibleProvider) Classify(ctx context.Context, document DocumentContext) (Classification, error) {
	answer, err := p.chat(ctx, "Return only JSON with category, confidence (0-1), fields, and reasoning. Classify this office document.\nName: "+document.Name+"\nText:\n"+document.Text)
	if err != nil {
		return Classification{}, err
	}
	var result Classification
	clean := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(answer), "```json"), "```"))
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return Classification{}, fmt.Errorf("AI returned invalid classification JSON: %w", err)
	}
	return result, nil
}

func (p *HTTPCompatibleProvider) Ask(ctx context.Context, question string, documents []DocumentContext) (Answer, error) {
	var prompt strings.Builder
	prompt.WriteString("Answer using only the supplied office documents. Cite sources as [document-id]. If the answer is absent, say so.\nQuestion: ")
	prompt.WriteString(question)
	for i := range documents {
		prompt.WriteString("\n\nDocument ")
		prompt.WriteString(documents[i].DocumentID)
		prompt.WriteString(" — ")
		prompt.WriteString(documents[i].Name)
		prompt.WriteString("\n")
		prompt.WriteString(documents[i].Text)
	}
	text, err := p.chat(ctx, prompt.String())
	if err != nil {
		return Answer{}, err
	}
	citations := []string{}
	for i := range documents {
		if strings.Contains(text, documents[i].DocumentID) {
			citations = append(citations, documents[i].DocumentID)
		}
	}
	return Answer{Text: text, Citations: citations}, nil
}

func (p *HTTPCompatibleProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{"model": p.embeddingModel, "input": texts}
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := p.post(ctx, "/embeddings", payload, &response); err != nil {
		return nil, err
	}
	items := make([][]float32, 0, len(response.Data))
	for i := range response.Data {
		items = append(items, response.Data[i].Embedding)
	}
	return items, nil
}

func (p *HTTPCompatibleProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return p.chat(ctx, prompt)
}

func (p *HTTPCompatibleProvider) chat(ctx context.Context, prompt string) (string, error) {
	payload := map[string]any{"model": p.chatModel, "messages": []map[string]string{{"role": "user", "content": prompt}}, "temperature": 0.1}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := p.post(ctx, "/chat/completions", payload, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("AI returned no answer")
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func (p *HTTPCompatibleProvider) post(ctx context.Context, path string, payload any, output any) error {
	if p.baseURL == "" {
		return ErrDisabled
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("AI request failed (%d): %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return json.NewDecoder(response.Body).Decode(output)
}
