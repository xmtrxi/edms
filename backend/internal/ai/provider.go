// Package ai defines the application-owned boundary for optional AI features.
// Vendor SDKs and protocols belong in adapters, never in document services.
package ai

import "context"

type DocumentContext struct {
	DocumentID string
	Name       string
	Text       string
	Metadata   map[string]any
}

type Classification struct {
	Category   string         `json:"category"`
	Confidence float64        `json:"confidence"`
	Fields     map[string]any `json:"fields,omitempty"`
	Reasoning  string         `json:"reasoning,omitempty"`
}

type Answer struct {
	Text      string   `json:"text"`
	Citations []string `json:"citations,omitempty"`
}

// Provider is intentionally small so an office can use a hosted model, a
// local model, or disable AI without changing workflow and storage code.
type Provider interface {
	Classify(context.Context, DocumentContext) (Classification, error)
	Ask(context.Context, string, []DocumentContext) (Answer, error)
	Embed(context.Context, []string) ([][]float32, error)
	// Complete answers a single prompt; callers own the prompt wording.
	Complete(context.Context, string) (string, error)
}

type DisabledProvider struct{}

func (DisabledProvider) Classify(context.Context, DocumentContext) (Classification, error) {
	return Classification{}, ErrDisabled
}

func (DisabledProvider) Ask(context.Context, string, []DocumentContext) (Answer, error) {
	return Answer{}, ErrDisabled
}

func (DisabledProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, ErrDisabled
}

func (DisabledProvider) Complete(context.Context, string) (string, error) {
	return "", ErrDisabled
}
