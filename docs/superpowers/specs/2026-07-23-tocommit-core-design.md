# tocommit v1 (core pipeline) - Design

## Goal

A Go CLI (`tocommit`) installable as a git `prepare-commit-msg` hook.
On `git commit`, it reads the staged diff, scores its severity, asks Groq for a theatrical conventional-commit message in a persona matching that severity, and writes it into `COMMIT_EDITMSG`.
If Groq fails or times out, it falls back to a plain, honest conventional-commit message so the developer is never blocked.

v1 scope is the core pipeline only: hook install, diff/severity engine, Groq client, fallback, YAML+env config.
No `huh` config wizard and no `bubbletea` interactive preview yet - both are explicitly deferred to v2.

## Non-goals (v1)

- Ollama / local LLM support (interface leaves room for it, no implementation)
- Interactive TUI wizard or commit preview
- LangChain-go or any agent framework (single request/response call, no tool use or chaining)

## Architecture

```
git commit
   -> .git/hooks/prepare-commit-msg  (installed by `tocommit install`)
        exec tocommit run "$1"       ($1 = path to COMMIT_EDITMSG)
             |
             v
        cmd/run.go
             |
             v
        diff.Collect()          git diff --cached --numstat + git diff --cached (truncated)
             |
             v
        severity.Score(stats)   -> Minor | Medium | Catastrophic
             |
             v
        llm.Generate(ctx, diff, severity)   context.WithTimeout(2500ms)
             |            \
             | success      \ error/timeout
             v               v
        write message    fallback.PlainMessage(stats)
             |
             v
        overwrite COMMIT_EDITMSG file
```

## Components

### 1. `cmd/` (Cobra)

- `tocommit run <commit-msg-file>` - the hook entrypoint. Never returns non-zero on LLM failure (falls back instead); only exits non-zero on unrecoverable errors (e.g. can't read the diff at all, can't write the file).
- `tocommit install` - writes `.git/hooks/prepare-commit-msg`:
  ```sh
  #!/bin/sh
  exec tocommit run "$1"
  ```
  Fails loudly if a hook already exists there and isn't ours (checks for a marker comment), so it never silently clobbers another hook.
- `tocommit uninstall` - removes the hook if it's ours.
- Bypass: if git invoked the hook with a commit source of `message` or `merge` (git passes this as `$2`), `tocommit run` exits 0 immediately without touching the file - covers `-m` and `--amend` per the original requirement, using git's own mechanism rather than re-parsing flags.

### 2. `internal/diff`

- Runs `git diff --cached --numstat` -> parses added/deleted/files-changed counts.
- Runs `git diff --cached` -> truncates to a fixed byte budget (e.g. 8000 chars) so prompt size and latency stay bounded on huge diffs.
- Returns a `Stats{FilesChanged, Insertions, Deletions, ChangedFiles []string}` and the truncated raw diff text.

### 3. `internal/severity`

Pure function `Score(stats Stats) Tier`, two rules, first match wins:

1. **Core-file override:** if any changed file matches a fixed override list (`go.mod`, `go.sum`, `Dockerfile`, `Makefile`, CI config paths like `.github/workflows/*`) -> `Catastrophic`.
2. **Line-count tiers** on `Insertions + Deletions`:
   - `< 10` -> `Minor`
   - `10-100` -> `Medium`
   - `> 100` -> `Catastrophic`

Each `Tier` maps to a persona name (`victorian-gothic`, `soap-opera`, `shakespearean-tragedy`) used in the prompt.

### 4. `internal/llm`

- `Client` interface: `Generate(ctx context.Context, req Request) (string, error)`.
- `groq.Client` - the only implementation in v1. Plain `net/http` POST to `https://api.groq.com/openai/v1/chat/completions` (OpenAI-compatible schema), model + API key from config.
- Prompt template fixed in code: system prompt sets persona + enforces `type(scope): summary` first line + conventional-commit types, user message carries diff stats + truncated diff.
- Caller wraps the call in `context.WithTimeout` (config default 2500ms); on any error (timeout, non-200, malformed response) returns the error untouched - no retries in v1 (nothing later depends on retry behavior; add if false-positive timeouts turn out to be a real problem).

### 5. `internal/fallback`

- `PlainMessage(stats Stats) string` - builds `chore: update N files (+X/-Y)` (or `fix`/`feat` if inferable is out of scope - always `chore` in v1, keeps this deterministic and dumb on purpose) with no LLM call. Used whenever `llm.Generate` errors.

### 6. `internal/config`

- Loads `~/.config/tocommit/config.yaml`:
  ```yaml
  provider: groq
  model: llama-3.3-70b-versatile
  timeout_ms: 2500
  ```
- `GROQ_API_KEY` read from env only - never stored in the YAML file.
- Missing file -> built-in defaults, so `tocommit run` works with zero setup beyond the env var.

## Data flow / error handling summary

| Failure point | Behavior |
|---|---|
| Not a git repo / no staged changes | `run` exits 0, leaves message file untouched |
| `git diff` exec fails | exit non-zero, git shows its own error - this is an environment problem, not one tocommit should paper over |
| Groq timeout/error/missing API key | fallback plain message, exit 0 |
| Can't write commit-msg file | exit non-zero |
| Hook already installed (not ours) on `tocommit install` | exit non-zero with explanation, no overwrite |

## Testing

- `internal/severity`: table-driven unit tests over the tier boundaries and the override list.
- `internal/diff`: unit test the `numstat` parser against fixture output strings (no real git exec needed for the parser itself).
- `internal/llm/groq`: unit test request building and response parsing against a `httptest.Server`; a context-cancellation test confirms timeout triggers the error path.
- `internal/fallback`: unit test message formatting.
- One integration-style test for `cmd run`: real temp git repo, staged change, fake LLM client injected, asserts final `COMMIT_EDITMSG` content for both the success and the fallback path.

## Deferred to v2

- `tocommit config` wizard (`huh`)
- Interactive commit preview / regenerate / edit (`bubbletea`, `lipgloss`)
- Ollama provider
- Retry logic for transient LLM errors, if it proves necessary in practice
