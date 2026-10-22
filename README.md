# 🎭 ToGitOrNotToGit (`tocommit`)

[![Go Version](https://img.shields.io/github/go-mod/go-version/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=00ADD8)](https://go.dev)
[![License](https://img.shields.io/github/license/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=blue)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=green)](https://github.com/K1NGS1LVER/ToGitOrNotToGit/releases)

> _"Out, damn bug! Out, I say!"_ — **Macbeth (Act V, Scene I), adapted for Git**

**ToGitOrNotToGit** (binary name: `tocommit`, current release: `v0.4.0`) is a Git hook CLI that inspects your staged changes and uses AI to generate theatrical, dramatic commit messages based on the severity of your code changes.

Transform boring commit messages like `fix typo` or `updated config` into Victorian Gothic laments, Soap Opera betrayals, and Shakespearean Tragedies — right inside your terminal workflow.

---

## 🚀 Key Features

- 🎭 **Theatrical Persona Escalation**: Automatically assigns a persona (*Victorian Gothic*, *Soap Opera*, or *Shakespearean Tragedy*) based on diff line counts and sensitive file changes (`go.mod`, `go.sum`, `Dockerfile`, `Makefile`, `.github/workflows/*`).
- 🖥️ **Interactive Terminal Preview (TUI)**: Review, edit (`e`), regenerate (`r`), or accept (`enter`) generated commit messages interactively before committing.
- ⚡ **Zero-Block Fallback System**: If the configured model is retired or unavailable, `tocommit` first cascades through your `fallback_models`, then writes a clean conventional fallback commit without ever blocking your git flow.
- 🔔 **Explicit Model Alerts**: If every model fails, `tocommit` prints an actionable warning (naming the model and telling you to run `tocommit models` or switch to a local Ollama model) instead of failing silently.
- 🛡️ **Zero-Leak Security**: API keys (`GROQ_API_KEY`, `OPENAI_API_KEY`, or `TOCOMMIT_API_KEY`) are read strictly from environment variables and are **never written to config files or disk**.
- 🛠️ **Smart Bypass**: Automatically skips execution for automated commits (`-m`, `-F`, `--amend`, `merge`, `squash`) to preserve git integrity.
- 🔌 **Multi-Provider**: Works with Groq, OpenAI, local Ollama, or any OpenAI-compatible endpoint via `base_url`.
- 🔍 **Debug Mode**: Set `TOCOMMIT_DEBUG=1` (or pass `--debug`) to see the chosen provider/model/persona and the reason behind any fallback.

---

## ⚡ Quick Start

### 1. Installation

Requires **Go 1.22+**. Install the binary directly:

```zsh
go install github.com/K1NGS1LVER/ToGitOrNotToGit/cmd/tocommit@latest
```

Ensure `$(go env GOPATH)/bin` is in your shell `PATH`.

Alternatively, download pre-built release binaries for macOS, Linux, or Windows directly from the [Releases](https://github.com/K1NGS1LVER/ToGitOrNotToGit/releases) page.

#### Updating an existing installation

The Git hook runs `tocommit` by name, so it uses the first executable named
`tocommit` in your shell's `PATH`. Updating the hook is not required when only
the binary changes.

Update an installation made with `go install`:

```zsh
go install github.com/K1NGS1LVER/ToGitOrNotToGit/cmd/tocommit@v0.4.0
rehash 2>/dev/null || hash -r
tocommit --version
```

For a locally built binary:

```zsh
make build
mkdir -p "$(go env GOPATH)/bin"
cp ./tocommit "$(go env GOPATH)/bin/tocommit"
rehash 2>/dev/null || hash -r
tocommit --version
```

The version check should print `tocommit version v0.4.0`. If it prints an older
version, run `command -v tocommit` and update that executable or move its
directory earlier in `PATH`.

### 2. Set API Key

Set your provider's key in your environment. Groq is the default:

```zsh
export GROQ_API_KEY="gsk_your_groq_api_key"
```

Supported providers:

| `provider` | Endpoint | API key env var |
| :--- | :--- | :--- |
| `groq` (default) | Groq | `GROQ_API_KEY` |
| `openai` | OpenAI | `OPENAI_API_KEY` |
| `ollama` | local Ollama | *(none required)* |
| `openai-compatible` | any OpenAI-compatible endpoint (`base_url`) | `OPENAI_API_KEY` |

`TOCOMMIT_API_KEY` overrides all of the above if set. **Keys are read from the environment only and are never written to disk.**

### 3. Install the Git Hook

Inside any Git repository:

```zsh
tocommit install
```

This installs `.git/hooks/prepare-commit-msg` safely without clobbering existing non-tocommit hooks.

To remove the hook at any time:
```zsh
tocommit uninstall
```

---

## 📖 Usage & Workflow

Once installed, simply stage your changes and commit as usual:

```zsh
git add .
git commit
```

That normal `git commit` path runs the installed `prepare-commit-msg` hook,
which asks `tocommit` to generate the commit message using the configured
custom persona or the severity-selected preset. This keeps the generated
messages in the repository history as evidence that the tool was used. Avoid
`git commit -m` and `git commit -F` when you want the hook to generate the
message; those modes intentionally bypass generation.

> [!TIP]
> Do **not** pass `-m` when committing if you want `tocommit` to generate a message. Passing `-m` or `-F` intentionally bypasses hook generation.

Check the installed version with:

```zsh
tocommit --version
```

Available commands:

| Command | Purpose |
| :--- | :--- |
| `tocommit install` | Install the managed `prepare-commit-msg` hook in the current repository |
| `tocommit uninstall` | Remove the managed hook without touching an unrelated hook |
| `tocommit config` | Interactively edit `~/.config/tocommit/config.yaml` |
| `tocommit models` | List model IDs available from the configured provider |
| `tocommit run <commit-msg-file> [source] [sha]` | Generate a message; normally called by the Git hook |
| `tocommit --version` | Print the installed binary version |

The hook is installed per repository. It contains `exec tocommit run ...`, so
every repository uses whichever `tocommit` binary your current `PATH` resolves.
After replacing a binary, verify the same version from inside the target
repository; reinstalling the hook is only necessary if the hook itself is
missing.

---

## 🎭 Persona Tiers & Severity Scoring

`tocommit` scores change severity based on line additions, deletions, and critical file paths:

| Tier | Trigger Criteria | Persona Assigned | Example Tone |
| :--- | :--- | :--- | :--- |
| **Minor** | `< 10` total lines modified | **Victorian Gothic** | *Dark, melancholic, reflective lamentation* |
| **Medium** | `10 - 100` lines modified | **Soap Opera** | *High drama, sudden betrayals, plot twists* |
| **Catastrophic** | `> 100` lines, or touches an override path (`go.mod`, `go.sum`, `Dockerfile`, `Makefile`, `.github/workflows/*`) | **Shakespearean Tragedy** | *Grand theatrical tragedy in iambic pentameter* |

You can also explicitly override the persona when running commands:
```zsh
tocommit run .git/COMMIT_EDITMSG --persona soap-opera
tocommit run .git/COMMIT_EDITMSG --persona "Ben Tennyson - a superhero from a cartoon show"
```

Custom personas are accepted as free-form text. You may provide just a character
name, such as `Tom and Jerry`, or include a description after a hyphen. The
configured persona is used as the default; an explicit `--persona` override
takes precedence, and an empty value keeps severity-based auto-detection. The
persona text is sent to the model as a style instruction; it does not need to
match one of the built-in names.

Persona resolution order:

1. Non-empty `--persona` on `tocommit run`.
2. Non-empty `persona` in `~/.config/tocommit/config.yaml`.
3. Severity-based selection: Victorian Gothic, Soap Opera, or Shakespearean Tragedy.

The override paths are configurable via `severity_overrides` (see [Configuration](#️-configuration)).

---

## 🖥️ Interactive Preview (TUI)

When `tui: true` (default) and Git is running in an interactive terminal,
`tocommit` renders an interactive terminal preview screen powered by
[Bubble Tea](https://github.com/charmbracelet/bubbletea). The selected persona
is resolved before this screen opens; the TUI reviews the generated message and
does not choose the persona:

```text
feat(auth): a tale of two tokens

Alas! The OAuth flow hath been rewritten in tears and blood...

[enter] accept  [e] edit  [r] regenerate  [q/ctrl+c] cancel
```

| Key | Action Description |
| :--- | :--- |
| `enter` | **Accept**: Writes the message to Git and proceeds to editor/commit |
| `e` | **Edit**: Opens an interactive text area (`ctrl+s` to save, `esc` to discard) |
| `r` | **Regenerate**: Calls Groq for a fresh alternative monologue |
| `q` / `ctrl+c` | **Cancel**: Aborts the commit cleanly without modifying the commit file |

---

## ⚙️ Configuration

Run the built-in interactive wizard:

```zsh
tocommit config
```

Or manually create/edit `~/.config/tocommit/config.yaml`:

```yaml
provider: groq
model: openai/gpt-oss-120b
fallback_models: [openai/gpt-oss-20b, qwen/qwen3.8-27b]  # empty = provider defaults
base_url: ""          # only for provider: openai-compatible
timeout_ms: 2500
tui: true
persona: ""             # built-in or custom text; empty = auto-severity
severity_overrides: [] # extra paths that force the Catastrophic tier
```

| Setting | Default | Description |
| :--- | :--- | :--- |
| `provider` | `groq` | LLM provider (`groq`, `openai`, `ollama`, `openai-compatible`) |
| `model` | `openai/gpt-oss-120b` | Model name on the provider |
| `fallback_models` | provider defaults | Models tried in order when `model` is unavailable; empty uses the built-in Groq fallbacks |
| `base_url` | `""` | Custom endpoint; required for `provider: openai-compatible` |
| `timeout_ms` | `2500` | Max wait time before silent fallback (1–60000) |
| `tui` | `true` | Enables interactive preview screen |
| `persona` | `""` | Optional built-in or custom persona text (empty = auto-severity) |
| `severity_overrides` | `[]` | Path patterns (globs, `dir/`, or basenames) that force Catastrophic; empty uses the built-in defaults |

Invalid config never blocks a commit: `tocommit` prints a warning, writes a conventional fallback message, and lets Git proceed. Run `tocommit config` to fix it. Persona values may contain spaces, punctuation, and descriptions; whitespace-only values are rejected.

---

## 🔁 Model Availability & Fallbacks

Providers retire models, and a retired model is the most common reason a commit message comes back plain.

When the configured `model` is rejected as unavailable, `tocommit` tries each entry in `fallback_models` in order before giving up.
Only a model-not-found failure starts the cascade; a bad key, timeout, or network error fails fast and never retries other models.

If every model fails, `tocommit` prints an actionable warning and still lets your commit through:

```text
tocommit: model "llama-3.3-70b-versatile" is not available on groq and no fallback model worked
tocommit: run 'tocommit models' to list valid models, then 'tocommit config' to change it,
tocommit: or set 'provider: ollama' in ~/.config/tocommit/config.yaml to use a local model.
```

List the model IDs your key can actually use:

```zsh
tocommit models
```

The configured model is marked with `*`.
Copy a valid id into `model` (or `fallback_models`) with `tocommit config`.

To run fully locally instead, point `tocommit` at Ollama:

```yaml
# ~/.config/tocommit/config.yaml
provider: ollama
model: llama3.1
```

---

## 📁 Project Architecture

```text
cmd/
├── tocommit/main.go   # CLI Entrypoint
├── root.go            # Cobra root command
├── run.go             # Main pipeline: Diff → Severity → LLM → TUI → File Write
├── install.go         # Hook installer & path resolver (core.hooksPath support)
├── models.go          # `tocommit models`: lists the provider's model IDs
└── config.go          # Interactive configuration wizard
internal/
├── config/            # YAML config parser, validation, model list & environment loader
├── diff/              # Git diff collector & numstat parser
├── llm/               # LLM client, model fallback cascade, models listing, message validator
├── logx/              # Opt-in debug logging
├── severity/          # Severity scoring engine & persona resolver
└── tui/               # Bubble Tea terminal preview UI
```

---

## 🔧 Troubleshooting

Enable debug logging to see what `tocommit` is doing:

```zsh
TOCOMMIT_DEBUG=1 git commit
```

This prints the provider, model, resolved persona, model cascade attempts, and the reason for any fallback (missing API key, timeout, HTTP status, invalid commit message, or config error).

> [!NOTE]
> When the LLM is unreachable or returns a message that isn't a valid conventional commit, `tocommit` writes a clean fallback (`chore: update N file(s) (+X/-Y)`) so your commit still goes through.
> A retired model or a rejected API key also prints an explicit, actionable warning so you know there is something to fix.
> In that case run `tocommit models` to see valid model ids, then `tocommit config` to update the model or provider.

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for details.
