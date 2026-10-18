package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
)

const (
	groqBaseURL   = "https://api.groq.com/openai/v1/chat/completions"
	openaiBaseURL = "https://api.openai.com/v1/chat/completions"
	ollamaBaseURL = "http://localhost:11434/v1/chat/completions"
)

// commitTypes is the single source of truth for accepted conventional-commit
// types. The system prompt and the validator both derive from it so they cannot
// drift apart.
var commitTypes = []string{
	"feat", "fix", "chore", "refactor", "docs", "test", "build", "ci", "perf", "style", "revert",
}

// commitLineRe matches a conventional-commit subject line: "type(scope)!: summary".
var commitLineRe = regexp.MustCompile(
	`^(` + strings.Join(commitTypes, "|") + `)(\([^)]+\))?!?: .+$`,
)

// OpenAIClient talks to any OpenAI-compatible chat-completions endpoint
// (Groq, OpenAI, Ollama, or a custom base URL).
type OpenAIClient struct {
	APIKey     string
	Model      string
	Provider   string
	BaseURL    string
	HTTPClient *http.Client
}

// New builds a client for the provider described by cfg. When cfg resolves to
// more than one model (a primary plus fallbacks), the returned client cascades
// through them, advancing only when a model is reported unavailable.
func New(cfg config.Config) Client {
	models := cfg.Models()
	if len(models) <= 1 {
		return newOpenAIClient(cfg, cfg.Model)
	}
	return &fallbackClient{
		models: models,
		build:  func(model string) Client { return newOpenAIClient(cfg, model) },
	}
}

// newOpenAIClient builds a client bound to a single model.
func newOpenAIClient(cfg config.Config, model string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:     cfg.APIKey,
		Model:      model,
		Provider:   config.ProviderLabel(cfg.Provider),
		BaseURL:    baseURLFor(cfg),
		HTTPClient: &http.Client{},
	}
}

func baseURLFor(cfg config.Config) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	switch cfg.Provider {
	case "openai":
		return openaiBaseURL
	case "ollama":
		return ollamaBaseURL
	default:
		return groqBaseURL
	}
}

// NewGroqClient returns a client pointed at the Groq default endpoint.
func NewGroqClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:     apiKey,
		Model:      model,
		Provider:   "groq",
		BaseURL:    groqBaseURL,
		HTTPClient: &http.Client{},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func systemPrompt(persona string) string {
	return fmt.Sprintf(
		"You are a commit-message generator writing in the %s persona. "+
			"Output a conventional commit: first line 'type(scope): summary' (max 72 characters), "+
			"then a blank line, then a short dramatic monologue body in that persona. "+
			"Valid types: %s.",
		persona, strings.Join(commitTypes, ", "),
	)
}

// validateMessage enforces the strict conventional-commit shape the prompt asks
// for, so malformed model output triggers the fallback instead of reaching git.
func validateMessage(msg string) error {
	if strings.TrimSpace(msg) == "" {
		return errors.New("message must not be empty")
	}

	lines := strings.Split(msg, "\n")
	first := strings.TrimRight(lines[0], " \t\r")

	if n := len([]rune(first)); n > 72 {
		return fmt.Errorf("first line is %d characters, want at most 72", n)
	}
	if !commitLineRe.MatchString(first) {
		return fmt.Errorf("first line %q is not a valid 'type(scope): summary' conventional commit", first)
	}
	// A body, if present, must be separated from the summary by a blank line.
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		return errors.New("body must be separated from the summary by a blank line")
	}
	return nil
}

// fail builds a categorized *Error carrying this client's provider and model.
func (c *OpenAIClient) fail(kind ErrorKind, err error, msg string) *Error {
	return &Error{Kind: kind, Provider: c.Provider, Model: c.Model, Message: msg, Err: err}
}

func (c *OpenAIClient) Generate(ctx context.Context, req Request) (string, error) {
	if c.APIKey == "" {
		return "", c.fail(KindAuth, nil, "missing API key")
	}

	body := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(req.Persona)},
			{Role: "user", Content: fmt.Sprintf("Diff stats: %s\n\nDiff:\n%s", req.Stats, req.Diff)},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", c.fail(KindUnknown, err, "marshaling request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return "", c.fail(KindUnknown, err, "building request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		kind := KindNetwork
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			kind = KindTimeout
		}
		return "", c.fail(kind, err, "request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		code, message := parseProviderError(raw)
		return "", &Error{
			Kind:     classify(resp.StatusCode, code, message),
			Status:   resp.StatusCode,
			Provider: c.Provider,
			Model:    c.Model,
			Message:  message,
		}
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", c.fail(KindInvalidOutput, err, "decoding response")
	}
	if len(parsed.Choices) == 0 {
		return "", c.fail(KindInvalidOutput, nil, "empty response")
	}

	raw := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if err := validateMessage(raw); err != nil {
		return "", c.fail(KindInvalidOutput, err, "invalid commit message")
	}
	return raw, nil
}
