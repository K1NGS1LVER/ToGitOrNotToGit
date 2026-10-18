package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/severity"
	"gopkg.in/yaml.v3"
)

const maxTimeoutMS = 60000

type Config struct {
	Provider          string   `yaml:"provider"`
	Model             string   `yaml:"model"`
	BaseURL           string   `yaml:"base_url,omitempty"`
	TimeoutMS         int      `yaml:"timeout_ms"`
	TUI               bool     `yaml:"tui"`
	Persona           string   `yaml:"persona"`
	SeverityOverrides []string `yaml:"severity_overrides,omitempty"`
	APIKey            string   `yaml:"-"`
}

// validProviders are the providers tocommit can talk to. An empty provider
// resolves to groq.
var validProviders = map[string]bool{
	"groq":              true,
	"openai":            true,
	"ollama":            true,
	"openai-compatible": true,
}

func Default() Config {
	return Config{
		Provider:  "groq",
		Model:     "llama-3.3-70b-versatile",
		TimeoutMS: 2500,
		TUI:       true,
	}
}

// Validate reports configuration that would make the hook misbehave. It is kept
// separate from loading so that `tocommit config` can still open (and repair) a
// file that fails validation.
func (c Config) Validate() error {
	if c.Provider != "" && !validProviders[c.Provider] {
		return fmt.Errorf("unknown provider %q; valid: groq, openai, ollama, openai-compatible", c.Provider)
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("model must not be empty")
	}
	if c.TimeoutMS <= 0 || c.TimeoutMS > maxTimeoutMS {
		return fmt.Errorf("timeout_ms must be between 1 and %d (got %d)", maxTimeoutMS, c.TimeoutMS)
	}
	if c.Persona != "" && !severity.IsValidPersona(c.Persona) {
		return fmt.Errorf("unknown persona %q; valid: %s", c.Persona, strings.Join(severity.ValidPersonas(), ", "))
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("base_url %q must be an absolute http(s) URL", c.BaseURL)
		}
	}
	return nil
}

// LoadFrom reads config from an explicit path. A missing file yields defaults.
// Validation is deliberately not performed here; call Validate for that.
func LoadFrom(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if uerr := yaml.Unmarshal(data, &cfg); uerr != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, uerr)
		}
	case os.IsNotExist(err):
		// no config file - use defaults
	default:
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg.APIKey = resolveAPIKey(cfg.Provider)
	return cfg, nil
}

// resolveAPIKey looks up the key for a provider from the environment, honoring
// the provider-agnostic TOCOMMIT_API_KEY first. Keys are never persisted.
func resolveAPIKey(provider string) string {
	if v := os.Getenv("TOCOMMIT_API_KEY"); v != "" {
		return v
	}
	switch provider {
	case "openai", "openai-compatible":
		return os.Getenv("OPENAI_API_KEY")
	case "ollama":
		return ""
	default: // "", "groq"
		return os.Getenv("GROQ_API_KEY")
	}
}

// DefaultPath returns the standard user location for tocommit's config file.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tocommit", "config.yaml"), nil
}

// Load reads config from the standard user location.
func Load() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(path)
}
