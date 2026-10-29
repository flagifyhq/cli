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
| `flagify auth login` | Sign in with the browser flow (`--profile <name>` to add a second identity without signing out of the first). If the current repo has a `.flagify/project.json` whose `preferredProfile` points elsewhere, the CLI prompts (TTY only) to rewrite the pin to the profile you just authenticated. If the browser comes back without tokens (expired session / interrupted flow), the CLI auto-reopens the flow on a TTY (up to 3 attempts); non-TTY runs get a single actionable error. Over SSH or on a headless Linux box (no `DISPLAY`/`WAYLAND_DISPLAY`) it switches to the device flow: it prints a one-time code and a console URL to approve from any other device. Force a flow with `--device` (alias `--no-browser`) or `--browser`. |
| `flagify auth logout` | Sign out of the active profile (`--profile <name>` or `--all`) |
| `flagify auth list` | List profiles with local session state: active, refreshable, expired, invalid, or logged out (`--format json` preserves `loggedIn` and adds `status`) |
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
| `flagify webhooks list` | List webhooks in a project; defaults to the aggregate view across environments (`--environment` to scope, `--format json`) |
| `flagify webhooks create` | Create a webhook subscribed to one or more flag/targeting events. Environment-scoped — the signing secret is printed exactly once |
| `flagify webhooks get <webhook-id>` | Show a webhook's URL, environment, events, and status (`--format json`) |
| `flagify webhooks delete <webhook-id>` | Delete a webhook subscription |
| `flagify webhooks deliveries <webhook-id>` | Show recent delivery attempts with status code and response excerpt (`--format json`, `--limit N`) |
| `flagify whoami` | Show current authenticated user (`--format json`) |
| `flagify ai-setup` | Generate AI tool configs (Claude Code skill, Cursor, Copilot, Windsurf). Includes the integrations catalogue; use `--include-flags` for a snapshot |
| `flagify types` | Generate typed flag key constants (`--typescript` or `--go`) for compile-time safety in application code |
| `flagify config` | Show current configuration (`--format json`) |
| `flagify config set <key> <value>` | Set a config value (api-url, console-url, workspace, project, environment) |
| `flagify config get <key>` | Get a single config value |
| `flagify completion` | Generate shell completion scripts |
| `flagify version` | Print CLI version |
| `flagify update` | Update the CLI to the latest release (`--check` to report only, `--force` to reinstall). Direct installs self-replace after SHA256 verification; Homebrew/npm/`go install` installs print the upgrade command instead. Requires `--yes` to install in non-TTY. |

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

Credentials live under named profiles in `~/.flagify/config.json` (schema v2, migrated automatically from any older flat file with a `.bak` preserved alongside). Concurrent `flagify` processes are safe: every write takes a short exclusive lock on the sibling `~/.flagify/config.lock`, and token refresh is serialized through it so one invocation cannot revert another's rotated session.

On its next login, each profile receives a stable opaque device ID so profiles for the same Flagify user keep independent refresh sessions. Existing credentials are not rewritten or revoked automatically. If an older profile can no longer refresh, reauthenticate only that profile with `flagify auth login --profile <name>`. Refresh failures identify the affected profile and return the server's refresh-session cause.

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


### Persistent account authentication

Sign in once per Flagify account and API server, using a profile such as `personal`.
Reuse that profile from every project folder for the account. Each folder selects
its own workspace, project and environment through `.flagify/project.json`; use
`flagify project bind --profile personal` for a local profile binding. Different Flagify
accounts must keep separate profiles. `flagify status` shows the resolved context;
`flagify whoami` checks authentication with the server.

After upgrading the API and CLI, approve `flagify auth login --profile personal`
once. Both browser and device login issue a persistent, revocable CLI credential.
It does not expire when you leave the CLI unused for days. Existing expired or
revoked refresh tokens cannot be recovered automatically; a new approval is needed
once. Existing valid legacy refresh tokens keep their five-day policy until the
next login; the CLI never silently changes their security policy.

The persistent credential only renews 15-minute CLI access tokens. It permits
project and flag operations, including keys and webhooks, within your current
workspace permissions, plus account identification and workspace/project discovery.
It cannot change account credentials, billing or team membership, accept invitations,
or authorize another device. Every request checks server-side revocation; logout,
reauthorization of that device, account disabling and account token-version changes
invalidate access. Web sessions retain their existing lifetimes.

Credentials stay in `~/.flagify/config.json` (mode `0600`), outside repositories.
Never copy credentials into `.flagify/project.json`. The persistent credential is
stable across refreshes, so an interrupted refresh cannot consume its only copy.
The config lock still protects shared writes and legacy rotating refresh tokens.
`flagify auth list` describes local metadata only; `refreshable` does not prove that
the server has not revoked a grant.

`flagify auth logout --profile personal` revokes that device before removing its
local credentials; `auth remove` also revokes persistent credentials. A network or
server failure preserves credentials and reports an error so revocation can be
retried. `auth logout --all` processes each profile; retry after a partial failure.
Revoking a shared profile signs out every folder using it on this machine.
