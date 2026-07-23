package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Provider != "groq" {
		t.Errorf("Provider = %q, want groq", cfg.Provider)
	}
	if cfg.Model != "llama-3.3-70b-versatile" {
		t.Errorf("Model = %q, want llama-3.3-70b-versatile", cfg.Model)
	}
	if cfg.TimeoutMS != 2500 {
		t.Errorf("TimeoutMS = %d, want 2500", cfg.TimeoutMS)
	}
}

func TestLoadFrom_MissingFile_ReturnsDefaults(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults %+v", cfg, Default())
	}
}

func TestLoadFrom_OverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "provider: groq\nmodel: custom-model\ntimeout_ms: 1000\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want custom-model", cfg.Model)
	}
	if cfg.TimeoutMS != 1000 {
		t.Errorf("TimeoutMS = %d, want 1000", cfg.TimeoutMS)
	}
}

func TestLoadFrom_ReadsAPIKeyFromEnv(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key-123")
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.APIKey != "test-key-123" {
		t.Errorf("APIKey = %q, want test-key-123", cfg.APIKey)
	}
}
