package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
)

func runGitOrFail(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, out)
	}
}

func setupStagedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitOrFail(t, dir, "init")
	runGitOrFail(t, dir, "config", "user.email", "test@example.com")
	runGitOrFail(t, dir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitOrFail(t, dir, "add", "main.go")

	return dir
}

func TestIntegration_RunHook_EndToEnd_Success(t *testing.T) {
	dir := setupStagedRepo(t)
	chdir(t, dir)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := hookDeps{
		Config: func() (config.Config, error) {
			return config.Config{Model: "m", TimeoutMS: 2500, APIKey: "k"}, nil
		},
		Diff: diff.Collect,
		NewClient: func(cfg config.Config) llm.Client {
			return fakeClient{message: "feat: a new file enters the stage"}
		},
	}

	if err := runHook(msgFile, "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: a new file enters the stage\n" {
		t.Errorf("message file = %q, want the LLM message", got)
	}
}

func TestIntegration_RunHook_EndToEnd_FallsBackOnLLMError(t *testing.T) {
	dir := setupStagedRepo(t)
	chdir(t, dir)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := hookDeps{
		Config: func() (config.Config, error) {
			return config.Config{Model: "m", TimeoutMS: 2500, APIKey: "k"}, nil
		},
		Diff: diff.Collect,
		NewClient: func(cfg config.Config) llm.Client {
			return fakeClient{err: context.DeadlineExceeded}
		},
	}

	if err := runHook(msgFile, "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 1 file(s) (+1/-0)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}
