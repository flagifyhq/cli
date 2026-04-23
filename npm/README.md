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
# Sign in (creates a profile called "default" on first run)
flagify auth login

# Or add a named profile — handy when you juggle work and personal accounts
flagify auth login --profile work
flagify auth list
flagify auth switch personal

# Pin a repo to a workspace/project/environment in a committable file
flagify init --workspace-id ws_01J... --project-id pr_01J... --environment development

# See the resolved context for this invocation + the source of every field
flagify status
flagify status --format json

# Manage feature flags
flagify flags list -p <project-id>
flagify flags create my-flag -p <project-id> --type boolean
flagify flags toggle my-flag -p <project-id>
```

## Commands

> **Machine-readable output**: every read-only command below supports `--format json` for clean, pipe-friendly output. Combine with `jq` in scripts and CI. `--environment` accepts the canonical `development|staging|production` **and** the short aliases `dev|stg|prod`.

| Command | Description |
|---------|-------------|
| `flagify auth login` | Sign in with the browser flow (`--profile <name>` to add a second identity without signing out of the first) |
| `flagify auth logout` | Sign out of the active profile (`--profile <name>` or `--all`) |
| `flagify auth list` | List signed-in profiles (`--format json`) |
| `flagify auth switch <name>` | Set the active profile |
| `flagify auth remove <name>` | Delete a profile and any repo bindings that point to it |
| `flagify auth rename <old> <new>` | Rename a profile and update bindings |
| `flagify whoami` | Show the current user and pinned profile (`--format json`) |
| `flagify status` | Show the resolved context for this invocation with `sources` per field (`--format json`) |
| `flagify init` | Write `.flagify/project.json` for the current repo (`--print` to dry-run, `--force` to overwrite in non-TTY) |
| `flagify project bind --profile <name>` | Bind this repo to a local profile without touching the project file |
| `flagify project set <field> <value>` | Update a field in `.flagify/project.json` (environment, project, workspace, preferred-profile, …) |
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

Credentials live under named profiles in `~/.flagify/config.json` (schema v2, migrated automatically from any older flat file with a `.bak` preserved alongside).

Scope is resolved per-invocation. Precedence, highest first:

1. CLI flags (`--profile`, `--workspace-id`, `--workspace`, `--project-id`, `--project`, `--environment`).
2. Environment variables (`FLAGIFY_PROFILE`, `FLAGIFY_WORKSPACE_ID`, `FLAGIFY_WORKSPACE`, `FLAGIFY_PROJECT_ID`, `FLAGIFY_PROJECT`, `FLAGIFY_ENVIRONMENT`, `FLAGIFY_API_URL`).
3. `.flagify/project.json` walked up from the current directory.
4. Local binding recorded by `flagify project bind`.
5. The active profile's defaults.
6. Built-in defaults.

Run `flagify status` to see exactly which signal won for each field. See the full CLI reference at [flagify.dev/docs/cli](https://flagify.dev/docs/cli) for multi-account workflows and project file details.

> **Ephemeral tokens**: set `FLAGIFY_ACCESS_TOKEN` (and optionally `FLAGIFY_REFRESH_TOKEN`) to override the stored credentials for a single invocation. The CLI does not persist refreshed tokens when these are set.

## Requirements

- Flagify account ([flagify.dev](https://flagify.dev))

## License

MIT
