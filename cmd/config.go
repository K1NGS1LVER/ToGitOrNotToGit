package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/severity"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactively edit tocommit's config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.DefaultPath()
		if err != nil {
			return err
		}

		cfg, err := config.LoadFrom(path)
		if err != nil {
			return err
		}

		timeoutStr := strconv.Itoa(cfg.TimeoutMS)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Provider").
					Options(
						huh.NewOption("Groq", "groq"),
						huh.NewOption("OpenAI", "openai"),
						huh.NewOption("Ollama (local)", "ollama"),
						huh.NewOption("OpenAI-compatible (custom base URL)", "openai-compatible"),
					).
					Value(&cfg.Provider),
				huh.NewInput().Title("Model").Value(&cfg.Model).Validate(nonBlank("model")),
				huh.NewInput().Title("Base URL (optional, for custom endpoints)").Value(&cfg.BaseURL),
				huh.NewInput().Title("Timeout (ms)").Value(&timeoutStr).Validate(validateTimeoutMS),
				huh.NewConfirm().Title("Enable interactive commit preview (TUI)?").Value(&cfg.TUI),
				huh.NewInput().Title("Persona (empty = auto)").Value(&cfg.Persona).Validate(validatePersonaInput),
			),
		)

		if err := form.Run(); err != nil {
			return fmt.Errorf("running config form: %w", err)
		}

		// error impossible here: the form's Validate already rejected non-numeric input
		cfg.TimeoutMS, _ = strconv.Atoi(timeoutStr)

		if err := cfg.Validate(); err != nil {
			return err
		}

		if err := saveConfig(cfg, path); err != nil {
			return err
		}
		fmt.Println("Saved config to", path)
		return nil
	},
}

func nonBlank(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s must not be empty", field)
		}
		return nil
	}
}

func validateTimeoutMS(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a whole number of milliseconds")
	}
	if n <= 0 || n > 60000 {
		return fmt.Errorf("must be between 1 and 60000")
	}
	return nil
}

func validatePersonaInput(s string) error {
	if s != "" && !severity.IsValidPersona(s) {
		return fmt.Errorf("valid: %s", strings.Join(severity.ValidPersonas(), ", "))
	}
	return nil
}

func saveConfig(cfg config.Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
