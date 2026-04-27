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
- [Multi-account profiles](#multi-account-profiles)
- [Project file](#project-file)
- [Environment variables](#environment-variables)
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

Sign in via the browser. Credentials land in `~/.flagify/config.json`, scoped to a **profile** so you can keep work and personal accounts in the same machine without logout/login loops.

```bash
flagify auth login
flagify auth login --profile work       # add or refresh a named profile
```

Re-running `flagify auth login` does not error when another profile is already signed in — pass `--profile` to target a specific one. The default profile is called `default` and is created on first login.

If you log into a profile from a directory that already has a `.flagify/project.json`, and the committed `preferredProfile` does not match the profile you just authenticated, the CLI prompts to rewrite the pin to the newly logged-in profile. Skipped outside a TTY so CI never mutates committed files silently.

### `flagify whoami`

Show the currently resolved user and which profile the invocation is using.

```bash
flagify whoami
# ✓ Jane Doe (jane@company.com)  profile: work
flagify whoami --format json       # { "profile": "work", "user": {...} }
```

| Flag | Short | Description |
|------|-------|-------------|
| `--format` | | Output format (`table`, `json`) |

## Multi-account profiles

Every set of credentials lives under a named profile inside `~/.flagify/config.json`. Switch between profiles, list them, log out of one without touching the rest, or remove a profile entirely.

```bash
flagify auth login --profile work       # add or refresh a named profile
flagify auth list                        # tabular view with the active profile marked
flagify auth list --format json
flagify auth switch personal             # change the active profile
flagify auth logout                      # sign out of the active profile (keeps defaults)
flagify auth logout --profile work       # sign out of a specific profile
flagify auth logout --all                # sign out of every profile
flagify auth remove work                 # delete a profile and its repo bindings
flagify auth rename work acme            # rename and update any bindings that point to it
flagify auth whoami                      # alias of `flagify whoami`
```

Profiles are local to each machine. The names are yours — `work`, `personal`, `client-acme`, whatever fits. A repo committing a `.flagify/project.json` can hint at a preferred profile name, but it is never required.

### Binding a repo to a profile

When you clone a repo that already has `.flagify/project.json`, the CLI needs to know which local profile to use. If the committed `preferredProfile` hint matches one of your profiles, it is picked automatically. Otherwise, bind explicitly:

```bash
cd my-repo
flagify project bind --profile work
```

Bindings are stored under `bindings` in `~/.flagify/config.json` keyed by the absolute repo path. They are never committed.

### How the CLI picks a profile

Precedence when several signals could choose a profile, highest first:

1. `--profile <name>` on the command line.
2. `FLAGIFY_PROFILE` in the environment.
3. A local binding for the directory containing `.flagify/project.json`.
4. `preferredProfile` from `.flagify/project.json`, if that name exists locally.
5. The `current` profile from `~/.flagify/config.json`.
6. The sole profile, when exactly one exists.

If none of those produce a single unambiguous answer and more than one profile is signed in, the CLI errors with an actionable message pointing at `flagify auth list`.

## Project file

`flagify init` writes a committable `.flagify/project.json` that pins workspace, project, and environment for the repo without ever containing tokens.

```json
{
  "version": 1,
  "workspaceId": "ws_01J...",
  "workspace": "acme",
  "projectId": "pr_01J...",
  "project": "api",
  "environment": "development",
  "preferredProfile": "work"
}
```

`preferredProfile` is a hint. Teammates with different local profile names can still use the same project file — the CLI falls through to binding or `current` when the hint does not match a local profile.

### `flagify init`

Create or update the project file for the current repo.

```bash
flagify init --workspace-id ws_01J... --project-id pr_01J... --environment development
flagify init --print               # print the JSON it would write, don't touch disk
flagify init --force               # overwrite an existing project file in non-interactive shells
```

Idempotent: re-running with the same scope says `Already initialized` and leaves the file unchanged (same mtime). Running with a different scope prompts for confirmation in TTYs and errors in CI unless `--force` is passed.

| Flag | Description |
|------|-------------|
| `--workspace-id` / `--workspace` | Workspace ULID or slug (one is required) |
| `--project-id` / `--project` | Project ULID or slug (one is required) |
| `--environment` | Environment key (defaults to `development`) |
| `--preferred-profile` | Hint written to `.flagify/project.json` (defaults to the resolved profile) |
| `--print` | Print the JSON without writing to disk |
| `--force` | Overwrite an existing project file without prompting |

### `flagify project bind`

Bind the current repo to a local profile. Does not modify the committed project file.

```bash
flagify project bind --profile work
```

### `flagify project status`

Alias of `flagify status`. Handy when you are already thinking in terms of the project file.

### `flagify project set <field> <value>`

Update a single field of `.flagify/project.json` in place.

```bash
flagify project set environment staging
flagify project set preferred-profile work
```

Valid fields: `environment`, `project`, `project-id`, `workspace`, `workspace-id`, `preferred-profile`.

## Environment variables

Every `FLAGIFY_*` env var is honored at the same precedence layer — between CLI flags and the project file. Useful for CI and scripts.

| Variable | Overrides |
|----------|-----------|
| `FLAGIFY_PROFILE` | Which profile to use (fails loud if the name does not exist) |
| `FLAGIFY_WORKSPACE_ID` / `FLAGIFY_WORKSPACE` | Workspace (ID wins over slug at the same level) |
| `FLAGIFY_PROJECT_ID` / `FLAGIFY_PROJECT` | Project (ID wins over slug) |
| `FLAGIFY_ENVIRONMENT` | Environment key |
| `FLAGIFY_API_URL` | API base URL |
| `FLAGIFY_ACCESS_TOKEN` / `FLAGIFY_REFRESH_TOKEN` | Ephemeral tokens — the CLI will not persist refreshed tokens when these are set |

### `flagify status`

Show the resolved context for this invocation, with the source of each field.

```bash
flagify status
# Profile       work            (profile-default)
# User          Jane Doe <jane@acme.com>    (profile)
# Workspace     acme (ws_01J...)            (project-file)
# Project       api (pr_01J...)             (project-file)
# Environment   staging                     (flag)
# API URL       https://api.flagify.dev     (profile-default)
# Project file  /repo/.flagify/project.json
# Global store  ~/.flagify/config.json

flagify status --format json | jq .
```

The `source` column in JSON mode answers "why is the CLI using this value?" — values are `flag`, `env`, `project-file`, `binding`, `profile-default`, or `default`.

## Commands

> **Machine-readable output**: every read-only command (`*list`, `*get`, `whoami`, `config`, `keys generate`, `targeting list/set`) accepts `--format json` for clean, pipe-friendly output. Example: `flagify projects list --format json | jq '.[0].id'`.
>
> **Environment aliases**: `--environment` accepts the canonical `development|staging|production` **and** the short aliases `dev|stg|prod`.

### `flagify workspaces list`

List your workspaces.

```bash
flagify workspaces list
flagify workspaces list --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--format` | | Output format (`table`, `json`) |

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
flagify projects list --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--workspace` | `-w` | Workspace ID (falls back to config default) |
| `--format` | | Output format (`table`, `json`) |

---

### `flagify projects get`

Show project details with environments.

```bash
flagify projects get proj_xxx
flagify projects get proj_xxx --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--format` | | Output format (`table`, `json`) |

---

### `flagify projects pick`

Interactively select a default project. Saved to `~/.flagify/config.json`.

```bash
flagify projects pick
```

---

### `flagify projects delete`

Delete a project along with all its environments, flags, segments, and API keys. Requires admin role. Asks for confirmation unless `--yes` is passed.

```bash
flagify projects delete proj_xxx
```

```bash
flagify projects delete proj_xxx --yes
```

> This action is irreversible. If the deleted project was your saved default, the `project` and `projectId` config entries are cleared.

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
| `--environment` | `-e` | Environment key (must match the environment slug configured in the API for this project) |
| `--all` | `-a` | Toggle in all environments at once |

---

### `flagify flags health`

Scan the project for flag configuration issues. Detects:

- **`env_mismatch`** — flag is on in production but off in the preceding environment, or value drift between prod and pre-prod.
- **`rule_value_matches_default`** — a targeting rule's `valueOverride` equals the flag's `defaultValue`, making the rule a no-op (users outside the rollout fall through to `defaultValue` and receive the same value the rule would serve).

```bash
flagify flags health -p proj_xxx
flagify flags health -p proj_xxx --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--format` | | Output format (`table`, `json`) |

Exit code is 0 whether issues are found or not — use `--format json` and jq/grep for CI gating.

---

### `flagify keys generate`

Generate an API key pair (publishable + secret) for an environment. Keys are required for SDK integration.

```bash
flagify keys generate -p proj_xxx -e development
flagify keys generate -p proj_xxx -e development --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |
| `--format` | | Output format (`table`, `json`) — JSON returns `{environment, publishableKey, secretKey}` |

> **Important:** The secret key is only shown once. Save it immediately.

---

### `flagify keys list`

List all API keys for an environment.

```bash
flagify keys list -p proj_xxx -e development
flagify keys list -p proj_xxx -e development --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |
| `--format` | | Output format (`table`, `json`) |

---

### `flagify keys revoke`

Revoke a single API key by prefix (or `--id`), or revoke every active key in the environment with `--all`. Passing none of the three is treated as an error so the destructive form is always explicit.

```bash
# Revoke just one key — pass the prefix from 'keys list'
flagify keys revoke pk_dev_abc -p proj_xxx -e development
```

```bash
# Revoke a specific key by ULID (use when two keys share a prefix)
flagify keys revoke --id key_01J... -p proj_xxx -e development
```

```bash
# Revoke every active key in the environment (opt-in, destructive)
flagify keys revoke --all -p proj_xxx -e development
```

| Flag | Short | Description |
|------|-------|-------------|
| `[prefix]` | | Positional: prefix of the key to revoke (from `flagify keys list`) |
| `--id` | | Revoke the key with this ULID (use when prefixes collide) |
| `--all` | | Revoke every active API key in the environment |
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment key (required) |
| `--yes` | `-y` | Skip the confirmation prompt |

`[prefix]`, `--id`, and `--all` are mutually exclusive.

---

### `flagify segments list`

List all user segments defined in a project. Segments are reusable groups of users that targeting rules can reference.

```bash
flagify segments list -p proj_xxx
flagify segments list -p proj_xxx --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID (falls back to config default) |
| `--format` | | Output format (`table`, `json`) |

---

### `flagify segments create`

Create a new segment with optional JSON rules.

```bash
flagify segments create "Pro users" \
  -p proj_xxx \
  --match ALL \
  --rules '[{"attribute":"plan","operator":"equals","value":"pro"}]'
```

| Flag | Default | Description |
|------|---------|-------------|
| `--project` / `-p` | config default | Project ID |
| `--match` | `ALL` | Match mode: `ALL` or `ANY` |
| `--rules` | — | Rules as a JSON array of `{attribute, operator, value}` |
| `--yes` / `-y` | `false` | Skip the confirmation prompt |

---

### `flagify segments delete`

Delete a segment by ID. Asks for confirmation unless `--yes` is passed.

```bash
flagify segments delete seg_xxx
```

---

### `flagify webhooks list`

List webhooks in the current project. By default returns the aggregate view across every environment; pass `--environment` (or set it as a config default) to scope the result to one environment, e.g. only the production hooks.

```bash
flagify webhooks list                              # all environments
flagify webhooks list -e production                # only production hooks
flagify webhooks list -p proj_xxx --format json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID (falls back to config default) |
| `--environment` | `-e` | Filter to one environment slug or ID (optional) |
| `--format` |  | Output format (`table`, `json`) |

The aggregate table includes an extra `Environment` column so you can see at a glance which env each hook belongs to; the env-filtered view drops that column.

---

### `flagify webhooks create`

Create a webhook subscribed to one or more flag/targeting events. Webhooks are **environment-scoped**: each subscription targets a single environment, so a project can ship distinct hooks for `development`, `staging`, and `production` without cross-talk. The signing secret is printed **exactly once** — save it on the receiver (e.g. `FLAGIFY_WEBHOOK_SECRET=...`); Flagify can not retrieve it later. If you lose it, delete and recreate the webhook.

```bash
flagify webhooks create \
  --environment production \
  --name "Slack #releases" \
  --url https://hooks.slack.com/services/T00/B00/xxx \
  --events flag.created,flag.toggled,flag.deleted
```

Supported events: `flag.created`, `flag.updated`, `flag.toggled`, `flag.deleted`, `targeting_rule.created`, `targeting_rule.updated`, `targeting_rule.deleted`. Pass `--events ""` (or omit) to receive every supported event.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID or slug (falls back to config default) |
| `--environment` | `-e` | Environment slug or ID (required — falls back to config default) |
| `--name` |  | Display name (required) |
| `--url` |  | Receiver URL, must be `https://` for production (required) |
| `--events` |  | Comma-separated event list; empty = all events |

---

### `flagify webhooks get`

Show one webhook's URL, events, and status.

```bash
flagify webhooks get wh_01...
```

The secret is **never** shown by `get` or `list` — it only appears in the response of `create`.

---

### `flagify webhooks delete`

Delete a webhook by ID. Asks for confirmation unless `--yes` is passed.

```bash
flagify webhooks delete wh_01... --yes
```

---

### `flagify webhooks deliveries`

Recent delivery attempts for a webhook (newest first). Useful for debugging when a subscriber is failing — each row shows the HTTP code, attempt number, and status.

```bash
flagify webhooks deliveries wh_01...
flagify webhooks deliveries wh_01... --format json
```

The dispatcher retries failed deliveries up to 3 times (0s, 5s, 30s). Every attempt — succeeded or failed — is recorded as its own row.

---

### `flagify targeting list <flag-key>`

Show the targeting rules for a flag in an environment, in priority order.

```bash
flagify targeting list checkout-redesign -p proj_xxx -e production
flagify targeting list checkout-redesign -e production --format json
```

With `--format json`, the output is `{ "flag", "environment", "rules": [...] }` even when there are no rules (empty `rules` array) — safe for scripts.

Exit codes:
- `0`: rules fetched (may be empty).
- `1` + `flag "<key>" not found in project`: the flag does not exist.
- `1` + `environment "<env>" not configured for flag "<key>"`: the flag exists but the environment has no flag config.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID (falls back to config default) |
| `--environment` | `-e` | Environment key (defaults to `development`; accepts aliases `dev`, `stg`, `prod`) |
| `--format` | | Output format (`table`, `json`) |

---

### `flagify targeting set <flag-key>`

Replace **all** targeting rules for a flag in an environment. Pass the full desired rule set as a JSON array.

```bash
flagify targeting set checkout-redesign \
  -p proj_xxx -e production \
  --rules '[{"priority":1,"enabled":true,"segmentId":"seg_xxx","valueOverride":true,"rolloutPercentage":100}]'
```

Rule object fields: `priority` (number), `enabled` (boolean), `segmentId` (optional string), `conditions` (optional array of `{attribute, operator, value}`), `valueOverride` (any), `rolloutPercentage` (optional number 0–100).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project ID (falls back to config default) |
| `--environment` | `-e` | Environment key (defaults to `development`; accepts aliases `dev`, `stg`, `prod`) |
| `--rules` | | Rules as a JSON array (required) |
| `--format` | | Output format (`table`, `json`) — JSON returns the saved rules as `{flag, environment, rules}` |
| `--yes` | `-y` | Skip the confirmation prompt |

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

By default, generated configs tell the AI tool to run `flagify flags list` for live data and include a reference to the Flagify integrations catalogue (GitHub Actions and more) so the AI knows which first-party tools are available. Use `--include-flags` to also embed a snapshot of your current flags:

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

### `flagify types`

Generate a source file that exports every flag key in the current project as a typed constant. Catch typos in flag names at compile time instead of at runtime.

**TypeScript:**

```bash
flagify types --typescript
# ✓ Generated flagify.ts (12 flags)
```

```typescript
// Auto-generated by Flagify CLI — do not edit manually
// Project: Web App
// Generated: 2026-04-19T00:00:00Z

export const FLAG_KEYS = {
  "dark-mode": "dark-mode",
  "new-checkout-flow": "new-checkout-flow",
  "onboarding-v2": "onboarding-v2",
} as const;

export type FlagKey = keyof typeof FLAG_KEYS;
```

Use in application code:

```typescript
import { FLAG_KEYS, type FlagKey } from './flagify'

flagify.isEnabled(FLAG_KEYS['new-checkout-flow']) // autocompleted, typo-safe
```

**Go:**

```bash
flagify types --go --package myflags
# ✓ Generated flagify.go (12 flags)
```

```go
// Auto-generated by Flagify CLI — do not edit manually
// Project: Web App
// Generated: 2026-04-19T00:00:00Z

package myflags

const (
	FlagDarkMode        = "dark-mode"
	FlagNewCheckoutFlow = "new-checkout-flow"
	FlagOnboardingV2    = "onboarding-v2"
)
```

Pick a custom output path:

```bash
flagify types --typescript --output src/generated/flags.ts
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--typescript` | `-t` | -- | Emit TypeScript (mutually exclusive with `--go`) |
| `--go` | `-g` | -- | Emit Go (mutually exclusive with `--typescript`) |
| `--output` | `-o` | `flagify.<ext>` | Output file path |
| `--package` | -- | `flagify` | Go package name (only with `--go`) |
| `--project` | `-p` | from config | Project ID (falls back to `projectId` in `~/.flagify/config.json`) |

Exactly one of `--typescript` or `--go` is required. Existing files that start with the `// Auto-generated by Flagify CLI` header are overwritten silently; any other pre-existing file at `--output` triggers a confirmation prompt (skip with `--yes`) so a hand-authored file is never clobbered on first run.

---

### `flagify version`

Print the CLI version and build info.

```bash
flagify version
# flagify 1.0.0 (abc123)
```

## Configuration

The CLI stores configuration in `~/.flagify/config.json` under schema v2 (multi-account):

```json
{
  "version": 2,
  "current": "work",
  "accounts": {
    "work": {
      "accessToken": "eyJhbGci...",
      "refreshToken": "eyJhbGci...",
      "apiUrl": "https://api.flagify.dev",
      "consoleUrl": "https://console.flagify.dev",
      "user": { "id": "...", "email": "jane@acme.com", "name": "Jane Doe" },
      "defaults": {
        "workspace": "acme",
        "workspaceId": "01J5K8...",
        "project": "api",
        "projectId": "01J5KB...",
        "environment": "development"
      }
    },
    "personal": { "accessToken": "...", "refreshToken": "..." }
  },
  "bindings": {
    "/Users/jane/dev/acme-api": { "profile": "work" }
  }
}
```

| Top-level field | Description |
|-----------------|-------------|
| `version` | Schema version (currently `2`) |
| `current` | Name of the active profile |
| `accounts[<name>]` | One entry per signed-in identity; `defaults` mirrors the old flat scope fields |
| `bindings[<path>]` | Local repo → profile mapping, written by `flagify project bind` |

> **Migration from v1**: the first time a v2-aware CLI runs against an older flat `~/.flagify/config.json`, it migrates in place and writes a `.bak` alongside. Re-running does not re-migrate. Existing automation that wrote to the flat shape keeps working — the CLI projects the active profile as the flat view internally.

File permissions: `0700` on `~/.flagify/`, `0600` on `config.json`. Writes are atomic (temp file + rename).

### `flagify config`

For global and profile-level settings:

```bash
flagify config                       # pretty view of the active profile
flagify config --format json
flagify config set api-url https://api.flagify.dev
flagify config set console-url https://console.flagify.dev
flagify config get api-url
```

For per-repo scope (workspace / project / environment), prefer `flagify project set ...` so the change lands in the committable project file instead of the profile defaults.

Valid `config set` keys: `api-url`, `console-url`, `workspace`, `project`, `environment` (the last three still write to the active profile's defaults — they are kept for muscle memory from v1).

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

These flags are available on every command. Precedence for scope resolution: flag > env var > `.flagify/project.json` > binding > active profile's defaults > built-in defaults. IDs win over slugs at the same level.

| Flag | Short | Description |
|------|-------|-------------|
| `--profile` | | Profile to use (overrides `FLAGIFY_PROFILE`, bindings, and `current`) |
| `--workspace-id` | | Workspace ULID (wins over `--workspace` when both are set) |
| `--workspace` | `-w` | Workspace slug |
| `--project-id` | | Project ULID (wins over `--project` when both are set) |
| `--project` | `-p` | Project slug |
| `--environment` | `-e` | Environment key (`development`, `staging`, `production`, or any custom slug) |
| `--yes` | `-y` | Skip confirmation prompts |
| `--help` | `-h` | Help for any command |

> **Non-interactive mode**: The CLI automatically detects when it is not running in a terminal (piped output, CI, AI agents) and skips confirmation prompts. Pass `-y` to skip them explicitly.

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
