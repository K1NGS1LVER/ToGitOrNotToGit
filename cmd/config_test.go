package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"gopkg.in/yaml.v3"
)

func TestSaveConfig_WritesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Config{Provider: "groq", Model: "llama-3.3-70b-versatile", TimeoutMS: 1234, TUI: false, APIKey: "should-never-be-written"}
	if err := saveConfig(cfg, path); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}

	var got config.Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshaling saved config: %v", err)
	}
	if got.Provider != "groq" || got.Model != "llama-3.3-70b-versatile" || got.TimeoutMS != 1234 || got.TUI != false {
		t.Errorf("saved config = %+v, want provider=groq model=llama-3.3-70b-versatile timeout_ms=1234 tui=false", got)
	}
	if strings.Contains(string(data), "should-never-be-written") {
		t.Error("saved YAML contains the API key - it must never be written to the config file")
	}
}

func TestSaveConfig_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "config.yaml")

	cfg := config.Default()
	if err := saveConfig(cfg, path); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to exist at %s: %v", path, err)
	}
}
