# 🎭 ToGitOrNotToGit (`tocommit`)

[![Go Version](https://img.shields.io/github/go-mod/go-version/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=00ADD8)](https://go.dev)
[![License](https://img.shields.io/github/license/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=blue)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=green)](https://github.com/K1NGS1LVER/ToGitOrNotToGit/releases)

> _"Out, damn bug! Out, I say!"_ — **Macbeth (Act V, Scene I), adapted for Git**

**ToGitOrNotToGit** (binary name: `tocommit`) is a Git hook CLI that inspects your staged changes and uses AI to generate theatrical, dramatic commit messages based on the severity of your code changes. 

Transform boring commit messages like `fix typo` or `updated config` into Victorian Gothic laments, Soap Opera betrayals, and Shakespearean Tragedies — right inside your terminal workflow.

---

## 🚀 Key Features

- 🎭 **Theatrical Persona Escalation**: Automatically assigns a persona (*Victorian Gothic*, *Soap Opera*, or *Shakespearean Tragedy*) based on diff line counts and sensitive file changes (`go.mod`, `go.sum`, `Dockerfile`, `Makefile`, `.github/workflows/*`).
- 🖥️ **Interactive Terminal Preview (TUI)**: Review, edit (`e`), regenerate (`r`), or accept (`enter`) generated commit messages interactively before committing.
- ⚡ **Zero-Block Fallback System**: If network requests fail, timeout (default `2500ms`), or an API key is missing, `tocommit` instantly generates a clean conventional fallback commit without blocking your git flow.
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

> [!TIP]
> Do **not** pass `-m` when committing if you want `tocommit` to generate a message. Passing `-m` or `-F` intentionally bypasses hook generation.

Check the installed version with:

```zsh
tocommit --version
```

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
```

The override paths are configurable via `severity_overrides` (see [Configuration](#️-configuration)).

---

## 🖥️ Interactive Preview (TUI)

When `tui: true` (default), `tocommit` renders an interactive terminal preview screen powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea):

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
model: llama-3.3-70b-versatile
base_url: ""          # only for provider: openai-compatible
timeout_ms: 2500
tui: true
persona: ""
severity_overrides: [] # extra paths that force the Catastrophic tier
```

| Setting | Default | Description |
| :--- | :--- | :--- |
| `provider` | `groq` | LLM provider (`groq`, `openai`, `ollama`, `openai-compatible`) |
| `model` | `llama-3.3-70b-versatile` | Model name on the provider |
| `base_url` | `""` | Custom endpoint; required for `provider: openai-compatible` |
| `timeout_ms` | `2500` | Max wait time before silent fallback (1–60000) |
| `tui` | `true` | Enables interactive preview screen |
| `persona` | `""` | Optional forced default persona (empty = auto-severity) |
| `severity_overrides` | `[]` | Path patterns (globs, `dir/`, or basenames) that force Catastrophic; empty uses the built-in defaults |

Invalid config never blocks a commit: `tocommit` prints a warning, writes a conventional fallback message, and lets git proceed. Run `tocommit config` to fix it.

---

## 📁 Project Architecture

```text
cmd/
├── tocommit/main.go   # CLI Entrypoint
├── root.go            # Cobra root command
├── run.go             # Main pipeline: Diff → Severity → LLM → TUI → File Write
├── install.go         # Hook installer & path resolver (core.hooksPath support)
└── config.go          # Interactive configuration wizard
internal/
├── config/            # YAML config parser, validation & environment loader
├── diff/              # Git diff collector & numstat parser
├── llm/               # LLM client interface, OpenAI-compatible client & message validator
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

This prints the provider, model, resolved persona, and the reason for any fallback (missing API key, timeout, HTTP status, invalid commit message, or config error).

> [!NOTE]
> When the LLM is unreachable or returns a message that isn't a valid conventional commit, `tocommit` writes a clean fallback (`chore: update N file(s) (+X/-Y)`) so your commit still goes through.

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for details.
