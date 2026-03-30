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

# List feature flags
flagify flags list

# Create a new flag
flagify flags create --key my-flag --name "My Flag" --type boolean

# Toggle a flag
flagify flags toggle my-flag --enabled
```

## Commands

| Command | Description |
|---------|-------------|
| `flagify login` | Authenticate with email and password |
| `flagify logout` | Clear stored credentials |
| `flagify flags list` | List all flags in the current project |
| `flagify flags create` | Create a new feature flag |
| `flagify flags toggle` | Enable or disable a flag |
| `flagify version` | Print CLI version |

## Configuration

The CLI stores credentials in `~/.flagify/config.json`. Use `--project` and `--environment` flags to target specific resources.

## Requirements

- Flagify account ([flagify.dev](https://flagify.dev))

## License

MIT
