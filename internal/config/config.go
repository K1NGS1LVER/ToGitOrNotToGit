package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	TimeoutMS int    `yaml:"timeout_ms"`
	TUI       bool   `yaml:"tui"`
	Persona   string `yaml:"persona"`
	APIKey    string `yaml:"-"`
}

func Default() Config {
	return Config{
		Provider:  "groq",
		Model:     "llama-3.3-70b-versatile",
		TimeoutMS: 2500,
		TUI:       true,
	}
}

// LoadFrom reads config from an explicit path. A missing file yields defaults.
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

	cfg.APIKey = os.Getenv("GROQ_API_KEY")
	return cfg, nil
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
