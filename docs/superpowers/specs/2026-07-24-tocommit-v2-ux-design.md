# tocommit v2 (config wizard + commit-preview TUI) - Design

## Goal

Ship the two features v1 explicitly deferred: an interactive config wizard (`tocommit config`) and an interactive commit-preview screen that replaces git's editor step for a normal `git commit`.
Both are additive to the v1 pipeline; no v1 behavior changes when the terminal isn't interactive or the feature is disabled.

## Non-goals (v2)

- Ollama / local LLM support - still out of scope, no work here.
- Retry logic for transient LLM errors - not addressed by this design.
- Any new config fields beyond the one this design needs (`tui`).

## Architecture

```
git commit (normal, TTY)
   -> cmd/run.go
        diff.Collect() -> severity.Score() -> llm.Generate() (or fallback on error)
             |
             v
        isatty(stdin) && isatty(stdout) && cfg.TUI == true ?
             |                                    |
            yes                                   no
             |                                    |
             v                                    v
        internal/tui.Run(msg, regenFunc)     write message straight to file
             |                                    (v1 behavior, unchanged)
    +--------+----------+-------------+
    |        |          |             |
  Accept   Edit      Regenerate     Cancel
    |        |          |             |
    v        v          v             v
  write   inline    llm.Generate() again   exit non-zero
  file    textarea,  (spinner shown),      (git aborts commit,
          then       loop back to          file untouched)
          Accept     showing state
```

```
tocommit config          (standalone command, no git/diff/LLM involved)
   -> internal/config.Load() or Default()
   -> huh.Form over {provider, model, timeout_ms, tui}
   -> write ~/.config/tocommit/config.yaml
```

## Components

### 1. `internal/config` (extended)

- New field: `TUI bool` (yaml tag `tui`), defaults to `true` in `Default()`.
- No other changes. `GROQ_API_KEY` continues to be env-only, never written by the wizard.

### 2. `cmd/config.go` (new)

- `tocommit config` Cobra command.
- Loads existing config (or `Default()` if none), pre-fills a `huh.Form` with four fields: provider (text), model (text), timeout_ms (text, numeric validation), tui (confirm).
- On submit, marshals the resulting `config.Config` to YAML and writes `~/.config/tocommit/config.yaml`, creating the directory if needed.
- No huh field for `APIKey` - the form never touches it.

### 3. `internal/tui` (new package)

- `preview.go`: a bubbletea `Model` with three states - `showing`, `regenerating`, `editing`.
  - `showing`: renders the current message text, footer hints for Accept (enter) / Edit (e) / Regenerate (r) / Cancel (ctrl+c or q).
  - `editing`: swaps to a `bubbles/textarea` pre-filled with the current message; Enter commits the edit and returns to `showing`; Esc discards the edit (keeps the pre-edit message) and returns to `showing`.
  - `regenerating`: renders a `bubbles/spinner` while a `tea.Cmd` runs the injected `regenFunc` in the background; on completion, updates the message and returns to `showing`. On error, the message becomes the fallback text (still shown in `showing`, still fully interactive).
  - `ctrl+c` cancels from any state (`showing`, `editing`, `regenerating`) - it always means "abort the commit," never "discard this keystroke."
- Public entry point: `Run(initial string, regenFunc func() (string, error)) (final string, accepted bool, err error)`. `err` is only non-nil if the bubbletea program itself fails to start/run - a genuinely broken terminal, not a Groq failure.
- `regenFunc` is provided by `cmd/run.go` as a closure over the existing `llm.Client`, `context.WithTimeout`, and `llm.Request` - the TUI package has no knowledge of Groq, config, or diffs. It only knows "give me a new string, maybe slowly, maybe it fails."

### 4. `cmd/run.go` (modified)

- After the existing generate-or-fallback logic produces `message`, add:
  ```go
  if isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) && cfg.TUI {
      final, accepted, err := tui.Run(message, regenFunc)
      if err != nil {
          // TUI itself broke - fall back to v1 behavior, don't block the commit
          return os.WriteFile(msgFile, []byte(message+"\n"), 0o644)
      }
      if !accepted {
          return fmt.Errorf("commit cancelled")
      }
      message = final
  }
  ```
- `regenFunc` closes over `client`, `ctx` (a fresh `context.WithTimeout(cfg.TimeoutMS)` per regenerate call, not the original one - each regenerate gets its own full timeout budget), and the same `llm.Request` used for the first call.
- TTY detection uses `github.com/mattn/go-isatty` (new dependency - stdlib has no portable isatty check).

## Data flow / error handling summary

| Case | Behavior |
|---|---|
| No TTY on stdin or stdout | v1 path: write message straight to file, no TUI |
| `tui: false` in config | v1 path, same as above |
| TUI: Accept | write message (possibly edited) to file, exit 0 |
| TUI: Cancel | exit non-zero; git aborts the commit; message file untouched |
| TUI: Regenerate succeeds | message updates in place, stays in `showing` state |
| TUI: Regenerate errors/times out | message becomes the fallback text, stays in `showing` state, still interactive |
| TUI program fails to start/run | fall back to v1 write-straight-to-file path using whatever message was already generated |
| `tocommit config`, no existing config file | form pre-filled from `Default()` |
| `tocommit config` submit | writes/overwrites `~/.config/tocommit/config.yaml`; never touches `GROQ_API_KEY` |

## Testing

- `internal/tui`: unit tests driving the `Model.Update()` state machine directly (Accept, Edit-then-Accept, Regenerate-success, Regenerate-error, Cancel) with a fake `regenFunc` - no real terminal needed, `tea.Model.Update` is a pure function over messages.
- `cmd/config.go`: unit test the load-defaults-or-existing plus save-to-YAML logic with the form's output pre-supplied (not testing huh's interactive rendering itself).
- `cmd/run.go`: extend the existing fake-LLM integration test with a case that forces non-TTY (or `tui: false`) and asserts the v1 write-straight-to-file path still fires unchanged.
- Accepted gap: no automated test drives an actual TTY end-to-end through the real bubbletea program - that's a manual/dogfooding check, consistent with how v1's real-Groq path was verified once by hand rather than in CI.

## Deferred (still not v2)

- Ollama / local LLM support
- Retry logic for transient LLM errors
- Any config fields beyond `tui` (e.g. persona names, severity thresholds) - out of scope until a real need shows up
