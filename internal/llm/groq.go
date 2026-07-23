package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		return "", fmt.Errorf("groq: unexpected status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("groq: decoding response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("groq: empty response")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
