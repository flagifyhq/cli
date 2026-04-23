# Changelog

All notable changes to the Flagify CLI will be documented in this file.

## [v2.0.0](https://github.com/flagifyhq/cli/releases/tag/v2.0.0) — 2026-04-23

### Breaking changes

- **Top-level `flagify login` and `flagify logout` have been removed.** Use `flagify auth login` and `flagify auth logout` instead. The auth namespace is now the single home for profile-aware sign-in, sign-out, profile switching, removal, and rename flows.
- **Classic email/password login has been removed from the CLI.** `flagify auth login` now uses the browser authorization flow. This keeps credential entry on the Flagify console domain and avoids handling raw passwords in the terminal.

### Features

- **Multi-account auth profiles are now the default auth model.** `flagify auth login --profile <name>` adds or refreshes a profile without logging out of another account, and `flagify auth list`, `switch`, `remove`, `rename`, and `whoami` manage local identities explicitly.

## [v1.7.0](https://github.com/flagifyhq/cli/releases/tag/v1.7.0) — 2026-04-20

### Features

- **`flagify types`** — new command that generates a source file exporting every flag key in the current project as a typed constant. Supports `--typescript` (writes `flagify.ts` with `FLAG_KEYS` object + `FlagKey` union type) and `--go` (writes `flagify.go` with `Flag<PascalCase>` consts under a configurable `--package`). Keys are sorted alphabetically for stable diffs, the output file header marks the file as auto-generated, and invalid kebab keys (legacy flags created outside the CLI) are skipped with a warning when emitting Go. Project names fetched from the dashboard are sanitized before landing in the file header so free-text fields cannot break out of the leading comment and produce broken source. When `--output` points at an existing file that does *not* carry the Flagify auto-generated header, the command prompts for confirmation (bypassed with `--yes`) so a hand-authored `flags.ts`/`flags.go` is never clobbered on first run. Run it whenever flags change and commit the regenerated file alongside the change so application code catches typos in flag names at compile time instead of at runtime (#40).

### Documentation

- **`flagify ai-setup` templates** gain a section documenting `flagify types` and the `FLAG_KEYS` / `FlagKey` import pattern, so AI tooling (Claude Code, Cursor, Copilot, Windsurf) steers new code toward typed flag keys instead of raw string literals (#40).

## [v1.6.0](https://github.com/flagifyhq/cli/releases/tag/v1.6.0) — 2026-04-19

### Breaking changes

- **`flagify keys revoke` no longer revokes every key by default.** The command now requires one of three selectors: a positional `[prefix]` (revoke a single key by prefix, resolved via `flagify keys list`), `--id <ulid>` (revoke a specific key by ULID when prefixes collide), or `--all` (the previous env-wide behavior, now explicit and opt-in). Passing none of them returns an error instead of silently revoking everything. If you have CI that relied on the no-argument form, add `--all` to restore the old behavior. The single-key path hits the project-scoped `POST /v1/projects/{pid}/environments/{env}/keys/{kid}/revoke` endpoint and emits the existing `apikey.revoked` audit event (distinct from the `apikey.all_revoked` event that `--all` produces), so audit trails can tell the two flows apart.

### Improvements

- The confirmation prompt for `flagify keys revoke` now colours `secret` key types in red so the higher-blast-radius action stands out from a `publishable` revoke.

## [v1.5.0](https://github.com/flagifyhq/cli/releases/tag/v1.5.0) — 2026-04-19

### Features

- **`flagify flags health`** — scan the current project for flag configuration issues. Surfaces two classes of problem today: `env_mismatch` (flag on in prod but off in the preceding environment, or value drift) and the new `rule_value_matches_default` (a targeting rule whose `valueOverride` equals the flag's `defaultValue`, making the rule a silent no-op because users outside the rollout fall through to `defaultValue` and receive the same value the rule would serve). Table output colours severity and shows an environment pill for rule-scoped issues; `--format json` returns the full payload including a `fix` hint. Exit code is always 0 — use `jq -e` for CI gating. Pairs with the API health check extended in the same session (#38).

## [v1.4.0](https://github.com/flagifyhq/cli/releases/tag/v1.4.0) — 2026-04-18

### Bug fixes

- `flagify targeting list` and `flagify segments list` now send the project ULID to the API instead of the project slug, fixing the long-standing `not_found: resource not found` error that masked every targeting query (#35).

### Features

- **Uniform `--format json`** across every read-only command: `flags list`, `flags get`, `keys list`, `keys generate`, `projects list`, `projects get`, `workspaces list`, `segments list`, `targeting list`, `targeting set`, `whoami`, `config`. Pipe-friendly output for scripts, CI, and AI tooling. `keys generate --format json` returns `{environment, publishableKey, secretKey}` for one-shot scripting; `config --format json` deliberately omits tokens and secrets so the output is safe to log (#35).
- **Slug-friendly `--environment`**: the CLI now sends the environment slug directly to the new project-scoped API routes (`/v1/projects/{pid}/...`). Custom environment slugs (`prod-eu`, `qa-2`, etc.) work without configuration. The `findFlagEnvID` / `resolveEnvironmentID` round-trips that the CLI used to perform locally have been removed (#36).
- `targeting list` and `targeting set` use POSIX-style `<flag-key>` argument syntax and emit distinct, scriptable error messages for each failure mode (missing key, flag not found, environment not configured for flag) with actionable next-step hints (#35).

### Improvements

- `flagify` root command sets `SilenceUsage: true` so failures keep `stderr` clean for `jq` and shell scripts (no full `--help` dump after every API error) (#35).
- Sentinel errors `ErrFlagNotFound` and `ErrEnvNotFound` exposed for callers that import the CLI as a library and want to branch on the failure mode via `errors.Is` (#35).

## [v1.3.0](https://github.com/flagifyhq/cli/releases/tag/v1.3.0) — 2026-04-13

### Documentation

- Documented `segments`, `targeting`, and `whoami` commands in the CLI README (#28).
- `CLAUDE.md` template now points at `api.flagify.dev` and includes an Integrations section (#29, #30, #31).

## [v1.2.0](https://github.com/flagifyhq/cli/releases/tag/v1.2.0) — 2026-04-13

### Features

- `flagify login` now redirects the browser to the Flagify console success page after authorization, instead of showing an inline HTML page in localhost. The success and error states are rendered with animated icons on the console domain (#26)
- `console-url` added as a valid key for `flagify config set` and `flagify config get`. Allows overriding the console URL for local development (#26)

### Improvements

- Removed dead `callbackHTML` function that served inline HTML in the browser callback (#26)

## [v1.1.0](https://github.com/flagifyhq/cli/releases/tag/v1.1.0) — 2026-04-10

### Features

- `flagify projects delete <project-id>` — permanently delete a project along with all its environments, flags, segments, and API keys. Requires the `admin` role or higher on the workspace. Prompts for confirmation unless `--yes`/`-y` is passed. If the deleted project was your saved default, the `project` and `projectId` config entries are cleared automatically (#16)

## [v1.0.7](https://github.com/flagifyhq/cli/releases/tag/v1.0.7) — 2026-04-10

> Version v1.0.6 was skipped to keep the CLI aligned with the `@flagify/*` SDKs, which were already at v1.0.6. The CLI jumps directly from v1.0.5 → v1.0.7.

### Docs

- `flagify ai-setup` templates now teach the correct user-context & targeting pattern across all four AI tools (Claude, Cursor, Copilot, Windsurf). Adds a "User context & targeting (CRITICAL)" section to the Claude template, explicit DO-NOT guidance against custom React hooks that call `client.evaluate()` per flag, and clarification that targeting rules live server-side and the SDK only forwards user attributes (#14)
- The three Claude slash commands (`flagify-create`, `flagify-toggle`, `flagify-list`) now cross-reference the targeting section when relevant (#14)
- Add CHANGELOG.md with full release history (#12, #13)

### Bug Fixes

- `flagify ai-setup` templates previously showed the user object as `{ userId: 'u_123' }`, which is the wire format. The correct developer-facing API is `{ id: 'u_123' }` — the SDK serializes `id` → `userId` internally. Fixed in all templates (#14)

## [v1.0.5](https://github.com/flagifyhq/cli/releases/tag/v1.0.5) — 2026-04-08

### Features

- JSON output format with `--format json` for all commands (#11)
- `flags get <key>` command to inspect a single flag (#11)
- Kebab-case flag aliases for all options (#11)
- TTY detection for non-interactive environments (#11)

### Bug Fixes

- Update CLI auth URL to `/auth/cli-auth` (#10)

### Improvements

- Interactive flag picker for `flags toggle` when no key is provided (#9)

## [v0.0.3](https://github.com/flagifyhq/cli/releases/tag/v0.0.3) — 2026-03-31

### Features

- Browser-based login flow and `whoami` command
- Interactive pickers, config defaults, and confirmation prompts
- `config set` / `config get` subcommands with slug-based config
- API key management commands (`keys generate`, `keys list`, `keys revoke`)
- Shell completion command
- Workspaces and projects list/get commands
- Segments and targeting CLI commands
- `ai-setup` command for AI tool config generation (#2)
- `toggle --all` flag for bulk toggling
- Multivariate flag variants with `FlagVariant` type
- Custom styled help output with Lipgloss
- Auto-refresh tokens on 401
- Improved AI editor templates with SDK docs and CLI reference

## [v0.0.2](https://github.com/flagifyhq/cli/releases/tag/v0.0.2) — 2026-03-29

### Docs

- Add npm README and fix repo links

## [v0.0.1](https://github.com/flagifyhq/cli/releases/tag/v0.0.1) — 2026-03-29

Initial release of the Flagify CLI.

### Features

- `login` / `logout` authentication flow
- `flags list`, `flags create`, `flags toggle` commands
- Config management with `config` command
- Release pipeline with GoReleaser, Homebrew tap, and npm publish
