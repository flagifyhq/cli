# Flagify CLI — CLAUDE.md

Instructions for AI assistants and contributors working on the Flagify CLI.

## Project overview

Go CLI for managing Flagify feature flags. Built with Cobra, styled with Lipgloss, interactive pickers with Huh.

## Key file locations

| What | Where |
|------|-------|
| Entry point | `cmd/flagify/main.go` |
| Commands | `cmd/*.go` |
| API client + types | `internal/api/client.go` |
| API client tests | `internal/api/client_test.go` |
| Config (load/save) | `internal/config/config.go` |
| Config tests | `internal/config/config_test.go` |
| UI helpers (lipgloss) | `internal/ui/ui.go` |
| Confirm prompt | `internal/ui/confirm.go` |
| Interactive pickers | `internal/picker/picker.go` |
| npm wrapper package | `npm/` |
| Release config | `.goreleaser.yaml` |

## When adding or changing a CLI command

1. Add types and methods to `internal/api/client.go` if new API calls are needed.
2. Create or update the command in `cmd/`. Follow existing patterns:
   - Use `resolveFlag(cmd, "project", cfg.Project)` to fall back to config defaults.
   - Use `ui.Table()`, `ui.Success()`, `ui.KeyValue()` for styled output.
   - Add `ui.Confirm()` before destructive actions (create, toggle, generate, revoke). Read-only commands (list, get) skip confirmation.
   - Register in `init()` under the correct parent command.
3. Add tests to `internal/api/client_test.go` using `httptest.NewServer` pattern.
4. Run `go test ./...` and `make build`.
5. Update docs in **3 places** (all must stay in sync):
   - `README.md` — command section with usage, flags table
   - `npm/README.md` — commands table
   - Website docs at `../apps/apps/website/src/content/docs/cli/commands.mdx` — full reference with example output, options table, Callouts

## When changing config fields

1. Update struct in `internal/config/config.go`.
2. Update `cmd/config.go` display.
3. Update config example + fields table in: `README.md`, `npm/README.md`, website `cli/configuration.mdx`.
4. Update `internal/config/config_test.go`.

## Documentation format rules

- Separate shell commands into individual code blocks (don't combine multiple in one block — breaks copy-paste).
- Use `Callout` components for warnings/tips in website `.mdx` files.
- Keep examples realistic — use seed data IDs when possible.

## Build & test

```bash
make build      # compile to ./bin/flagify
make test       # go test ./...
make install    # install globally
make lint       # go vet
```

## Release

Tag with `v*` triggers GitHub Actions: GoReleaser (binary + Homebrew) + npm publish. Don't add CI/CD unless explicitly asked.

## Style conventions

- Colors: Cyan `#00D4FF`, Green `#00CC88`, Red `#FF6B6B`, Yellow `#FFCC00`, Dim `#666666`
- Icons: `✓` success, `●` info, `⚠` warning, `→` prompt arrow
- Tables: `ui.Table()` with lipgloss (not tabwriter)
- Config: camelCase JSON fields
