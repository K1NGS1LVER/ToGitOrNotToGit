package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm"
)

type fakeClient struct {
	message   string
	messages  []string
	err       error
	gotReq    *llm.Request
	callCount int
}

func (f *fakeClient) Generate(ctx context.Context, req llm.Request) (string, error) {
	f.gotReq = &req
	f.callCount++
	if len(f.messages) > 0 {
		idx := f.callCount - 1
		if idx >= len(f.messages) {
			idx = len(f.messages) - 1
		}
		return f.messages[idx], f.err
	}
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
		IsTTY: func() bool { return false },
		RunTUI: func(initial string, regen func() (string, error)) (string, bool, error) {
			panic("RunTUI should not be called when IsTTY is false")
		},
	}
}

func TestRunHook_BypassOnMessageSource(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runHook(msgFile, "message", "", testDeps(diff.Stats{FilesChanged: 1}, &fakeClient{message: "should not be used"}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_BypassOnMergeSource(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runHook(msgFile, "merge", "", testDeps(diff.Stats{FilesChanged: 1}, &fakeClient{message: "should not be used"})); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_BypassOnSquashSource(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runHook(msgFile, "squash", "", testDeps(diff.Stats{FilesChanged: 1}, &fakeClient{message: "should not be used"})); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_BypassOnCommitSource(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runHook(msgFile, "commit", "", testDeps(diff.Stats{FilesChanged: 1}, &fakeClient{message: "should not be used"}))
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

	err := runHook(msgFile, "", "", testDeps(diff.Stats{FilesChanged: 0}, &fakeClient{message: "should not be used"}))
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
	client := &fakeClient{message: "feat: a tale of two files"}
	err := runHook(msgFile, "", "", testDeps(stats, client))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: a tale of two files\n" {
		t.Errorf("message file = %q, want the LLM message", got)
	}

	if client.gotReq == nil {
		t.Fatal("client did not receive a request")
	}
	if want := "victorian-gothic"; client.gotReq.Persona != want {
		t.Errorf("request Persona = %q, want %q", client.gotReq.Persona, want)
	}
}

func TestRunHook_FallsBackOnLLMError(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}
	err := runHook(msgFile, "", "", testDeps(stats, &fakeClient{err: errTestLLMFailure}))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 2 file(s) (+5/-1)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}

var errTestLLMFailure = context.DeadlineExceeded

func TestRunHook_SkipsTUIWhenNotTTY(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: plain message"})
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: plain message\n" {
		t.Errorf("message file = %q, want v1-style plain write", got)
	}
}

func TestRunHook_SkipsTUIWhenDisabledInConfig(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: plain message"})
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: false}, nil
	}
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		panic("RunTUI should not be called when cfg.TUI is false")
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: plain message\n" {
		t.Errorf("message file = %q, want v1-style plain write", got)
	}
}

func TestRunHook_TUIAcceptedWritesFinalMessage(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: draft message"})
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		if initial != "feat: draft message" {
			t.Errorf("TUI got initial = %q, want %q", initial, "feat: draft message")
		}
		return "feat: edited in TUI", true, nil
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: edited in TUI\n" {
		t.Errorf("message file = %q, want TUI-edited message", got)
	}
}

func TestRunHook_TUICancelledAbortsCommit(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("original"), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: draft message"})
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		return "", false, nil
	}

	err := runHook(msgFile, "", "", deps)
	if err == nil {
		t.Fatal("expected runHook to return an error when the TUI is cancelled")
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "original" {
		t.Errorf("message file = %q, want untouched", got)
	}
}

func TestRunHook_TUIErrorFallsBackToPlainWrite(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: draft message"})
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		return "", false, errTUIBroken
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v, want nil (TUI failure falls back, doesn't abort)", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: draft message\n" {
		t.Errorf("message file = %q, want the pre-TUI message written straight through", got)
	}
}

var errTUIBroken = errors.New("tui: program failed to start")

func TestRunHook_TUIRegenerateSuccessCallsLLMAgain(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	client := &fakeClient{messages: []string{"feat: first draft", "feat: second draft"}}
	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, client)
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}

	var gotRegenErr error
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		text, err := regen()
		gotRegenErr = err
		return text, true, nil
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}
	if gotRegenErr != nil {
		t.Fatalf("regen returned error: %v", gotRegenErr)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "feat: second draft\n" {
		t.Errorf("message file = %q, want the regenerated (second) message", got)
	}
}

func TestRunHook_TUIRegenerateErrorReturnsFallbackText(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	client := &fakeClient{message: "feat: first draft"}
	deps := testDeps(diff.Stats{FilesChanged: 2, Insertions: 5, Deletions: 1}, client)
	deps.IsTTY = func() bool { return true }
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}

	var gotRegenErr error
	deps.RunTUI = func(initial string, regen func() (string, error)) (string, bool, error) {
		client.err = context.DeadlineExceeded
		text, err := regen()
		gotRegenErr = err
		return text, true, nil
	}

	if err := runHook(msgFile, "", "", deps); err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}
	if gotRegenErr == nil {
		t.Fatal("expected regen to return an error")
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "chore: update 2 file(s) (+5/-1)\n" {
		t.Errorf("message file = %q, want the fallback message", got)
	}
}

func TestRunHook_PersonaOverrideReachesLLM(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}
	client := &fakeClient{message: "feat: a dramatic plot twist"}
	err := runHook(msgFile, "", "soap-opera", testDeps(stats, client))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	if client.gotReq == nil {
		t.Fatal("client did not receive a request")
	}
	if want := "soap-opera"; client.gotReq.Persona != want {
		t.Errorf("request Persona = %q, want %q", client.gotReq.Persona, want)
	}
}

func TestValidatePersona_InvalidReturnsError(t *testing.T) {
	err := validatePersona("invalid")
	if err == nil {
		t.Fatal("expected error for invalid persona")
	}
}

func TestValidatePersona_EmptyReturnsNil(t *testing.T) {
	if err := validatePersona(""); err != nil {
		t.Fatalf("expected nil for empty persona, got %v", err)
	}
}

func TestRunHook_EmptyPersonaAutoDetects(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	stats := diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}
	client := &fakeClient{message: "feat: auto-detected persona"}
	err := runHook(msgFile, "", "", testDeps(stats, client))
	if err != nil {
		t.Fatalf("runHook returned error: %v", err)
	}

	if client.gotReq == nil {
		t.Fatal("client did not receive a request")
	}
	// Empty persona should resolve via severity.Score, same as existing behavior
	if want := "victorian-gothic"; client.gotReq.Persona != want {
		t.Errorf("request Persona = %q, want %q (auto-detected)", client.gotReq.Persona, want)
	}
}
