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

## Authentication

```bash
flagify login
```

Prompts for email and password. Credentials are stored in `~/.flagify/config.json`.

## Commands

### `flagify flags list`

List all flags in a project.

```bash
flagify flags list --project proj_xxx
flagify flags list -p proj_xxx -e prod
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment (dev, staging, prod) |

---

### `flagify flags create`

Create a new feature flag.

```bash
flagify flags create my-feature -p proj_xxx
flagify flags create checkout-variant -p proj_xxx --type string --description "A/B test for checkout"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | -- | Project key (required) |
| `--type` | `-t` | `boolean` | Flag type (boolean, string, number, json) |
| `--description` | -- | -- | Flag description |

---

### `flagify flags toggle`

Toggle a boolean flag on or off.

```bash
flagify flags toggle my-feature -p proj_xxx
flagify flags toggle dark-mode -p proj_xxx -e prod
```

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key (required) |
| `--environment` | `-e` | Environment (dev, staging, prod) |

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
  "project": "proj_xxx"
}
```

| Field | Description |
|-------|-------------|
| `accessToken` | JWT access token (set via `flagify login`) |
| `refreshToken` | JWT refresh token (set via `flagify login`) |
| `apiUrl` | API base URL (default: `https://api.flagify.dev`) |
| `project` | Default project key (avoids passing `--project` every time) |

View current config:

```bash
flagify config
```

## Global flags

These flags are available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project key |
| `--environment` | `-e` | Environment (dev, staging, prod) |
| `--help` | `-h` | Help for any command |

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
```

## License

MIT -- see [LICENSE](./LICENSE) for details.

---

<p align="center">
  <sub>Built with care by the <a href="https://flagify.dev">Flagify</a> team</sub>
</p>
