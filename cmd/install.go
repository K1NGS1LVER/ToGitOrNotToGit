package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const hookMarker = "# managed by tocommit"

const hookScript = "#!/bin/sh\n" + hookMarker + "\nexec tocommit run \"$1\" \"$2\" \"$3\"\n"

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the prepare-commit-msg hook in the current repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := hookPath()
		if err != nil {
			return err
		}

		existing, err := os.ReadFile(path)
		switch {
		case err == nil && !strings.Contains(string(existing), hookMarker):
			return fmt.Errorf("a prepare-commit-msg hook already exists at %s and wasn't installed by tocommit; remove it first", path)
		case err != nil && !os.IsNotExist(err):
			return fmt.Errorf("reading existing hook: %w", err)
		}

		if err := os.WriteFile(path, []byte(hookScript), 0o755); err != nil {
			return fmt.Errorf("writing hook: %w", err)
		}
		fmt.Println("Installed prepare-commit-msg hook at", path)
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the tocommit prepare-commit-msg hook from the current repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := hookPath()
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading hook: %w", err)
		}
		if !strings.Contains(string(data), hookMarker) {
			return fmt.Errorf("hook at %s wasn't installed by tocommit; leaving it alone", path)
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing hook: %w", err)
		}
		fmt.Println("Removed prepare-commit-msg hook at", path)
		return nil
	},
}

func hookPath() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any of the parent directories)")
	}
	gitDir := strings.TrimSpace(string(out))
	return filepath.Join(gitDir, "hooks", "prepare-commit-msg"), nil
}
