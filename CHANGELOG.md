# Changelog

All notable changes to the Flagify CLI will be documented in this file.

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
