package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	cmdNameStyle = lipgloss.NewStyle().
			Foreground(cyan)

	cmdDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	flagShortStyle = lipgloss.NewStyle().
			Foreground(yellow)

	flagNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	flagDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	hintStyle = lipgloss.NewStyle().
			Foreground(dim)
)

// ApplyCustomHelp sets a styled help function on the given root command.
// It applies recursively to all subcommands.
func ApplyCustomHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(customHelp)
}

func customHelp(cmd *cobra.Command, args []string) {
	var b strings.Builder

	// Title line
	name := cmd.CommandPath()
	if cmd.Short != "" {
		fmt.Fprintf(&b, "%s %s %s\n", titleStyle.Render("flagify"), dimStyle.Render("—"), cmd.Short)
	} else {
		fmt.Fprintf(&b, "%s\n", titleStyle.Render(name))
	}

	// Long description
	if cmd.Long != "" && cmd.Long != cmd.Short {
		fmt.Fprintf(&b, "\n%s\n", cmdDescStyle.Render(cmd.Long))
	}

	// Usage
	fmt.Fprintf(&b, "\n%s\n", sectionStyle.Render("USAGE"))
	if cmd.Runnable() {
		fmt.Fprintf(&b, "  %s\n", dimStyle.Render(cmd.UseLine()))
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&b, "  %s\n", dimStyle.Render(cmd.CommandPath()+" <command> [flags]"))
	}

	// Commands
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&b, "\n%s\n", sectionStyle.Render("COMMANDS"))

		// Find max command name length for alignment
		maxLen := 0
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() || sub.Name() == "help" {
				if len(sub.Name()) > maxLen {
					maxLen = len(sub.Name())
				}
			}
		}

		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() && sub.Name() != "help" {
				continue
			}
			padding := strings.Repeat(" ", maxLen-len(sub.Name())+2)
			fmt.Fprintf(&b, "  %s%s%s\n",
				cmdNameStyle.Render(sub.Name()),
				padding,
				cmdDescStyle.Render(sub.Short),
			)
		}
	}

	// Flags
	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintf(&b, "\n%s\n", sectionStyle.Render("FLAGS"))
		printFlags(&b, cmd.LocalFlags())
	}

	// Global/inherited flags
	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintf(&b, "\n%s\n", sectionStyle.Render("GLOBAL FLAGS"))
		printFlags(&b, cmd.InheritedFlags())
	}

	// Examples
	if cmd.Example != "" {
		fmt.Fprintf(&b, "\n%s\n", sectionStyle.Render("EXAMPLES"))
		fmt.Fprintf(&b, "%s\n", dimStyle.Render(cmd.Example))
	}

	// Hint
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&b, "\n%s %s\n",
			hintStyle.Render("Use"),
			dimStyle.Render(fmt.Sprintf("%s <command> --help for more information about a command.", cmd.CommandPath())),
		)
	}

	fmt.Fprint(cmd.OutOrStdout(), b.String())
}

func printFlags(b *strings.Builder, flags interface{ FlagUsages() string }) {
	usage := flags.FlagUsages()
	for _, line := range strings.Split(strings.TrimRight(usage, "\n"), "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}

		// Parse the cobra flag usage line format: "  -s, --name type   description"
		trimmed := strings.TrimLeft(line, " ")

		// Find where the description starts (after multiple spaces following the flag def)
		// Cobra formats flags as: "  -s, --flag string   description"
		parts := splitFlagLine(trimmed)
		if parts.flag != "" {
			padding := strings.Repeat(" ", max(1, 28-len(parts.flag)))
			styled := "  "
			if parts.short != "" {
				styled += flagShortStyle.Render(parts.short) + ", "
			} else {
				styled += "    "
			}
			styled += flagNameStyle.Render(parts.long)
			if parts.typeName != "" {
				styled += " " + dimStyle.Render(parts.typeName)
			}
			// Calculate visual padding based on raw lengths
			rawLen := 4 // "  " + (shorthand or "    ")
			if parts.short != "" {
				rawLen = 2 + len(parts.short) + 2
			}
			rawLen += len(parts.long)
			if parts.typeName != "" {
				rawLen += 1 + len(parts.typeName)
			}
			padding = strings.Repeat(" ", max(2, 30-rawLen))
			styled += padding + flagDescStyle.Render(parts.desc)
			fmt.Fprintf(b, "%s\n", styled)
		} else {
			fmt.Fprintf(b, "  %s\n", dimStyle.Render(trimmed))
		}
	}
}

type flagParts struct {
	flag     string // full raw flag portion
	short    string // e.g. "-s"
	long     string // e.g. "--name"
	typeName string // e.g. "string"
	desc     string // description
}

func splitFlagLine(line string) flagParts {
	var fp flagParts

	// Find description: look for 3+ spaces which separate flag def from description
	descIdx := -1
	spaceCount := 0
	for i, ch := range line {
		if ch == ' ' {
			spaceCount++
			if spaceCount >= 3 {
				descIdx = i + 1
				break
			}
		} else {
			spaceCount = 0
		}
	}

	flagPart := line
	if descIdx > 0 {
		flagPart = strings.TrimRight(line[:descIdx-2], " ")
		fp.desc = strings.TrimLeft(line[descIdx:], " ")
	}

	fp.flag = flagPart

	// Parse short and long flag names
	tokens := strings.Fields(flagPart)
	for i, tok := range tokens {
		tok = strings.TrimRight(tok, ",")
		if strings.HasPrefix(tok, "--") {
			fp.long = tok
			// Remaining tokens after long flag are the type
			if i+1 < len(tokens) {
				fp.typeName = strings.Join(tokens[i+1:], " ")
			}
			break
		} else if strings.HasPrefix(tok, "-") {
			fp.short = tok
		}
	}

	return fp
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
