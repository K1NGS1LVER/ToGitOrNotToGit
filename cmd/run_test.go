package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
)

type fakeClient struct {
	message string
	err     error
}

func (f fakeClient) Generate(ctx context.Context, req llm.Request) (string, error) {
	return f.message, f.err
}

func testDeps(stats diff.Stats, client llm.Client) hookDeps {
	return hookDeps{
		Config: func() (config.Config, error) {
			return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k"}, nil
		},
		Diff: func() (diff.Stats, string, error) {
			return stats, "diff text", nil
		},
		NewClient: func(cfg config.Config) llm.Client {
			return client
		},
	}
}

func TestRunHook_BypassOnMessageSource(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runHook(msgFile, "message", testDeps(diff.Stats{FilesChanged: 1}, fakeClient{message: "should not be used"}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_NoStagedChanges(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runHook(msgFile, "", testDeps(diff.Stats{FilesChanged: 0}, fakeClient{message: "should not be used"}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_WritesLLMMessageOnSuccess(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}
	err := runHook(msgFile, "", testDeps(stats, fakeClient{message: "feat: a tale of two files"}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: a tale of two files\n" {
		t.Errorf("message file = %q, want the LLM message", got)
	}
}

func TestRunHook_FallsBackOnLLMError(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}
	err := runHook(msgFile, "", testDeps(stats, fakeClient{err: errTestLLMFailure}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 2 file(s) (+5/-1)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}

var errTestLLMFailure = context.DeadlineExceeded
