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
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/tui"
	"github.com/mattn/go-isatty"
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
	IsTTY     func() bool
	RunTUI    func(initial string, regen func() (string, error)) (string, bool, error)
}

func defaultDeps() hookDeps {
	return hookDeps{
		Config: config.Load,
		Diff:   diff.Collect,
		NewClient: func(cfg config.Config) llm.Client {
			return llm.NewGroqClient(cfg.APIKey, cfg.Model)
		},
		IsTTY: func() bool {
			return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
		},
		RunTUI: tui.Run,
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

	req := llm.Request{
		Persona: tier.Persona(),
		Stats:   fmt.Sprintf("%d file(s), +%d/-%d", stats.FilesChanged, stats.Insertions, stats.Deletions),
		Diff:    rawDiff,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
	defer cancel()

	message, err := client.Generate(ctx, req)
	if err != nil {
		message = stats.FallbackMessage()
	}

	// cfg.TUI is checked before deps.IsTTY() (not the other order) so that
	// hookDeps literals which leave IsTTY nil - as the non-TUI integration
	// tests do - never invoke it when the config has TUI turned off.
	if cfg.TUI && deps.IsTTY() {
		regen := func() (string, error) {
			rctx, rcancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
			defer rcancel()
			msg, err := client.Generate(rctx, req)
			if err != nil {
				return stats.FallbackMessage(), err
			}
			return msg, nil
		}

		final, accepted, err := deps.RunTUI(message, regen)
		if err != nil {
			return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
		}
		if !accepted {
			return fmt.Errorf("commit cancelled")
		}
		message = final
	}

	return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
}
