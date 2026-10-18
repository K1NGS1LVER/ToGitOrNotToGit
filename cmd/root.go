package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/logx"
	"github.com/spf13/cobra"
)

// version is overridden at build time via
// -ldflags "-X github.com/K1NGS1LVER/ToGitOrNotToGit/cmd.version=<tag>".
var version = "dev"

// buildVersion prefers the injected version, then falls back to the module
// version reported by the toolchain (populated for `go install`ed binaries).
func buildVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

var debugFlag bool

var rootCmd = &cobra.Command{
	Use:     "tocommit",
	Short:   "Turns your git commits into theatrical monologues",
	Version: buildVersion(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logx.EnableFromEnv()
		if debugFlag {
			logx.Enable()
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable verbose debug logging to stderr")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
