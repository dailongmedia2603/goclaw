//go:build !sqliteonly

package fbcloak

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// PlanLLM is the abstraction Generator uses to call any LLM provider with
// a system prompt + user prompt. Production wraps providers.Provider; tests
// inject a fake.
type PlanLLM interface {
	Generate(ctx context.Context, in PlanLLMInput) (PlanLLMOutput, error)
}

type PlanLLMInput struct {
	SystemPrompt string  // may be cached by Anthropic adapter when paired with cache boundary marker
	UserPrompt   string  // fresh per call
	Model        string  // empty → provider's DefaultModel()
	MaxTokens    int     // default 800
	Temperature  float32 // default 0.4
}

type PlanLLMOutput struct {
	Text         string
	Model        string
	InputTokens  int
	OutputTokens int
	CachedTokens int
}

// providerLLM bridges providers.Provider into PlanLLM. Anthropic adapter
// inserts cache_control at the providers.CacheBoundaryMarker boundary; the
// system prompt becomes a single cached block, the user prompt fresh.
// Other providers ignore the marker and the request still works (cache miss).
type providerLLM struct {
	p providers.Provider
}

// NewPlanLLM wraps an existing providers.Provider for use by the Generator.
func NewPlanLLM(p providers.Provider) PlanLLM {
	return &providerLLM{p: p}
}

func (l *providerLLM) Generate(ctx context.Context, in PlanLLMInput) (PlanLLMOutput, error) {
	maxTokens := in.MaxTokens
	if maxTokens == 0 {
		maxTokens = 800
	}
	temp := in.Temperature
	if temp == 0 {
		temp = 0.4
	}

	// Combined message body so Anthropic adapter sees one user message that
	// gets split at CacheBoundaryMarker into cached + fresh blocks.
	body := strings.TrimSpace(in.SystemPrompt) + "\n" + providers.CacheBoundaryMarker + "\n" + strings.TrimSpace(in.UserPrompt)

	resp, err := l.p.Chat(ctx, providers.ChatRequest{
		Model: in.Model,
		Messages: []providers.Message{
			{Role: "user", Content: body},
		},
		Options: map[string]any{
			"max_tokens":  maxTokens,
			"temperature": temp,
		},
	})
	if err != nil {
		return PlanLLMOutput{}, fmt.Errorf("provider chat: %w", err)
	}

	out := PlanLLMOutput{
		Text:  resp.Content,
		Model: in.Model,
	}
	if out.Model == "" {
		out.Model = l.p.DefaultModel()
	}
	if resp.Usage != nil {
		out.InputTokens = resp.Usage.PromptTokens
		out.OutputTokens = resp.Usage.CompletionTokens
		out.CachedTokens = resp.Usage.CacheReadTokens
	}
	return out, nil
}
