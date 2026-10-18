package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
)

func TestRunHook_WarnsAndFallsBackOnUnavailableModel(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}
	deps := testDeps(stats, &fakeClient{err: &llm.Error{
		Kind:     llm.KindModelUnavailable,
		Status:   404,
		Provider: "groq",
		Model:    "retired-model",
		Message:  "The model does not exist",
	}})

	stderr := captureStderr(t, func() {
		if err := runHook(msgFile, "", "", deps); err != nil {
			t.Fatalf("runHook returned error: %v, want nil (a model failure must not block the commit)", err)
		}
	})

	for _, want := range []string{"not available on groq", "tocommit models", "ollama"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 2 file(s) (+5/-1)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}

func TestRunHook_WarnsOnAuthFailure(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 2, Deletions: 0}, &fakeClient{err: &llm.Error{
		Kind:     llm.KindAuth,
		Status:   401,
		Provider: "groq",
		Model:    "m",
		Message:  "invalid api key",
	}})

	stderr := captureStderr(t, func() {
		if err := runHook(msgFile, "", "", deps); err != nil {
			t.Fatalf("runHook returned error: %v, want nil", err)
		}
	})

	if !strings.Contains(stderr, "GROQ_API_KEY") {
		t.Errorf("stderr = %q, want it to name the key env var", stderr)
	}
}

func TestRunHook_StaysQuietOnGenericFailure(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 2, Deletions: 0}, &fakeClient{err: context.DeadlineExceeded})

	stderr := captureStderr(t, func() {
		if err := runHook(msgFile, "", "", deps); err != nil {
			t.Fatalf("runHook returned error: %v, want nil", err)
		}
	})

	if stderr != "" {
		t.Errorf("stderr = %q, want empty for a non-actionable failure", stderr)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" a , b ,, c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitList = %v, want %v", got, want)
			break
		}
	}
}

func TestModelsCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "models" {
			found = true
		}
	}
	if !found {
		t.Error("expected a 'models' command to be registered on the root command")
	}
}
