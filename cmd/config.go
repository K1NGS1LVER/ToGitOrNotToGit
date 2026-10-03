package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
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
				huh.NewInput().Title("Provider").Value(&cfg.Provider),
				huh.NewInput().Title("Model").Value(&cfg.Model),
				huh.NewInput().Title("Timeout (ms)").Value(&timeoutStr).Validate(func(s string) error {
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("must be a whole number of milliseconds")
					}
					return nil
				}),
			huh.NewConfirm().Title("Enable interactive commit preview (TUI)?").Value(&cfg.TUI),
			huh.NewInput().Title("Persona (empty = auto)").Value(&cfg.Persona),
		),
		)

		if err := form.Run(); err != nil {
			return fmt.Errorf("running config form: %w", err)
		}

		// error impossible here: huh's Validate above already rejected non-numeric input
		cfg.TimeoutMS, _ = strconv.Atoi(timeoutStr)

		if err := saveConfig(cfg, path); err != nil {
			return err
		}
		fmt.Println("Saved config to", path)
		return nil
	},
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
