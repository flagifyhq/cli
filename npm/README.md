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

| Command | Description |
|---------|-------------|
| `flagify login` | Authenticate with email and password |
| `flagify logout` | Clear stored credentials |
| `flagify workspaces list` | List your workspaces |
| `flagify projects list` | List projects in a workspace |
| `flagify projects get` | Show project details with environments |
| `flagify flags list` | List all flags in a project |
| `flagify flags create` | Create a new feature flag |
| `flagify flags toggle` | Enable or disable a flag |
| `flagify config` | Show current configuration |
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
