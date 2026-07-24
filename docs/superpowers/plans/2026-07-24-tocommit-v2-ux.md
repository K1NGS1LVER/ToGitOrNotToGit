# tocommit v2 (config wizard + commit-preview TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an interactive `tocommit config` wizard and a bubbletea commit-preview screen (accept/edit/regenerate/cancel) that replaces git's editor step during a normal `git commit`, with both features fully inert when the terminal isn't interactive or `tui: false` is set.

**Architecture:** New `internal/tui` package holds a self-contained bubbletea `Model` with no knowledge of Groq/config/diffs - it only knows "show this text, and call this function if the user asks to regenerate." `cmd/run.go` wires it in behind a new `IsTTY`/`RunTUI` pair on the existing `hookDeps` struct, following the same dependency-injection pattern already used for `Config`/`Diff`/`NewClient`. `cmd/config.go` is a new standalone Cobra command using `huh` for the form; it shares `internal/config`'s existing `Config` struct and `LoadFrom`/`Default` functions, adding only a `TUI bool` field.

**Tech Stack:** `github.com/charmbracelet/bubbletea` (TUI runtime), `github.com/charmbracelet/bubbles` (`textarea`, `spinner` components), `github.com/charmbracelet/huh` (config form), `github.com/mattn/go-isatty` (TTY detection). No lipgloss - the preview screen is plain text in v2; styling is a `ponytail:` deferred upgrade, not a requirement anywhere in the spec.

## Global Constraints

- Go 1.22+ (existing `go.mod` floor - do not change it).
- `GROQ_API_KEY` stays env-only. The config wizard's `huh.Form` never has a field bound to `Config.APIKey`, and `Config.APIKey` already carries `yaml:"-"` so it can never round-trip through the wizard's save path even by accident.
- No new config fields beyond `TUI bool` (yaml `tui`). No severity/persona tuning, no Ollama, no retry logic - all explicitly out of scope per the spec.
- v1 behavior is unchanged whenever `IsTTY()` is false or `cfg.TUI` is false: `runHook` must still write the generated-or-fallback message straight to the file, byte-for-byte the same as today.
- Preserve the existing dependency-injection pattern in `cmd/run.go` (`hookDeps` with function fields, a `default*Deps()` constructor, tests overriding individual fields) - do not introduce a different pattern for the new TTY/TUI hooks.
- `internal/tui` must not import `internal/config`, `internal/diff`, or `internal/llm` - it takes an initial string and a `func() (string, error)` regenerate callback, nothing else. This is what makes it unit-testable without a real terminal.

---

### Task 1: `TUI` config field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.TUI bool` (yaml tag `tui`), `config.Default()` returns `TUI: true`. Consumed by Task 4 (`cmd/run.go`'s `IsTTY() && cfg.TUI` check) and Task 3 (`cmd/config.go`'s wizard confirm field).

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go` (check the existing file first for the right place to add these - alongside the other `Default`/`LoadFrom` tests):

```go
func TestDefault_TUIEnabledByDefault(t *testing.T) {
	cfg := Default()
	if !cfg.TUI {
		t.Errorf("Default().TUI = false, want true")
	}
}

func TestLoadFrom_TUIFieldParsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := "provider: groq\nmodel: llama-3.3-70b-versatile\ntimeout_ms: 2500\ntui: false\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.TUI {
		t.Errorf("cfg.TUI = true, want false (from YAML)")
	}
}

func TestLoadFrom_MissingTUIFieldDefaultsTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := "provider: groq\nmodel: llama-3.3-70b-versatile\n"
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if !cfg.TUI {
		t.Errorf("cfg.TUI = false, want true (default preserved when YAML omits the field)")
	}
}
```

Check the top of `internal/config/config_test.go` already imports `os`, `path/filepath`, and `testing` - if any are missing, add them.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/... -run 'TestDefault_TUIEnabledByDefault|TestLoadFrom_TUIFieldParsed|TestLoadFrom_MissingTUIFieldDefaultsTrue' -v`
Expected: FAIL (compile error: `cfg.TUI` undefined, or similar) since the field doesn't exist yet.

- [ ] **Step 3: Add the field**

In `internal/config/config.go`, change the `Config` struct and `Default()`:

```go
type Config struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	TimeoutMS int    `yaml:"timeout_ms"`
	TUI       bool   `yaml:"tui"`
	APIKey    string `yaml:"-"`
}

func Default() Config {
	return Config{
		Provider:  "groq",
		Model:     "llama-3.3-70b-versatile",
		TimeoutMS: 2500,
		TUI:       true,
	}
}
```

Note `LoadFrom` starts from `cfg := Default()` then unmarshals over it - since YAML unmarshaling only overwrites fields present in the document, a config file that omits `tui` keeps the default `true`. No other change needed in `LoadFrom`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS (all config tests, old and new)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add TUI config field, defaulting to enabled"
```

---

### Task 2: `internal/tui` commit-preview model

**Files:**
- Create: `internal/tui/preview.go`
- Create: `internal/tui/preview_test.go`

**Interfaces:**
- Consumes: nothing from other tasks - takes only stdlib types plus its own `regenFunc = func() (string, error)`.
- Produces: `tui.Run(initial string, regen func() (string, error)) (final string, accepted bool, err error)`. Consumed by Task 4 (`cmd/run.go`).

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
```

- [ ] **Step 2: Write the failing tests**

Create `internal/tui/preview_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel_InitialState(t *testing.T) {
	m := newModel("feat: initial message", func() (string, error) { return "", nil })
	if m.state != stateShowing {
		t.Errorf("initial state = %v, want stateShowing", m.state)
	}
	if m.message != "feat: initial message" {
		t.Errorf("initial message = %q, want %q", m.message, "feat: initial message")
	}
}

func TestUpdate_EnterAccepts(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if !m2.accepted {
		t.Errorf("accepted = false, want true after enter in stateShowing")
	}
}

func TestUpdate_QCancels(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m2 := updated.(model)
	if m2.accepted {
		t.Errorf("accepted = true, want false after q")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after q, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestUpdate_CtrlCCancelsFromAnyState(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2 := updated.(model)
	if m2.accepted {
		t.Errorf("accepted = true, want false after ctrl+c")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after ctrl+c, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestUpdate_EEntersEditingWithCurrentMessage(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m2 := updated.(model)
	if m2.state != stateEditing {
		t.Errorf("state = %v, want stateEditing", m2.state)
	}
	if m2.textarea.Value() != "feat: a message" {
		t.Errorf("textarea value = %q, want %q", m2.textarea.Value(), "feat: a message")
	}
}

func TestUpdate_EnterInEditingSavesText(t *testing.T) {
	m := newModel("feat: original", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: edited by hand")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing after enter in editing", m2.state)
	}
	if m2.message != "feat: edited by hand" {
		t.Errorf("message = %q, want %q", m2.message, "feat: edited by hand")
	}
}

func TestUpdate_EscInEditingDiscards(t *testing.T) {
	m := newModel("feat: original", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: a discarded edit")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing after esc in editing", m2.state)
	}
	if m2.message != "feat: original" {
		t.Errorf("message = %q, want original message preserved, got edit discarded incorrectly", m2.message)
	}
}

func TestUpdate_EditingPassesOtherKeysToTextarea(t *testing.T) {
	m := newModel("feat: ", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: ")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m2 := updated.(model)
	if m2.state != stateEditing {
		t.Errorf("state = %v, want to stay stateEditing", m2.state)
	}
	if m2.textarea.Value() == "feat: " {
		t.Errorf("textarea value unchanged, want the 'x' keystroke to have been applied")
	}
}

func TestUpdate_RInShowingStartsRegenerating(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "feat: regenerated", nil })
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m2 := updated.(model)
	if m2.state != stateRegenerating {
		t.Errorf("state = %v, want stateRegenerating", m2.state)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd to kick off regeneration")
	}
}

func TestUpdate_RegenResultSuccessReturnsToShowing(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating

	updated, _ := m.Update(regenResultMsg{text: "feat: freshly regenerated", err: nil})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing", m2.state)
	}
	if m2.message != "feat: freshly regenerated" {
		t.Errorf("message = %q, want %q", m2.message, "feat: freshly regenerated")
	}
	if m2.err != nil {
		t.Errorf("err = %v, want nil", m2.err)
	}
}

func TestUpdate_RegenResultErrorShowsFallbackWithNote(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating

	regenErr := errRegenTestFailure
	updated, _ := m.Update(regenResultMsg{text: "chore: update 2 file(s) (+5/-1)", err: regenErr})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing", m2.state)
	}
	if m2.message != "chore: update 2 file(s) (+5/-1)" {
		t.Errorf("message = %q, want the fallback text passed in the msg", m2.message)
	}
	if m2.err == nil {
		t.Errorf("err = nil, want the regen error to be recorded")
	}
	if view := m2.View(); view == "" {
		t.Error("View() returned empty string")
	}
}

var errRegenTestFailure = context.DeadlineExceeded
```

Add `"context"` to the imports of `preview_test.go` for `errRegenTestFailure`.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tui/... -v`
Expected: FAIL (compile errors - `newModel`, `model`, `stateShowing`, etc. don't exist yet)

- [ ] **Step 4: Write the implementation**

Create `internal/tui/preview.go`:

```go
package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type state int

const (
	stateShowing state = iota
	stateEditing
	stateRegenerating
)

type regenResultMsg struct {
	text string
	err  error
}

type model struct {
	message  string
	state    state
	regen    func() (string, error)
	spinner  spinner.Model
	textarea textarea.Model
	accepted bool
	err      error
}

func newModel(initial string, regen func() (string, error)) model {
	ta := textarea.New()
	ta.SetValue(initial)

	return model{
		message:  initial,
		state:    stateShowing,
		regen:    regen,
		spinner:  spinner.New(),
		textarea: ta,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func regenerateCmd(regen func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		text, err := regen()
		return regenResultMsg{text: text, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		m.accepted = false
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateShowing:
			switch msg.String() {
			case "enter":
				m.accepted = true
				return m, tea.Quit
			case "e":
				m.state = stateEditing
				m.textarea.SetValue(m.message)
				cmd := m.textarea.Focus()
				return m, cmd
			case "r":
				m.state = stateRegenerating
				return m, tea.Batch(m.spinner.Tick, regenerateCmd(m.regen))
			case "q":
				m.accepted = false
				return m, tea.Quit
			}
		case stateEditing:
			switch msg.Type {
			case tea.KeyEnter:
				m.message = m.textarea.Value()
				m.state = stateShowing
				return m, nil
			case tea.KeyEsc:
				m.state = stateShowing
				return m, nil
			}
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	case regenResultMsg:
		m.message = msg.text
		m.err = msg.err
		m.state = stateShowing
		return m, nil
	case spinner.TickMsg:
		if m.state == stateRegenerating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateEditing:
		return m.textarea.View() + "\n[enter] save  [esc] discard\n"
	case stateRegenerating:
		return m.spinner.View() + " regenerating...\n"
	default:
		note := ""
		if m.err != nil {
			note = "\n(regenerate failed, showing fallback message)\n"
		}
		return m.message + note + "\n[enter] accept  [e] edit  [r] regenerate  [q/ctrl+c] cancel\n"
	}
}

// Run launches the interactive preview screen and blocks until the user
// accepts or cancels. regen is called (possibly more than once) whenever
// the user asks to regenerate; it is expected to always return usable text
// even when err is non-nil (e.g. a fallback message), since that text is
// what gets displayed alongside the failure note.
func Run(initial string, regen func() (string, error)) (final string, accepted bool, err error) {
	p := tea.NewProgram(newModel(initial, regen))
	result, err := p.Run()
	if err != nil {
		return "", false, err
	}
	m := result.(model)
	return m.message, m.accepted, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/... -v`
Expected: PASS (all preview model tests)

- [ ] **Step 6: Commit**

```bash
git add internal/tui/preview.go internal/tui/preview_test.go go.mod go.sum
git commit -m "feat: add commit-preview TUI model (accept/edit/regenerate/cancel)"
```

`// ponytail: no lipgloss styling - plain text View(). Add color/layout when someone actually asks for visual polish.`

---

### Task 3: `tocommit config` wizard

**Files:**
- Create: `cmd/config.go`
- Create: `cmd/config_test.go`

**Interfaces:**
- Consumes: `config.Config` (all exported fields), `config.LoadFrom(path string) (config.Config, error)`, `config.Default()` - all from Task 1 / already-shipped v1 code. No dependency on `internal/tui`.
- Produces: nothing consumed by other tasks - `tocommit config` is a standalone leaf command.

- [ ] **Step 1: Add dependency**

```bash
go get github.com/charmbracelet/huh
```

- [ ] **Step 2: Write the failing tests**

Create `cmd/config_test.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"gopkg.in/yaml.v3"
)

func TestSaveConfig_WritesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Config{Provider: "groq", Model: "llama-3.3-70b-versatile", TimeoutMS: 1234, TUI: false, APIKey: "should-never-be-written"}
	if err := saveConfig(cfg, path); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}

	var got config.Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshaling saved config: %v", err)
	}
	if got.Provider != "groq" || got.Model != "llama-3.3-70b-versatile" || got.TimeoutMS != 1234 || got.TUI != false {
		t.Errorf("saved config = %+v, want provider=groq model=llama-3.3-70b-versatile timeout_ms=1234 tui=false", got)
	}
	if strings.Contains(string(data), "should-never-be-written") {
		t.Error("saved YAML contains the API key - it must never be written to the config file")
	}
}

func TestSaveConfig_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "config.yaml")

	cfg := config.Default()
	if err := saveConfig(cfg, path); err != nil {
		t.Fatalf("saveConfig returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to exist at %s: %v", path, err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./cmd/... -run 'TestSaveConfig' -v`
Expected: FAIL (compile error: `saveConfig` undefined)

- [ ] **Step 4: Write the implementation**

Create `cmd/config.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactively edit tocommit's config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := configFilePath()
		if err != nil {
			return err
		}

		cfg, err := config.LoadFrom(path)
		if err != nil {
			return err
		}

		timeoutStr := strconv.Itoa(cfg.TimeoutMS)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Provider").Value(&cfg.Provider),
				huh.NewInput().Title("Model").Value(&cfg.Model),
				huh.NewInput().Title("Timeout (ms)").Value(&timeoutStr).Validate(func(s string) error {
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("must be a whole number of milliseconds")
					}
					return nil
				}),
				huh.NewConfirm().Title("Enable interactive commit preview (TUI)?").Value(&cfg.TUI),
			),
		)

		if err := form.Run(); err != nil {
			return fmt.Errorf("running config form: %w", err)
		}

		cfg.TimeoutMS, _ = strconv.Atoi(timeoutStr)

		if err := saveConfig(cfg, path); err != nil {
			return err
		}
		fmt.Println("Saved config to", path)
		return nil
	},
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tocommit", "config.yaml"), nil
}

func saveConfig(cfg config.Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/... -run 'TestSaveConfig' -v`
Expected: PASS

- [ ] **Step 6: Run the full test suite to make sure nothing else broke**

Run: `go build ./... && go test ./...`
Expected: PASS (build succeeds, all packages green)

- [ ] **Step 7: Commit**

```bash
git add cmd/config.go cmd/config_test.go go.mod go.sum
git commit -m "feat: add tocommit config wizard"
```

---

### Task 4: Wire the preview TUI into `cmd/run.go`

**Files:**
- Modify: `cmd/run.go`
- Modify: `cmd/run_test.go`

**Interfaces:**
- Consumes: `config.Config.TUI` (Task 1), `tui.Run(initial string, regen func() (string, error)) (string, bool, error)` (Task 2).
- Produces: `hookDeps.IsTTY func() bool` and `hookDeps.RunTUI func(string, func() (string, error)) (string, bool, error)` - new fields on the existing struct, used only within `cmd/run.go` itself.

- [ ] **Step 1: Add dependency**

```bash
go get github.com/mattn/go-isatty
```

- [ ] **Step 2: Write the failing tests**

First, extend the shared `fakeClient` in `cmd/run_test.go` to support returning a different message on each successive call (needed to prove the regenerate closure calls `Generate` again rather than replaying the first response). Replace the existing `fakeClient` struct and `Generate` method with:

```go
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
```

This is backward compatible: every existing test sets `message` and leaves `messages` nil, so `len(f.messages) > 0` is false and behavior is unchanged.

Next, update `testDeps` to set the new `hookDeps` fields, defaulting to non-TTY (so all five existing tests keep passing unchanged - non-TTY means the TUI branch never runs):

```go
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
```

Now add the new tests to `cmd/run_test.go`:

```go
func TestRunHook_SkipsTUIWhenNotTTY(t *testing.T) {
	dir := t.TempDir()
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte(""), 0o644)

	deps := testDeps(diff.Stats{FilesChanged: 1, Insertions: 3, Deletions: 1}, &fakeClient{message: "feat: plain message"})
	deps.Config = func() (config.Config, error) {
		return config.Config{Provider: "groq", Model: "m", TimeoutMS: 2500, APIKey: "k", TUI: true}, nil
	}

	if err := runHook(msgFile, "", deps); err != nil {
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

	if err := runHook(msgFile, "", deps); err != nil {
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

	if err := runHook(msgFile, "", deps); err != nil {
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

	err := runHook(msgFile, "", deps)
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

	if err := runHook(msgFile, "", deps); err != nil {
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

	if err := runHook(msgFile, "", deps); err != nil {
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

	if err := runHook(msgFile, "", deps); err != nil {
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
```

Add `"errors"` to the imports of `cmd/run_test.go`.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./cmd/... -v`
Expected: FAIL (compile errors - `hookDeps.IsTTY`/`RunTUI` don't exist yet)

- [ ] **Step 4: Wire it into `cmd/run.go`**

Replace the full contents of `cmd/run.go`:

```go
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

	if deps.IsTTY() && cfg.TUI {
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/... -v`
Expected: PASS (all `cmd` tests, old and new)

- [ ] **Step 6: Run the full suite and build**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: everything green

- [ ] **Step 7: Commit**

```bash
git add cmd/run.go cmd/run_test.go go.mod go.sum
git commit -m "feat: wire commit-preview TUI into the hook (TTY + config gated)"
```

---

## Self-Review Notes

- **Spec coverage:** config wizard (Task 3), preview TUI states/transitions (Task 2), TTY+config gating and v1-unchanged fallback (Task 4), new `tui` config field (Task 1) - every section of the spec maps to a task. The spec's "Deferred" list (Ollama, retry, extra config fields) has intentionally no task.
- **Type consistency checked:** `tui.Run` signature in Task 2's `Run` matches exactly what Task 4's `hookDeps.RunTUI` field type expects (`func(string, func() (string, error)) (string, bool, error)`). `config.Config.TUI` (Task 1) is the exact field Task 3's wizard binds to and Task 4's gating check reads.
- **Fallback text ownership:** `internal/tui` never computes a fallback message itself - `regen`'s caller (`cmd/run.go`) is always responsible for substituting `stats.FallbackMessage()` when `client.Generate` errors, keeping the `tui` package free of any `diff`/`llm` knowledge, per the Global Constraints.
