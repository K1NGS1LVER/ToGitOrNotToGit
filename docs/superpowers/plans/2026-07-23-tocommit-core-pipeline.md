# tocommit Core Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the v1 `tocommit` CLI: a git `prepare-commit-msg` hook that scores staged diffs by severity, asks Groq for a theatrical conventional-commit message, and falls back to a plain message if Groq fails or times out.

**Architecture:** Cobra CLI (`cmd` package) wires four independent `internal` packages - `config` (YAML+env), `diff` (git exec + parsing + fallback formatting), `severity` (pure scoring function), `llm` (Groq HTTP client behind a `Client` interface) - together in `cmd/run.go`. `tocommit install`/`uninstall` manage the hook file itself.

**Tech Stack:** Go 1.22+, Cobra (`github.com/spf13/cobra`), `gopkg.in/yaml.v3`, stdlib `net/http`/`os/exec` only otherwise.

## Global Constraints

- Module path: `github.com/K1NGS1LVER/ToGitOrNotToGit`. Binary name: `tocommit`.
- Go 1.22+.
- No LangChain-go, no agent framework - single request/response HTTP call only.
- No Ollama implementation in v1 (interface allows it later, no code for it now).
- No TUI (`huh`/`bubbletea`/`lipgloss`) in v1.
- LLM provider fixed to Groq, OpenAI-compatible endpoint `https://api.groq.com/openai/v1/chat/completions`.
- Config file: `~/.config/tocommit/config.yaml`. Fields: `provider`, `model`, `timeout_ms`. Defaults: `groq`, `llama-3.3-70b-versatile`, `2500`.
- `GROQ_API_KEY` comes from the environment only - never read from or written to the YAML file.
- Severity tiers on `insertions + deletions`: `< 10` Minor, `10-100` Medium, `> 100` Catastrophic. Any changed file matching `go.mod`, `go.sum`, `Dockerfile`, `Makefile`, or `.github/workflows/*` forces Catastrophic regardless of line count.
- Fallback message (used whenever the LLM call errors) is always `chore: update N file(s) (+X/-Y)` - deterministic, no persona, no LLM call.
- Hook bypass: `prepare-commit-msg`'s `$2` source of `message` (covers `-m`/`-F`) or `commit` (covers `-c`/`-C`/`--amend`) skips generation entirely, exit 0, file untouched. `merge`/`template`/`squash` are NOT bypassed.
- Raw diff text is truncated to 8000 bytes before going into any LLM prompt.
- LLM call timeout default 2500ms via `context.WithTimeout`; no retries in v1.

---

### Task 1: `internal/config` - module init + config loading

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config{Provider, Model string; TimeoutMS int; APIKey string}`, `config.Default() Config`, `config.LoadFrom(path string) (Config, error)`, `config.Load() (Config, error)`.

- [ ] **Step 1: Initialize the Go module and add the YAML dependency**

```bash
go mod init github.com/K1NGS1LVER/ToGitOrNotToGit
go get gopkg.in/yaml.v3
```

Expected: `go.mod` created, `go.sum` created, no errors.

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Provider != "groq" {
		t.Errorf("Provider = %q, want groq", cfg.Provider)
	}
	if cfg.Model != "llama-3.3-70b-versatile" {
		t.Errorf("Model = %q, want llama-3.3-70b-versatile", cfg.Model)
	}
	if cfg.TimeoutMS != 2500 {
		t.Errorf("TimeoutMS = %d, want 2500", cfg.TimeoutMS)
	}
}

func TestLoadFrom_MissingFile_ReturnsDefaults(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults %+v", cfg, Default())
	}
}

func TestLoadFrom_OverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "provider: groq\nmodel: custom-model\ntimeout_ms: 1000\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want custom-model", cfg.Model)
	}
	if cfg.TimeoutMS != 1000 {
		t.Errorf("TimeoutMS = %d, want 1000", cfg.TimeoutMS)
	}
}

func TestLoadFrom_ReadsAPIKeyFromEnv(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key-123")
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.APIKey != "test-key-123" {
		t.Errorf("APIKey = %q, want test-key-123", cfg.APIKey)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/...`
Expected: FAIL - `undefined: Default` (package doesn't exist yet)

- [ ] **Step 4: Write minimal implementation**

`internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	TimeoutMS int    `yaml:"timeout_ms"`
	APIKey    string `yaml:"-"`
}

func Default() Config {
	return Config{
		Provider:  "groq",
		Model:     "llama-3.3-70b-versatile",
		TimeoutMS: 2500,
	}
}

// LoadFrom reads config from an explicit path. A missing file yields defaults.
func LoadFrom(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if uerr := yaml.Unmarshal(data, &cfg); uerr != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, uerr)
		}
	case os.IsNotExist(err):
		// no config file - use defaults
	default:
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg.APIKey = os.Getenv("GROQ_API_KEY")
	return cfg, nil
}

// Load reads config from the standard user location.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolving home directory: %w", err)
	}
	return LoadFrom(filepath.Join(home, ".config", "tocommit", "config.yaml"))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/internal/config`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config
git commit -m "feat: add config loading (YAML + env API key)"
```

---

### Task 2: `internal/diff` - collect, parse, and fallback-format the staged diff

**Files:**
- Create: `internal/diff/diff.go`
- Test: `internal/diff/diff_test.go`

**Interfaces:**
- Produces: `diff.Stats{FilesChanged, Insertions, Deletions int; ChangedFiles []string}`, `Stats.TotalLines() int`, `Stats.FallbackMessage() string`, `diff.Collect() (Stats, string, error)`.

- [ ] **Step 1: Write the failing test**

`internal/diff/diff_test.go`:

```go
package diff

import "testing"

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    Stats
	}{
		{
			name:   "single text file",
			output: "3\t1\tmain.go\n",
			want: Stats{
				FilesChanged: 1,
				Insertions:   3,
				Deletions:    1,
				ChangedFiles: []string{"main.go"},
			},
		},
		{
			name:   "multiple files",
			output: "10\t2\ta.go\n5\t0\tb.go\n",
			want: Stats{
				FilesChanged: 2,
				Insertions:   15,
				Deletions:    2,
				ChangedFiles: []string{"a.go", "b.go"},
			},
		},
		{
			name:   "binary file uses dash markers",
			output: "-\t-\tlogo.png\n",
			want: Stats{
				FilesChanged: 1,
				Insertions:   0,
				Deletions:    0,
				ChangedFiles: []string{"logo.png"},
			},
		},
		{
			name:   "empty output means no staged changes",
			output: "",
			want:   Stats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNumstat(tt.output)
			if err != nil {
				t.Fatalf("parseNumstat returned error: %v", err)
			}
			if got.FilesChanged != tt.want.FilesChanged ||
				got.Insertions != tt.want.Insertions ||
				got.Deletions != tt.want.Deletions ||
				len(got.ChangedFiles) != len(tt.want.ChangedFiles) {
				t.Errorf("parseNumstat(%q) = %+v, want %+v", tt.output, got, tt.want)
			}
		})
	}
}

func TestStats_FallbackMessage(t *testing.T) {
	s := Stats{FilesChanged: 3, Insertions: 12, Deletions: 4}
	got := s.FallbackMessage()
	want := "chore: update 3 file(s) (+12/-4)"
	if got != want {
		t.Errorf("FallbackMessage() = %q, want %q", got, want)
	}
}

func TestStats_TotalLines(t *testing.T) {
	s := Stats{Insertions: 7, Deletions: 3}
	if s.TotalLines() != 10 {
		t.Errorf("TotalLines() = %d, want 10", s.TotalLines())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/diff/...`
Expected: FAIL - `undefined: parseNumstat`

- [ ] **Step 3: Write minimal implementation**

`internal/diff/diff.go`:

```go
package diff

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const maxDiffBytes = 8000

type Stats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
	ChangedFiles []string
}

func (s Stats) TotalLines() int {
	return s.Insertions + s.Deletions
}

func (s Stats) FallbackMessage() string {
	return fmt.Sprintf("chore: update %d file(s) (+%d/-%d)", s.FilesChanged, s.Insertions, s.Deletions)
}

// Collect runs git against the staged changes in the current working directory.
func Collect() (Stats, string, error) {
	numstatOut, err := runGit("diff", "--cached", "--numstat")
	if err != nil {
		return Stats{}, "", fmt.Errorf("running git diff --numstat: %w", err)
	}

	stats, err := parseNumstat(numstatOut)
	if err != nil {
		return Stats{}, "", err
	}

	rawDiff, err := runGit("diff", "--cached")
	if err != nil {
		return Stats{}, "", fmt.Errorf("running git diff: %w", err)
	}
	if len(rawDiff) > maxDiffBytes {
		rawDiff = rawDiff[:maxDiffBytes] + "\n... (truncated)"
	}

	return stats, rawDiff, nil
}

func parseNumstat(output string) (Stats, error) {
	var s Stats

	trimmed := strings.TrimRight(output, "\n")
	if trimmed == "" {
		return s, nil
	}

	for _, line := range strings.Split(trimmed, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return Stats{}, fmt.Errorf("unexpected numstat line: %q", line)
		}

		s.FilesChanged++
		s.ChangedFiles = append(s.ChangedFiles, fields[2])

		if fields[0] != "-" {
			n, err := strconv.Atoi(fields[0])
			if err != nil {
				return Stats{}, fmt.Errorf("parsing insertions in %q: %w", line, err)
			}
			s.Insertions += n
		}

		if fields[1] != "-" {
			n, err := strconv.Atoi(fields[1])
			if err != nil {
				return Stats{}, fmt.Errorf("parsing deletions in %q: %w", line, err)
			}
			s.Deletions += n
		}
	}

	return s, nil
}

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/diff/...`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff`

- [ ] **Step 5: Commit**

```bash
git add internal/diff
git commit -m "feat: add diff collection, numstat parsing, fallback message"
```

---

### Task 3: `internal/severity` - score a diff into a persona tier

**Files:**
- Create: `internal/severity/severity.go`
- Test: `internal/severity/severity_test.go`

**Interfaces:**
- Consumes: `diff.Stats` (Task 2) - fields `TotalLines() int`, `ChangedFiles []string`.
- Produces: `severity.Tier` (`Minor`, `Medium`, `Catastrophic`), `Tier.Persona() string`, `severity.Score(stats diff.Stats) Tier`.

- [ ] **Step 1: Write the failing test**

`internal/severity/severity_test.go`:

```go
package severity

import (
	"testing"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
)

func TestScore_LineCountTiers(t *testing.T) {
	tests := []struct {
		name       string
		insertions int
		deletions  int
		want       Tier
	}{
		{"nine lines is minor", 5, 4, Minor},
		{"ten lines is medium", 6, 4, Medium},
		{"hundred lines is medium", 60, 40, Medium},
		{"hundred one lines is catastrophic", 60, 41, Catastrophic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := diff.Stats{Insertions: tt.insertions, Deletions: tt.deletions}
			if got := Score(stats); got != tt.want {
				t.Errorf("Score(%+v) = %v, want %v", stats, got, tt.want)
			}
		})
	}
}

func TestScore_CoreFileOverride(t *testing.T) {
	tests := []string{"go.mod", "go.sum", "Dockerfile", "Makefile", ".github/workflows/ci.yml"}
	for _, f := range tests {
		t.Run(f, func(t *testing.T) {
			stats := diff.Stats{Insertions: 1, Deletions: 0, ChangedFiles: []string{f}}
			if got := Score(stats); got != Catastrophic {
				t.Errorf("Score with changed file %q = %v, want Catastrophic", f, got)
			}
		})
	}
}

func TestTier_Persona(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{Minor, "victorian-gothic"},
		{Medium, "soap-opera"},
		{Catastrophic, "shakespearean-tragedy"},
	}
	for _, tt := range tests {
		if got := tt.tier.Persona(); got != tt.want {
			t.Errorf("Tier(%d).Persona() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/severity/...`
Expected: FAIL - `undefined: Score`

- [ ] **Step 3: Write minimal implementation**

`internal/severity/severity.go`:

```go
package severity

import (
	"path/filepath"
	"strings"

	"github.com/K1NGS1LVER/ToGitOrNotToGit/internal/diff"
)

type Tier int

const (
	Minor Tier = iota
	Medium
	Catastrophic
)

func (t Tier) Persona() string {
	switch t {
	case Minor:
		return "victorian-gothic"
	case Catastrophic:
		return "shakespearean-tragedy"
	default:
		return "soap-opera"
	}
}

var overrideBasenames = []string{"go.mod", "go.sum", "Dockerfile", "Makefile"}

func isOverrideFile(path string) bool {
	base := filepath.Base(path)
	for _, name := range overrideBasenames {
		if base == name {
			return true
		}
	}
	return strings.HasPrefix(path, ".github/workflows/")
}

func Score(stats diff.Stats) Tier {
	for _, f := range stats.ChangedFiles {
		if isOverrideFile(f) {
			return Catastrophic
		}
	}

	switch total := stats.TotalLines(); {
	case total < 10:
		return Minor
	case total <= 100:
		return Medium
	default:
		return Catastrophic
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/severity/...`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/internal/severity`

- [ ] **Step 5: Commit**

```bash
git add internal/severity
git commit -m "feat: add severity scoring with core-file override"
```

---

### Task 4: `internal/llm` - Client interface + Groq implementation

**Files:**
- Create: `internal/llm/client.go`
- Create: `internal/llm/groq.go`
- Test: `internal/llm/groq_test.go`

**Interfaces:**
- Produces: `llm.Request{Persona, Stats, Diff string}`, `llm.Client` interface (`Generate(ctx context.Context, req Request) (string, error)`), `llm.NewGroqClient(apiKey, model string) *GroqClient`, `GroqClient.BaseURL string` (overridable for tests).

- [ ] **Step 1: Write the failing test**

`internal/llm/groq_test.go`:

```go
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGroqClient_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}
		resp := chatResponse{}
		resp.Choices = []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: "feat(auth): a tale of two tokens"}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	got, err := client.Generate(context.Background(), Request{Persona: "soap-opera", Stats: "1 file", Diff: "+x"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if got != "feat(auth): a tale of two tokens" {
		t.Errorf("Generate() = %q, want the canned message", got)
	}
}

func TestGroqClient_Generate_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	_, err := client.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want error for 500 response")
	}
}

func TestGroqClient_Generate_MissingAPIKey(t *testing.T) {
	client := NewGroqClient("", "test-model")
	_, err := client.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want error for missing API key")
	}
}

func TestGroqClient_Generate_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := client.Generate(ctx, Request{})
	if err == nil {
		t.Fatal("Generate returned nil error, want timeout error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("error = %v, want context deadline exceeded", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/...`
Expected: FAIL - `undefined: NewGroqClient`

- [ ] **Step 3: Write minimal implementation**

`internal/llm/client.go`:

```go
package llm

import "context"

type Request struct {
	Persona string
	Stats   string
	Diff    string
}

type Client interface {
	Generate(ctx context.Context, req Request) (string, error)
}
```

`internal/llm/groq.go`:

```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type GroqClient struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewGroqClient(apiKey, model string) *GroqClient {
	return &GroqClient{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    "https://api.groq.com/openai/v1/chat/completions",
		HTTPClient: &http.Client{},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func systemPrompt(persona string) string {
	return fmt.Sprintf(
		"You are a commit-message generator writing in the %s persona. "+
			"Output a conventional commit: first line 'type(scope): summary', "+
			"then a blank line, then a short dramatic monologue body in that persona. "+
			"Valid types: feat, fix, chore, refactor, docs, test, build, ci.",
		persona,
	)
}

func (c *GroqClient) Generate(ctx context.Context, req Request) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("groq: missing API key")
	}

	body := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(req.Persona)},
			{Role: "user", Content: fmt.Sprintf("Diff stats: %s\n\nDiff:\n%s", req.Stats, req.Diff)},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("groq: marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("groq: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("groq: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq: unexpected status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("groq: decoding response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("groq: empty response")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/llm/...`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/internal/llm`

- [ ] **Step 5: Commit**

```bash
git add internal/llm
git commit -m "feat: add LLM Client interface and Groq implementation"
```

---

### Task 5: `cmd` - root command + `run` wiring

**Files:**
- Create: `cmd/root.go`
- Create: `cmd/run.go`
- Test: `cmd/run_test.go`

**Interfaces:**
- Consumes: `config.Config`/`config.Load` (Task 1), `diff.Stats`/`diff.Collect` (Task 2), `severity.Score`/`Tier.Persona()` (Task 3), `llm.Client`/`llm.Request`/`llm.NewGroqClient` (Task 4).
- Produces: `cmd.Execute()`, unexported `runHook(msgFile, source string, deps hookDeps) error` and `hookDeps` struct (used by Task 7's integration test in the same package).

- [ ] **Step 1: Write the failing test**

`cmd/run_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/...`
Expected: FAIL - `undefined: runHook`

- [ ] **Step 3: Add the Cobra dependency**

```bash
go get github.com/spf13/cobra
```

Expected: `go.mod`/`go.sum` updated, no errors.

- [ ] **Step 4: Write minimal implementation**

`cmd/root.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tocommit",
	Short: "Turns your git commits into theatrical monologues",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

`cmd/run.go`:

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
}

func defaultDeps() hookDeps {
	return hookDeps{
		Config: config.Load,
		Diff:   diff.Collect,
		NewClient: func(cfg config.Config) llm.Client {
			return llm.NewGroqClient(cfg.APIKey, cfg.Model)
		},
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutMS)*time.Millisecond)
	defer cancel()

	message, err := client.Generate(ctx, llm.Request{
		Persona: tier.Persona(),
		Stats:   fmt.Sprintf("%d file(s), +%d/-%d", stats.FilesChanged, stats.Insertions, stats.Deletions),
		Diff:    rawDiff,
	})
	if err != nil {
		message = stats.FallbackMessage()
	}

	return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./cmd/...`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/cmd`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/root.go cmd/run.go cmd/run_test.go
git commit -m "feat: wire diff/severity/llm pipeline into run command"
```

---

### Task 6: `cmd` - `install`/`uninstall` hook management

**Files:**
- Create: `cmd/install.go`
- Test: `cmd/install_test.go`

**Interfaces:**
- Produces: `installCmd`, `uninstallCmd` (registered on `rootCmd` from Task 5), unexported `hookPath() (string, error)`.

- [ ] **Step 1: Write the failing test**

`cmd/install_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/... -run TestInstall`
Expected: FAIL - `undefined: installCmd`

- [ ] **Step 3: Write minimal implementation**

`cmd/install.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/... -run TestInstall`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/cmd`

- [ ] **Step 5: Commit**

```bash
git add cmd/install.go cmd/install_test.go
git commit -m "feat: add install/uninstall commands for the git hook"
```

---

### Task 7: `main.go` entrypoint + end-to-end integration test

**Files:**
- Create: `main.go`
- Test: `cmd/integration_test.go`

**Interfaces:**
- Consumes: `cmd.Execute()`, `runHook`/`hookDeps` (Task 5), real `diff.Collect` (Task 2) against an actual temp git repo.

- [ ] **Step 1: Write the failing test**

`cmd/integration_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... && go test ./cmd/... -run TestIntegration`
Expected: FAIL to build - `no Go files in .` (no `main.go` yet, `go build ./...` errors on the root package)

- [ ] **Step 3: Write minimal implementation**

`main.go`:

```go
package main

import "github.com/K1NGS1LVER/ToGitOrNotToGit/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./... && go test ./cmd/... -run TestIntegration`
Expected: PASS - `ok  	github.com/K1NGS1LVER/ToGitOrNotToGit/cmd`

- [ ] **Step 5: Run the full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS on all packages - `internal/config`, `internal/diff`, `internal/severity`, `internal/llm`, `cmd`

- [ ] **Step 6: Commit**

```bash
git add main.go cmd/integration_test.go
git commit -m "feat: add main entrypoint and end-to-end hook integration test"
```

---

## Manual smoke test (after Task 7)

Not a substitute for the automated tests above, but confirms the real binary behaves against a real repo and a real Groq key:

```bash
go build -o /tmp/tocommit .
cd /tmp && mkdir smoke-test && cd smoke-test && git init
/tmp/tocommit install
echo "package main" > main.go
git add main.go
export GROQ_API_KEY=<your key>
git commit
```

Expected: the commit opens with a generated conventional-commit + monologue as the pre-filled message (small diff -> Victorian Gothic persona). Unset `GROQ_API_KEY` and repeat to confirm the fallback message (`chore: update 1 file(s) (+1/-0)`) appears instead.
