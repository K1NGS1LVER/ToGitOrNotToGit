package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GroqClient struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewGroqClient(apiKey, model string) *GroqClient {
	return &GroqClient{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    "https://api.groq.com/openai/v1/chat/completions",
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
			"Output a conventional commit: first line 'type(scope): summary', "+
			"then a blank line, then a short dramatic monologue body in that persona. "+
			"Valid types: feat, fix, chore, refactor, docs, test, build, ci.",
		persona,
	)
}

func validateMessage(msg string) error {
	if msg == "" {
		return errors.New("message must not be empty")
	}

	rest := strings.TrimLeft(msg, " \t")
	if !strings.Contains(rest, ":") {
		return fmt.Errorf("message %q is missing the required 'type(scope): summary' prefix", msg)
	}
	typ := strings.TrimSpace(strings.SplitN(rest, ":", 2)[0])
	if typ == "" {
		return fmt.Errorf("message %q has an empty type", msg)
	}
	if len(rest) > 0 {
		summary := strings.TrimSpace(rest[len(typ)+1:])
		if summary == "" {
			return fmt.Errorf("message %q has an empty summary after the colon", msg)
		}
	}
	return nil
}

func (c *GroqClient) Generate(ctx context.Context, req Request) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("groq: missing API key")
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
		return "", fmt.Errorf("groq: marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("groq: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("groq: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("groq: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("groq: decoding response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("groq: empty response")
	}

	raw := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if err := validateMessage(raw); err != nil {
		return "", fmt.Errorf("groq: invalid commit message: %w", err)
	}
	return raw, nil
}
