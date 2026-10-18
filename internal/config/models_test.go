package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestModels_PrimaryFirstThenConfiguredFallbacks(t *testing.T) {
	cfg := Config{Provider: "groq", Model: "primary", FallbackModels: []string{"a", "b"}}
	want := []string{"primary", "a", "b"}
	if got := cfg.Models(); !reflect.DeepEqual(got, want) {
		t.Errorf("Models() = %v, want %v", got, want)
	}
}

func TestModels_UsesProviderDefaultsWhenNoneConfigured(t *testing.T) {
	groq := Config{Provider: "groq", Model: "primary"}
	if got := groq.Models(); len(got) < 2 || got[0] != "primary" {
		t.Errorf("groq Models() = %v, want primary plus built-in fallbacks", got)
	}

	openai := Config{Provider: "openai", Model: "primary"}
	if got := openai.Models(); !reflect.DeepEqual(got, []string{"primary"}) {
		t.Errorf("openai Models() = %v, want just [primary] (no built-in fallbacks)", got)
	}
}

func TestModels_DedupesAndDropsBlanks(t *testing.T) {
	cfg := Config{Provider: "groq", Model: "primary", FallbackModels: []string{"primary", " ", "secondary", "secondary"}}
	want := []string{"primary", "secondary"}
	if got := cfg.Models(); !reflect.DeepEqual(got, want) {
		t.Errorf("Models() = %v, want %v", got, want)
	}
}

func TestDefaultFallbackModels(t *testing.T) {
	if got := DefaultFallbackModels("groq"); len(got) == 0 {
		t.Error("DefaultFallbackModels(groq) should not be empty")
	}
	if got := DefaultFallbackModels("ollama"); got != nil {
		t.Errorf("DefaultFallbackModels(ollama) = %v, want nil", got)
	}
	if got := DefaultFallbackModels("openai-compatible"); got != nil {
		t.Errorf("DefaultFallbackModels(openai-compatible) = %v, want nil", got)
	}
}

func TestProviderLabel(t *testing.T) {
	if got := ProviderLabel(""); got != "groq" {
		t.Errorf("ProviderLabel(\"\") = %q, want groq", got)
	}
	if got := ProviderLabel("ollama"); got != "ollama" {
		t.Errorf("ProviderLabel(ollama) = %q, want ollama", got)
	}
}

func TestKeyEnvVar(t *testing.T) {
	tests := map[string]string{
		"":                  "GROQ_API_KEY",
		"groq":              "GROQ_API_KEY",
		"openai":            "OPENAI_API_KEY",
		"openai-compatible": "OPENAI_API_KEY",
		"ollama":            "",
	}
	for provider, want := range tests {
		if got := KeyEnvVar(provider); got != want {
			t.Errorf("KeyEnvVar(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestLoadFrom_FallbackModelsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := "provider: groq\nmodel: primary\nfallback_models:\n  - one\n  - two\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if want := []string{"one", "two"}; !reflect.DeepEqual(cfg.FallbackModels, want) {
		t.Errorf("FallbackModels = %v, want %v", cfg.FallbackModels, want)
	}
}

func TestLoadFrom_MissingFallbackModelsKeepsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("provider: groq\nmodel: primary\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.FallbackModels != nil {
		t.Errorf("FallbackModels = %v, want nil so provider defaults apply", cfg.FallbackModels)
	}
}

func TestValidate_RejectsBlankFallbackModel(t *testing.T) {
	cfg := Default()
	cfg.FallbackModels = []string{"ok", "  "}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil, want an error for a blank fallback model")
	}
}
