# @flagify/cli

The official Flagify CLI for managing feature flags from the terminal.

## Install

```bash
npm install -g @flagify/cli
```

Or with Homebrew:

```bash
brew tap flagifyhq/tap
brew install flagify
```

Or download binaries directly from [GitHub Releases](https://github.com/flagifyhq/cli/releases).

**Tip:** Add a short alias to your shell profile for faster usage:

```bash
echo 'alias flag="flagify"' >> ~/.zshrc && source ~/.zshrc
```

## Quick Start

```bash
# Login to your account
flagify login

# Discover your resources
flagify workspaces list
flagify projects list -w <workspace-id>
flagify projects get <project-id>

# Manage feature flags
flagify flags list -p <project-id>
flagify flags create my-flag -p <project-id> --type boolean
flagify flags toggle my-flag -p <project-id>
```

## Commands

> **Machine-readable output**: every read-only command below supports `--format json` for clean, pipe-friendly output. Combine with `jq` in scripts and CI. `--environment` accepts the canonical `development|staging|production` **and** the short aliases `dev|stg|prod`.

| Command | Description |
|---------|-------------|
| `flagify login` | Authenticate with email and password |
| `flagify logout` | Clear stored credentials |
| `flagify whoami` | Show the currently authenticated user |
| `flagify workspaces list` | List your workspaces (`--format json`) |
| `flagify workspaces pick` | Interactively select a default workspace |
| `flagify projects list` | List projects in a workspace (`--format json`) |
| `flagify projects get` | Show project details with environments (`--format json`) |
| `flagify projects pick` | Interactively select a default project |
| `flagify projects delete <id>` | Delete a project and all its environments, flags, and segments (admin only) |
| `flagify environments pick` | Interactively select a default environment |
| `flagify flags list` | List all flags in a project (`--format json`) |
| `flagify flags get` | Get details for a specific flag with per-environment status (`--format json`) |
| `flagify flags create` | Create a new feature flag (kebab-case keys enforced) |
| `flagify flags toggle` | Enable or disable a flag (interactive picker if no key given, supports `--all`) |
| `flagify flags health` | Detect configuration issues (env mismatches, no-op targeting rules) (`--format json`) |
| `flagify keys generate` | Generate API key pair for environment (`--format json`) |
| `flagify keys list` | List API keys for environment (`--format json`) |
| `flagify keys revoke <prefix>` | Revoke a single API key by prefix (e.g. `flagify keys revoke pk_dev_abc`). Use `--id <ulid>` for explicit targeting, or `--all` to revoke every active key in the environment |
| `flagify segments list` | List user segments in a project (`--format json`) |
| `flagify segments create <name>` | Create a segment with optional JSON rules (`--match`, `--rules`) |
| `flagify segments delete <id>` | Delete a segment by ID |
| `flagify targeting list <flag-key>` | Show targeting rules for a flag in an environment (`--format json`; returns `{flag, environment, rules}`) |
| `flagify targeting set <flag-key>` | Replace all targeting rules for a flag (`--rules '<json>'`, `--format json`) |
| `flagify whoami` | Show current authenticated user (`--format json`) |
| `flagify ai-setup` | Generate AI tool configs (Claude, Cursor, Copilot, Windsurf). Includes the integrations catalogue; use `--include-flags` for a snapshot |
| `flagify types` | Generate typed flag key constants (`--typescript` or `--go`) for compile-time safety in application code |
| `flagify config` | Show current configuration (`--format json`) |
| `flagify config set <key> <value>` | Set a config value (api-url, console-url, workspace, project, environment) |
| `flagify config get <key>` | Get a single config value |
| `flagify completion` | Generate shell completion scripts |
| `flagify version` | Print CLI version |

## Shell Completions

**Zsh:**
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

## Configuration

The CLI stores credentials in `~/.flagify/config.json`. Use `--project` and `--environment` flags to target specific resources.

## Requirements

- Flagify account ([flagify.dev](https://flagify.dev))

## License

MIT
