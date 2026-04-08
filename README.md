<p align="center">
  <a href="https://flagify.dev">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://flagify.dev/logo-white.svg" />
      <source media="(prefers-color-scheme: light)" srcset="https://flagify.dev/logo-color.svg" />
      <img alt="Flagify" src="https://flagify.dev/logo-color.svg" width="280" />
    </picture>
  </a>
</p>

<p align="center">
  <strong>Feature flags for modern teams</strong>
</p>

<p align="center">
  <a href="https://github.com/flagifyhq/cli/releases"><img src="https://img.shields.io/github/v/release/flagifyhq/cli?style=flat-square&color=0D80F9" alt="release" /></a>
  <a href="https://github.com/flagifyhq/cli/blob/main/LICENSE"><img src="https://img.shields.io/github/license/flagifyhq/cli?style=flat-square&color=0D80F9" alt="license" /></a>
  <a href="https://github.com/flagifyhq/cli"><img src="https://img.shields.io/github/stars/flagifyhq/cli?style=flat-square&color=0D80F9" alt="github stars" /></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/built_with-Go-00ADD8?style=flat-square" alt="built with Go" /></a>
</p>

<p align="center">
  <a href="https://flagify.dev/docs">Documentation</a> &middot;
  <a href="https://flagify.dev/docs/cli">CLI Reference</a> &middot;
  <a href="https://github.com/flagifyhq/cli/issues">Issues</a> &middot;
  <a href="https://flagify.dev">Website</a>
</p>

---

## Overview

The official Flagify CLI for managing feature flags from the terminal. Built in Go for fast, standalone execution with zero runtime dependencies.

- **Fast** -- Single binary, no runtime needed
- **Cross-platform** -- macOS, Linux, Windows
- **Scriptable** -- Pipe-friendly output for CI/CD workflows
- **Secure** -- Credentials stored locally in `~/.flagify/config.json`

## Table of contents

- [Installation](#installation)
- [Authentication](#authentication)
- [Commands](#commands)
- [Configuration](#configuration)
- [Development](#development)
- [License](#license)

## Installation

### Go

```bash
go install github.com/flagifyhq/cli/cmd/flagify@latest
```

### npm

```bash
npm install -g @flagify/cli
```

### Homebrew

```bash
brew tap flagifyhq/tap
brew install flagify
```

### Binary

Download the latest release from [GitHub Releases](https://github.com/flagifyhq/cli/releases).

### Alias (optional)

For a shorter command, add an alias to your shell profile:

```bash
echo 'alias flag="flagify"' >> ~/.zshrc && source ~/.zshrc
```

Then use `flag` instead of `flagify`:

```bash
flag flags list -p proj_xxx
```

## Authentication

```bash
flagify login
```

Prompts for email and password. Credentials are stored in `~/.flagify/config.json`.

## Commands

### `flagify workspaces list`

List your workspaces.

```bash
flagify workspaces list
```

---

### `flagify workspaces pick`

Interactively select a default workspace. Saved to `~/.flagify/config.json`.

```bash
flagify workspaces pick
```

---

### `flagify projects list`

List projects in a workspace. Falls back to saved workspace if `--workspace` is not passed.

```bash
flagify projects list -w ws_xxx
```

| Flag | Short | Description |
|------|-------|-------------|
| `--workspace` | `-w` | Workspace ID (falls back to config default) |

---

### `flagify projects get`

Show project details with environments.

```bash
flagify projects get proj_xxx
```

---

### `flagify projects pick`

Interactively select a default project. Saved to `~/.flagify/config.json`.

```bash
flagify projects pick
```

---

### `flagify environments pick`

Interactively select a default environment. Saved to `~/.flagify/config.json`.

```bash
flagify environments pick
```

---

### `flagify flags list`

List all flags in a project.

```bash
flagify flags list --project proj_xxx
```

Output as JSON for scripts and AI tools:

```bash
flagify flags list -p proj_xxx --format json
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | -- | Project key (required) |
| `--format` | -- | `table` | Output format (`table`, `json`) |

---

### `flagify flags get`

Get details for a specific flag, including status per environment.

```bash
flagify flags get my-feature -p proj_xxx
```

```bash
flagify flags get my-feature -p proj_xxx --format json
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | -- | Project key (required) |
| `--format` | -- | `table` | Output format (`table`, `json`) |

---

### `flagify flags create`

Create a new feature flag. Flag keys must be kebab-case (e.g., `my-feature`).

```bash
flagify flags create my-feature -p proj_xxx
```

```bash
flagify flags create checkout-variant -p proj_xxx --type string --description "A/B test for checkout"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | -- | Project key (required) |
| `--type` | `-t` | `boolean` | Flag type (boolean, string, number, json) |
| `--description` | -- | -- | Flag description |

---

### `flagify flags toggle`

Toggle a flag on or off. If no flag key is provided, an interactive picker lets you select from existing flags.

```bash
flagify flags toggle my-feature -p proj_xxx
flagify flags toggle dark-mode -p proj_xxx -e prod
flagify flags toggle dark-mode -p proj_xxx --all
```

Run without a key to pick interactively:

```bash
flagify flags toggle -p proj_xxx
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment (dev, staging, prod) |
| `--all` | `-a` | Toggle in all environments at once |

---

### `flagify keys generate`

Generate an API key pair (publishable + secret) for an environment. Keys are required for SDK integration.

```bash
flagify keys generate -p proj_xxx -e development
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |

> **Important:** The secret key is only shown once. Save it immediately.

---

### `flagify keys list`

List all API keys for an environment.

```bash
flagify keys list -p proj_xxx -e development
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |

---

### `flagify keys revoke`

Revoke all active API keys for an environment.

```bash
flagify keys revoke -p proj_xxx -e development
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |

---

### `flagify ai-setup`

Generate AI tool config files for your project. Creates config files so AI coding tools (Claude Code, Cursor, GitHub Copilot, Windsurf) understand your Flagify setup.

```bash
flagify ai-setup
```

Generate for a specific tool:

```bash
flagify ai-setup --tool cursor
```

By default, generated configs tell the AI tool to run `flagify flags list` for live data. Use `--include-flags` to also embed a snapshot of your current flags:

```bash
flagify ai-setup --include-flags
```

> **Note:** The snapshot is a point-in-time copy. The generated config always includes a `flagify flags list` instruction so the AI tool can fetch live data regardless.

| Flag | Default | Description |
|------|---------|-------------|
| `--tool` | -- | Generate for a specific tool (claude, cursor, copilot, windsurf) |
| `--include-flags` | `false` | Embed a snapshot of current flags (marked as potentially outdated) |

**Files generated per tool:**

| Tool | Files |
|------|-------|
| Claude Code | `CLAUDE.md` (appends), `.claude/commands/flagify-create.md`, `.claude/commands/flagify-toggle.md`, `.claude/commands/flagify-list.md` |
| Cursor | `.cursorrules` |
| GitHub Copilot | `.github/copilot-instructions.md` |
| Windsurf | `.windsurfrules` |

---

### `flagify version`

Print the CLI version and build info.

```bash
flagify version
# flagify 1.0.0 (abc123)
```

## Configuration

The CLI stores configuration in `~/.flagify/config.json`:

```json
{
  "accessToken": "eyJhbGci...",
  "refreshToken": "eyJhbGci...",
  "apiUrl": "https://api.flagify.dev",
  "consoleUrl": "https://console.flagify.dev",
  "workspace": "acme-corp",
  "workspaceId": "01J5K8RQXHNZ4VMKD3GY7PSABET",
  "project": "web-app",
  "projectId": "01J5KBC3XPQR7WFMN4HY6TDASEV",
  "environment": "development"
}
```

| Field | Description |
|-------|-------------|
| `accessToken` | JWT access token (set via `flagify login`) |
| `refreshToken` | JWT refresh token (set via `flagify login`) |
| `apiUrl` | API base URL (default: `https://api.flagify.dev`) |
| `consoleUrl` | Console URL (set via `flagify login`) |
| `workspace` | Default workspace slug (set via `flagify workspaces pick`) |
| `workspaceId` | Default workspace ID (set via `flagify workspaces pick`) |
| `project` | Default project slug (set via `flagify projects pick`) |
| `projectId` | Default project ID (set via `flagify projects pick`) |
| `environment` | Default environment key (set via `flagify environments pick`) |

View current config:

```bash
flagify config
```

Set a value:

```bash
flagify config set environment staging
```

Get a single value (useful for scripts):

```bash
flagify config get project
```

Valid keys: `api-url`, `workspace`, `project`, `environment`

## Shell completions

**Zsh (macOS default):**
```bash
flagify completion zsh > "${fpath[1]}/_flagify"
```

**Bash:**
```bash
flagify completion bash > /usr/local/etc/bash_completion.d/flagify
```

**Fish:**
```bash
flagify completion fish > ~/.config/fish/completions/flagify.fish
```

## Global flags

These flags are available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--workspace` | `-w` | Workspace ID |
| `--project` | `-p` | Project key |
| `--environment` | `-e` | Environment (dev, staging, prod) |
| `--yes` | `-y` | Skip confirmation prompts |
| `--help` | `-h` | Help for any command |

> **Non-interactive mode**: The CLI automatically detects when it's not running in a terminal (e.g., piped output, CI, AI agents) and skips confirmation prompts. You can also use `-y` to explicitly skip them.

## Development

```bash
# Clone
git clone https://github.com/flagifyhq/cli.git
cd cli

# Run without compiling
make run ARGS="flags list -p proj_xxx"

# Build binary
make build

# Install globally
make install

# Run tests
make test

# Lint
make lint

# Clean build artifacts
make clean
```

### Project structure

```
cmd/flagify/       Entry point (main.go)
cmd/               Command definitions (cobra)
internal/api/      HTTP client for Flagify API
internal/config/   Local config management (~/.flagify/)
internal/picker/   Interactive selection (huh)
internal/ui/       Terminal styling (lipgloss)
```

## License

MIT -- see [LICENSE](./LICENSE) for details.

---

<p align="center">
  <sub>Built with care by the <a href="https://flagify.dev">Flagify</a> team</sub>
</p>
