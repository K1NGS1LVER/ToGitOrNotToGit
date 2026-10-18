package llm

import (
	"strings"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
)

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantErr bool
	}{
		{"simple type", "feat: add a thing", false},
		{"scoped", "fix(parser): handle empty input", false},
		{"breaking change", "feat(api)!: drop the v1 field", false},
		{"body after blank line", "feat: add a thing\n\nAn epic monologue unfolds.", false},
		{"multiline body", "fix: repair it\n\nline one\nline two", false},
		{"empty", "", true},
		{"whitespace only", "   \n  ", true},
		{"bare url", "https://example.com/path", true},
		{"prose without a type", "updated the config file", true},
		{"unknown type", "update: something", true},
		{"missing summary", "feat: ", true},
		{"no space after colon", "feat:do it", true},
		{"body not separated by blank line", "feat: add a thing\nimmediate body", true},
		{"first line too long", "feat: " + strings.Repeat("x", 80), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMessage(tt.msg)
			if tt.wantErr && err == nil {
				t.Errorf("validateMessage(%q) = nil, want an error", tt.msg)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateMessage(%q) = %v, want nil", tt.msg, err)
			}
		})
	}
}

func TestSystemPrompt_ListsEveryAcceptedType(t *testing.T) {
	prompt := systemPrompt("soap-opera")
	for _, typ := range commitTypes {
		if !strings.Contains(prompt, typ) {
			t.Errorf("system prompt is missing accepted type %q", typ)
		}
	}
}

func TestBaseURLFor(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"empty provider defaults to groq", config.Config{}, groqBaseURL},
		{"groq", config.Config{Provider: "groq"}, groqBaseURL},
		{"openai", config.Config{Provider: "openai"}, openaiBaseURL},
		{"ollama", config.Config{Provider: "ollama"}, ollamaBaseURL},
		{"custom base url", config.Config{Provider: "openai-compatible", BaseURL: "https://example.test/v1/chat"}, "https://example.test/v1/chat"},
		{"base url wins over provider", config.Config{Provider: "openai", BaseURL: "https://example.test/v1/chat"}, "https://example.test/v1/chat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseURLFor(tt.cfg); got != tt.want {
				t.Errorf("baseURLFor(%+v) = %q, want %q", tt.cfg, got, tt.want)
			}
		})
	}
}

func TestNew_UsesProviderSettings(t *testing.T) {
	client := New(config.Config{Provider: "openai", Model: "gpt-x", APIKey: "k"})
	oc, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatalf("New() returned %T, want *OpenAIClient", client)
	}
	if oc.BaseURL != openaiBaseURL {
		t.Errorf("BaseURL = %q, want %q", oc.BaseURL, openaiBaseURL)
	}
	if oc.Model != "gpt-x" || oc.APIKey != "k" {
		t.Errorf("client = %+v, want model=gpt-x apiKey=k", oc)
	}
	if oc.HTTPClient == nil {
		t.Error("HTTPClient is nil, want a usable client")
	}
}
