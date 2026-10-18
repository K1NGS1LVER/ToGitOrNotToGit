package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })

	fn()
	_ = w.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestRunHook_InvalidConfigFallsBackWithoutAborting(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := testDeps(diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}, &fakeClient{message: "should not be used"})
	// timeout_ms: 0 fails Validate; the hook must not block the commit.
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 0, APIKey: "k"}, nil
	}

	stderr := captureStderr(t, func() {
		if err := runHook(msgFile, "", "", deps); err != nil {
			t.Fatalf("runHook returned error: %v, want nil (a bad config must not block the commit)", err)
		}
	})

	if !strings.Contains(stderr, "tocommit:") {
		t.Errorf("stderr = %q, want a config warning", stderr)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 2 file(s) (+5/-1)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}

func TestRunHook_ConfigErrorReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}
	deps := testDeps(stats, &fakeClient{message: "should not be used"})
	deps.Config = func() (config.Config, error) {
		return config.Config{}, errConfigLoad
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v, want nil", err)
	}
}

var errConfigLoad = errConfig("boom")

type errConfig string

func (e errConfig) Error() string { return string(e) }
