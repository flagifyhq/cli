# Changelog

All notable changes to the Flagify CLI will be documented in this file.

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
