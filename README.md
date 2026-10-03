# 🎭 ToGitOrNotToGit (`tocommit`)

[![Go Version](https://img.shields.io/github/go-mod/go-version/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=00ADD8)](https://go.dev)
[![License](https://img.shields.io/github/license/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=blue)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/K1NGS1LVER/ToGitOrNotToGit?style=flat-square&color=green)](https://github.com/K1NGS1LVER/ToGitOrNotToGit/releases)

> _"Out, damn bug! Out, I say!"_ — **Macbeth (Act V, Scene I), adapted for Git**

**ToGitOrNotToGit** (binary name: `tocommit`) is a Git hook CLI that inspects your staged changes and uses AI to generate theatrical, dramatic commit messages based on the severity of your code changes. 

Transform boring commit messages like `fix typo` or `updated config` into Victorian Gothic laments, Soap Opera betrayals, and Shakespearean Tragedies — right inside your terminal workflow.

---

## 🚀 Key Features

- 🎭 **Theatrical Persona Escalation**: Automatically assigns a persona (*Victorian Gothic*, *Soap Opera*, or *Shakespearean Tragedy*) based on diff line counts and sensitive file changes (`go.mod`, `Dockerfile`, `.github/workflows/*`).
- 🖥️ **Interactive Terminal Preview (TUI)**: Review, edit (`e`), regenerate (`r`), or accept (`enter`) generated commit messages interactively before committing.
- ⚡ **Zero-Block Fallback System**: If network requests fail, timeout (default `2500ms`), or an API key is missing, `tocommit` instantly generates a clean conventional fallback commit without blocking your git flow.
- 🛡️ **Zero-Leak Security**: `GROQ_API_KEY` is read strictly from environment variables and is **never written to config files or disk**.
- 🛠️ **Smart Bypass**: Automatically skips execution for automated commits (`-m`, `-F`, `--amend`, `merge`, `squash`) to preserve git integrity.

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

Set your [Groq API Key](https://console.groq.com) in your environment:

```zsh
export GROQ_API_KEY="gsk_your_groq_api_key"
```

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

---

## 🎭 Persona Tiers & Severity Scoring

`tocommit` scores change severity based on line additions, deletions, and critical file paths:

| Tier | Trigger Criteria | Persona Assigned | Example Tone |
| :--- | :--- | :--- | :--- |
| **Minor** | `< 10` total lines modified | **Victorian Gothic** | *Dark, melancholic, reflective lamentation* |
| **Medium** | `10 - 100` lines modified | **Soap Opera** | *High drama, sudden betrayals, plot twists* |
| **Catastrophic** | `> 100` lines, or touches `go.mod`, `Dockerfile`, `.github/workflows/*` | **Shakespearean Tragedy** | *Grand theatrical tragedy in iambic pentameter* |

You can also explicitly override the persona when running commands:
```zsh
tocommit run .git/COMMIT_EDITMSG --persona soap-opera
```

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
timeout_ms: 2500
tui: true
persona: ""
```

| Setting | Default | Description |
| :--- | :--- | :--- |
| `provider` | `groq` | LLM provider (`groq` supported in v1) |
| `model` | `llama-3.3-70b-versatile` | Model name on Groq inference platform |
| `timeout_ms` | `2500` | Max wait time before silent fallback |
| `tui` | `true` | Enables interactive preview screen |
| `persona` | `""` | Optional forced default persona (empty = auto-severity) |

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
├── config/            # YAML config parser & environment loader
├── diff/              # Git diff collector & numstat parser
├── llm/               # LLM client interface, Groq adapter & message validator
├── severity/          # Severity scoring engine & persona resolver
└── tui/               # Bubble Tea terminal preview UI
```

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for details.
