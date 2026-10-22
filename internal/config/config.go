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
	FallbackModels    []string `yaml:"fallback_models,omitempty"`
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
		Model:     "openai/gpt-oss-120b",
		TimeoutMS: 2500,
		TUI:       true,
	}
}

// DefaultFallbackModels returns the models tocommit tries, in order, after the
// configured model when the user has not supplied their own fallback_models.
// Groq is the default provider and retires models periodically, so it ships a
// short list of current alternatives; the other providers default to none.
func DefaultFallbackModels(provider string) []string {
	switch ProviderLabel(provider) {
	case "groq":
		return []string{"openai/gpt-oss-20b", "qwen/qwen3.8-27b"}
	default:
		return nil
	}
}

// ProviderLabel returns a display name for a provider, resolving the empty
// string to the default provider.
func ProviderLabel(provider string) string {
	if provider == "" {
		return "groq"
	}
	return provider
}

// KeyEnvVar returns the environment variable tocommit reads a provider's API
// key from. An empty result means the provider needs no key (Ollama). Note that
// TOCOMMIT_API_KEY overrides this at load time.
func KeyEnvVar(provider string) string {
	switch provider {
	case "openai", "openai-compatible":
		return "OPENAI_API_KEY"
	case "ollama":
		return ""
	default: // "", "groq"
		return "GROQ_API_KEY"
	}
}

// Models returns the ordered, de-duplicated list of models to try: the
// configured model first, then the configured fallback_models, or the
// provider's built-in fallbacks when none are configured. Blank entries are
// dropped.
func (c Config) Models() []string {
	fallbacks := c.FallbackModels
	if len(fallbacks) == 0 {
		fallbacks = DefaultFallbackModels(c.Provider)
	}

	seen := make(map[string]bool, len(fallbacks)+1)
	models := make([]string, 0, len(fallbacks)+1)
	for _, m := range append([]string{c.Model}, fallbacks...) {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		models = append(models, m)
	}
	return models
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
	if c.Persona != "" && !severity.ValidatePersona(c.Persona) {
		return errors.New("persona must not be blank")
	}
	for i, m := range c.FallbackModels {
		if strings.TrimSpace(m) == "" {
			return fmt.Errorf("fallback_models[%d] must not be empty", i)
		}
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
	if env := KeyEnvVar(provider); env != "" {
		return os.Getenv(env)
	}
	return ""
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
