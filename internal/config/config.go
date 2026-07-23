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
	APIKey    string `yaml:"-"`
}

func Default() Config {
	return Config{
		Provider:  "groq",
		Model:     "llama-3.3-70b-versatile",
		TimeoutMS: 2500,
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

// Load reads config from the standard user location.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolving home directory: %w", err)
	}
	return LoadFrom(filepath.Join(home, ".config", "tocommit", "config.yaml"))
}
