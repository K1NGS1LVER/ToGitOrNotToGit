package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/logx"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/severity"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func validatePersona(name string) error {
	if name != "" && !severity.IsValidPersona(name) {
		return fmt.Errorf("unknown persona %q; valid: %s", name, strings.Join(severity.ValidPersonas(), ", "))
	}
	return nil
}

var bypassSources = map[string]bool{
	"message": true,
	"commit":  true,
	"merge":   true,
	"squash":  true,
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
		Config:    config.Load,
		Diff:      diff.Collect,
		NewClient: llm.New,
		IsTTY: func() bool {
			return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
		},
		RunTUI: tui.Run,
	}
}

// warnActionableFailure prints a one-shot, actionable warning to stderr when the
// failure is something the user can fix: a model that no longer exists on the
// provider, or a rejected API key. Every other failure stays debug-only so the
// hook keeps its quiet default and never spams a normal commit.
func warnActionableFailure(cfg config.Config, err error) {
	provider := config.ProviderLabel(cfg.Provider)
	switch llm.KindOf(err) {
	case llm.KindModelUnavailable:
		fmt.Fprintf(os.Stderr,
			"tocommit: model %q is not available on %s and no fallback model worked (%v)\n"+
				"tocommit: run 'tocommit models' to list valid models, then 'tocommit config' to change it,\n"+
				"tocommit: or set 'provider: ollama' in ~/.config/tocommit/config.yaml to use a local model.\n",
			cfg.Model, provider, err)
	case llm.KindAuth:
		if env := config.KeyEnvVar(cfg.Provider); env != "" {
			fmt.Fprintf(os.Stderr,
				"tocommit: %s rejected the API key; set %s (or TOCOMMIT_API_KEY) (%v)\n",
				provider, env, err)
		} else {
			fmt.Fprintf(os.Stderr, "tocommit: %s rejected the request (%v)\n", provider, err)
		}
	}
}

var persona string

var runCmd = &cobra.Command{
	Use:  "run <commit-msg-file> [source] [sha]",
	Args: cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validatePersona(persona); err != nil {
			return err
		}
		source := ""
		if len(args) > 1 {
			source = args[1]
		}
		return runHook(args[0], source, persona, defaultDeps())
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&persona, "persona", "", "override auto-detected persona (victorian-gothic, soap-opera, shakespearean-tragedy)")
}

func runHook(msgFile, source, persona string, deps hookDeps) error {
	if bypassSources[source] {
		logx.Debugf("bypassing generation: commit source %q", source)
		return nil
	}

	stats, rawDiff, err := deps.Diff()
	if err != nil {
		return fmt.Errorf("collecting diff: %w", err)
	}
	if stats.FilesChanged == 0 {
		logx.Debugf("no staged changes; leaving the commit message untouched")
		return nil
	}

	cfg, err := deps.Config()
	if err != nil {
		logx.Debugf("loading config failed: %v", err)
		fmt.Fprintf(os.Stderr, "tocommit: %v; using fallback message (run 'tocommit config')\n", err)
		return os.WriteFile(msgFile, []byte(stats.FallbackMessage()+"\n"), 0o644)
	}

	// A bad config must never block the commit. Warn, log, and fall back.
	if verr := cfg.Validate(); verr != nil {
		logx.Debugf("invalid config: %v", verr)
		fmt.Fprintf(os.Stderr, "tocommit: %v; using fallback message (run 'tocommit config')\n", verr)
		return os.WriteFile(msgFile, []byte(stats.FallbackMessage()+"\n"), 0o644)
	}

	tier := severity.Score(stats, cfg.SeverityOverrides)
	resolvedPersona := persona
	if resolvedPersona == "" {
		resolvedPersona = tier.Persona()
	}

	logx.Debugf("provider=%s model=%s persona=%s stats=%d file(s) +%d/-%d",
		cfg.Provider, cfg.Model, resolvedPersona, stats.FilesChanged, stats.Insertions, stats.Deletions)

	client := deps.NewClient(cfg)

	req := llm.Request{
		Persona: resolvedPersona,
		Stats:   fmt.Sprintf("%d file(s), +%d/-%d", stats.FilesChanged, stats.Insertions, stats.Deletions),
		Diff:    rawDiff,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
	defer cancel()

	message, err := client.Generate(ctx, req)
	if err != nil {
		logx.Debugf("generation failed; using fallback message: %v", err)
		warnActionableFailure(cfg, err)
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
				logx.Debugf("regeneration failed; using fallback message: %v", err)
				return stats.FallbackMessage(), err
			}
			return msg, nil
		}

		final, accepted, err := deps.RunTUI(message, regen)
		if err != nil {
			logx.Debugf("TUI failed (%v); writing message straight through", err)
			return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
		}
		if !accepted {
			return fmt.Errorf("commit cancelled")
		}
		message = final
	}

	return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
}
