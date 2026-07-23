package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestInstallThenUninstall(t *testing.T) {
	repo := initTestRepo(t)
	chdir(t, repo)

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install returned error: %v", err)
	}

	hookFile := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	data, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("reading installed hook: %v", err)
	}
	if !strings.Contains(string(data), hookMarker) {
		t.Errorf("installed hook missing marker: %s", data)
	}

	if err := uninstallCmd.RunE(uninstallCmd, nil); err != nil {
		t.Fatalf("uninstall returned error: %v", err)
	}
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Errorf("hook file still exists after uninstall")
	}
}

func TestInstall_RefusesToClobberForeignHook(t *testing.T) {
	repo := initTestRepo(t)
	chdir(t, repo)

	hookFile := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	if err := os.WriteFile(hookFile, []byte("#!/bin/sh\necho someone elses hook\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installCmd.RunE(installCmd, nil); err == nil {
		t.Fatal("install returned nil error, want refusal to overwrite foreign hook")
	}

	data, _ := os.ReadFile(hookFile)
	if strings.Contains(string(data), hookMarker) {
		t.Errorf("foreign hook was overwritten")
	}
}

func TestUninstall_RefusesToRemoveForeignHook(t *testing.T) {
	repo := initTestRepo(t)
	chdir(t, repo)

	hookFile := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	foreignContent := "#!/bin/sh\necho someone elses hook\n"
	if err := os.WriteFile(hookFile, []byte(foreignContent), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := uninstallCmd.RunE(uninstallCmd, nil); err == nil {
		t.Fatal("uninstall returned nil error, want refusal to remove foreign hook")
	}

	data, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("foreign hook was removed: %v", err)
	}
	if string(data) != foreignContent {
		t.Errorf("foreign hook content = %q, want untouched %q", data, foreignContent)
	}
}
