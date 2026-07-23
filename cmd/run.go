package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/severity"
	"github.com/spf13/cobra"
)

var bypassSources = map[string]bool{
	"message": true,
	"commit":  true,
}

type hookDeps struct {
	Config    func() (config.Config, error)
	Diff      func() (diff.Stats, string, error)
	NewClient func(cfg config.Config) llm.Client
}

func defaultDeps() hookDeps {
	return hookDeps{
		Config: config.Load,
		Diff:   diff.Collect,
		NewClient: func(cfg config.Config) llm.Client {
			return llm.NewGroqClient(cfg.APIKey, cfg.Model)
		},
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:  "run <commit-msg-file> [source] [sha]",
	Args: cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := ""
		if len(args) > 1 {
			source = args[1]
		}
		return runHook(args[0], source, defaultDeps())
	},
}

func runHook(msgFile, source string, deps hookDeps) error {
	if bypassSources[source] {
		return nil
	}

	stats, rawDiff, err := deps.Diff()
	if err != nil {
		return fmt.Errorf("collecting diff: %w", err)
	}
	if stats.FilesChanged == 0 {
		return nil
	}

	cfg, err := deps.Config()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	tier := severity.Score(stats)
	client := deps.NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
	defer cancel()

	message, err := client.Generate(ctx, llm.Request{
		Persona: tier.Persona(),
		Stats:   fmt.Sprintf("%d file(s), +%d/-%d", stats.FilesChanged, stats.Insertions, stats.Deletions),
		Diff:    rawDiff,
	})
	if err != nil {
		message = stats.FallbackMessage()
	}

	return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
}
