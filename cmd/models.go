package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
	"github.com/spf13/cobra"
)

const modelsListTimeout = 10 * time.Second

func init() {
	rootCmd.AddCommand(modelsCmd)
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List the model IDs available for the configured provider",
	Long: "List the model IDs the configured provider reports for your API key.\n" +
		"The configured model is marked with '*'. Copy a valid id into\n" +
		"model (or fallback_models) with 'tocommit config'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		provider := config.ProviderLabel(cfg.Provider)
		if env := config.KeyEnvVar(cfg.Provider); env != "" && cfg.APIKey == "" {
			return fmt.Errorf("no API key set for %s; export %s (or TOCOMMIT_API_KEY) and try again", provider, env)
		}

		ctx, cancel := context.WithTimeout(context.Background(), modelsListTimeout)
		defer cancel()

		ids, err := llm.ListModels(ctx, cfg)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			fmt.Printf("no models reported by %s\n", provider)
			return nil
		}

		for _, id := range ids {
			marker := " "
			if id == cfg.Model {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, id)
		}
		return nil
	},
}
