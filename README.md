# 🎭 ToGitOrNotToGit

> _"Out, damn bug! Out, I say!"_

**ToGitOrNotToGit** (binary name `tocommit`) is a local Git hook CLI tool that inspects your staged changes and asks an LLM to write your commit message as a theatrical monologue instead of `fix typo` or `updated config`.

## How it works

On `git commit`, the installed hook runs `tocommit run` before the editor opens.
It reads `git diff --cached`, scores the change's severity, and asks Groq for a persona-appropriate conventional-commit message.
On a real terminal, that message opens in an interactive preview screen (see Commit preview, below) before it ever reaches git's editor; otherwise it's written straight in, like v1.
If Groq errors or times out, it silently falls back to a plain deterministic message — you are never blocked waiting on the network.

### Severity tiers

Based on total changed lines (`insertions + deletions`), unless a core file is touched:

| Tier | Trigger | Persona |
|---|---|---|
| Minor | < 10 lines | Victorian Gothic |
| Medium | 10-100 lines | Soap Opera |
| Catastrophic | > 100 lines, **or** any changed file matches `go.mod`, `go.sum`, `Dockerfile`, `Makefile`, or `.github/workflows/*` | Shakespearean Tragedy |

### Fallback

If the Groq call errors, times out, or `GROQ_API_KEY` isn't set, the hook writes a plain message instead of blocking your commit:
```
chore: update 2 file(s) (+15/-3)
```

### Bypass

The hook skips itself entirely (exits without touching the message) when Git invokes it for `-m`/`-F` or `--amend`/`-c`/`-C` — it only fires for a normal interactive `git commit`.

## Install

Requires Go 1.22+ and a [Groq](https://console.groq.com) API key.

```bash
go install ./cmd/tocommit
```

This installs a `tocommit` binary to `$(go env GOPATH)/bin` — make sure that's on your `PATH`.

Set your API key (never stored in any config file, environment variable only):
```bash
export GROQ_API_KEY=gsk_your_key_here
```

In any git repo you want the theatrics in:
```bash
cd /path/to/repo
tocommit install
```

This writes `.git/hooks/prepare-commit-msg` in that repo. It refuses to overwrite a pre-existing hook that isn't already tocommit's (checked via a marker comment), so it won't clobber a hook you already have.

To remove it:
```bash
tocommit uninstall
```

## Usage

Just commit normally:
```bash
git add .
git commit
```

Don't pass `-m` — that bypasses generation by design (see Bypass, above). If you're on a real terminal and haven't disabled it (see Commit preview, below), a preview screen opens with the generated message. Otherwise the message goes straight into the editor like v1 — edit or accept it like any normal commit message.

## Commit preview

On an interactive `git commit` (a real terminal, `tui: true` in config, which is the default), instead of handing the message straight to git's editor, tocommit shows it in a preview screen first:

| Key | Action |
|---|---|
| `enter` | Accept the message as-is |
| `e` | Edit — opens the message in a text box (`ctrl+s` saves, `esc` discards the edit) |
| `r` | Regenerate — calls Groq again (spends another API call, respects the same timeout) |
| `q` / `ctrl+c` | Cancel — aborts the commit, nothing is written |

If Groq fails during a regenerate, the screen falls back to the plain deterministic message and keeps you in control (accept/edit/cancel still work) — the same never-block-you philosophy as the base pipeline.

Skipped automatically when stdin/stdout isn't a real terminal (scripts, CI) — v1's plain "write straight to the file" behavior kicks in instead, unchanged. Turn it off entirely with `tui: false` in config.

## Config wizard

```bash
tocommit config
```

Interactively edit `~/.config/tocommit/config.yaml` — walks through `provider`, `model`, `timeout_ms`, and `tui`, then saves. Equivalent to hand-editing the YAML below; never touches `GROQ_API_KEY`.

## Configuration (optional)

Defaults work with nothing but `GROQ_API_KEY` set. To override, create `~/.config/tocommit/config.yaml` by hand or via `tocommit config`:

```yaml
provider: groq
model: llama-3.3-70b-versatile
timeout_ms: 2500
tui: true
```

| Field | Default | Notes |
|---|---|---|
| `provider` | `groq` | Only `groq` is implemented in v1. |
| `model` | `llama-3.3-70b-versatile` | Any Groq-hosted chat model. |
| `timeout_ms` | `2500` | How long to wait for Groq before falling back. No retries. |
| `tui` | `true` | Show the commit-preview screen on an interactive commit. Set `false` for v1's plain behavior. |

`GROQ_API_KEY` is always read from the environment, never from this file.

## What's not built yet

- No local/offline LLM support (Ollama) — Groq only
- No retry logic on transient LLM failures — one attempt, then fallback

## Project layout

```
cmd/tocommit/main.go      entrypoint (go install ./cmd/tocommit)
cmd/root.go               Cobra root command
cmd/run.go                the hook: diff -> severity -> LLM -> fallback -> preview -> write
cmd/install.go            install/uninstall the git hook
cmd/config.go             tocommit config wizard
internal/config/          YAML + env config loading
internal/diff/            git diff exec, numstat parsing, fallback message
internal/severity/        line-count + core-file severity scoring
internal/llm/             LLM Client interface + Groq implementation
internal/tui/             commit-preview screen (accept/edit/regenerate/cancel)
```

Design spec and implementation plan: [`docs/superpowers/`](docs/superpowers/).
