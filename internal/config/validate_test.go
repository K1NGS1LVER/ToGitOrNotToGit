package config

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"defaults are valid", func(*Config) {}, false},
		{"empty provider is allowed", func(c *Config) { c.Provider = "" }, false},
		{"openai provider", func(c *Config) { c.Provider = "openai" }, false},
		{"ollama provider", func(c *Config) { c.Provider = "ollama" }, false},
		{"unknown provider", func(c *Config) { c.Provider = "bogus" }, true},
		{"empty model", func(c *Config) { c.Model = "  " }, true},
		{"zero timeout", func(c *Config) { c.TimeoutMS = 0 }, true},
		{"negative timeout", func(c *Config) { c.TimeoutMS = -1 }, true},
		{"timeout above cap", func(c *Config) { c.TimeoutMS = 60001 }, true},
		{"unknown persona", func(c *Config) { c.Persona = "tragic" }, true},
		{"valid persona", func(c *Config) { c.Persona = "soap-opera" }, false},
		{"relative base url", func(c *Config) { c.BaseURL = "not a url" }, true},
		{"non-http base url", func(c *Config) { c.BaseURL = "ftp://example.com" }, true},
		{"valid base url", func(c *Config) { c.BaseURL = "http://localhost:11434/v1/chat/completions" }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("TOCOMMIT_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "groq-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	tests := []struct {
		provider string
		want     string
	}{
		{"", "groq-key"},
		{"groq", "groq-key"},
		{"openai", "openai-key"},
		{"openai-compatible", "openai-key"},
		{"ollama", ""},
	}
	for _, tt := range tests {
		if got := resolveAPIKey(tt.provider); got != tt.want {
			t.Errorf("resolveAPIKey(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}

	t.Setenv("TOCOMMIT_API_KEY", "master-key")
	if got := resolveAPIKey("openai"); got != "master-key" {
		t.Errorf("resolveAPIKey with TOCOMMIT_API_KEY = %q, want master-key", got)
	}
}
