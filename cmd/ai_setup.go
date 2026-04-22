package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flagifyhq/cli/internal/templates"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var supportedTools = []string{"claude", "cursor", "copilot", "windsurf"}

var aiSetupCmd = &cobra.Command{
	Use:   "ai-setup",
	Short: "Generate AI tool config files for your project",
	Long:  "Creates config files (.cursorrules, .windsurfrules, CLAUDE.md, copilot-instructions) so AI coding tools understand your Flagify setup.",
	RunE: func(cmd *cobra.Command, args []string) error {
		tool, _ := cmd.Flags().GetString("tool")
		includeFlags, _ := cmd.Flags().GetBool("include-flags")

		if tool != "" && !isValidTool(tool) {
			return fmt.Errorf("unknown tool %q. Supported: %s", tool, strings.Join(supportedTools, ", "))
		}

		var flagsContext string
		if includeFlags {
			ctx, err := fetchFlagsContext(cmd)
			if err != nil {
				fmt.Println(ui.Warning(fmt.Sprintf("Could not fetch flags: %s. Generating without flag list.", err)))
			} else {
				flagsContext = ctx
			}
		}

		tools := supportedTools
		if tool != "" {
			tools = []string{tool}
		}

		data := templates.Data{FlagsContext: flagsContext}

		generated := 0
		for _, t := range tools {
			path, err := generateToolConfig(t, data)
			if err != nil {
				fmt.Println(ui.Warning(fmt.Sprintf("Failed to generate %s config: %s", t, err)))
				continue
			}
			fmt.Println(ui.Success(fmt.Sprintf("Generated %s %s", ui.Bold(t), ui.Dim(path))))
			generated++
		}

		if generated == 0 {
			return fmt.Errorf("no config files were generated")
		}

		fmt.Println()
		fmt.Println(ui.Info(fmt.Sprintf("%d config file(s) created. Commit them to your repo.", generated)))
		return nil
	},
}

func isValidTool(tool string) bool {
	for _, t := range supportedTools {
		if t == tool {
			return true
		}
	}
	return false
}

func fetchFlagsContext(cmd *cobra.Command) (string, error) {
	rc, err := resolveContext(cmd)
	if err != nil {
		return "", err
	}

	projectID := rc.ProjectIdentifier()
	if projectID == "" {
		return "", fmt.Errorf("no project configured. Run %s first", ui.Bold("flagify projects pick"))
	}

	client, err := getClientFromResolved(rc)
	if err != nil {
		return "", err
	}

	flags, err := client.ListFlags(projectID)
	if err != nil {
		return "", handleAccessError(err, rc)
	}
	if len(flags) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n## Active flags\n\n")
	sb.WriteString("| Flag | Type | Environments |\n")
	sb.WriteString("|------|------|--------------|\n")
	for _, f := range flags {
		envs := make([]string, 0, len(f.Environments))
		for _, e := range f.Environments {
			state := "off"
			if e.Enabled {
				state = "on"
			}
			envs = append(envs, e.EnvironmentKey+":"+state)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", f.Key, f.Type, strings.Join(envs, ", ")))
	}
	return sb.String(), nil
}

func generateToolConfig(tool string, data templates.Data) (string, error) {
	switch tool {
	case "claude":
		return generateClaude(data)
	case "cursor":
		return generateCursor(data)
	case "copilot":
		return generateCopilot(data)
	case "windsurf":
		return generateWindsurf(data)
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func generateClaude(data templates.Data) (string, error) {
	content, err := templates.Render("claude.md.tmpl", data)
	if err != nil {
		return "", err
	}

	claudeMdPath := "CLAUDE.md"
	existingContent := ""
	if raw, err := os.ReadFile(claudeMdPath); err == nil {
		existingContent = string(raw)
	}

	if strings.Contains(existingContent, "## Feature Flags") {
		start := strings.Index(existingContent, "## Feature Flags")
		end := len(existingContent)
		rest := existingContent[start+len("## Feature Flags"):]
		if idx := strings.Index(rest, "\n## "); idx != -1 {
			end = start + len("## Feature Flags") + idx
		}
		existingContent = existingContent[:start] + content + existingContent[end:]
	} else if existingContent != "" {
		existingContent = existingContent + "\n" + content
	} else {
		existingContent = content
	}

	if err := writeFile(claudeMdPath, existingContent); err != nil {
		return "", err
	}

	commandTemplates := map[string]string{
		".claude/commands/flagify-create.md": "claude-command-create.md.tmpl",
		".claude/commands/flagify-toggle.md": "claude-command-toggle.md.tmpl",
		".claude/commands/flagify-list.md":   "claude-command-list.md.tmpl",
	}

	for path, tmplName := range commandTemplates {
		body, err := templates.Render(tmplName, data)
		if err != nil {
			return "", err
		}
		if err := writeFile(path, body); err != nil {
			return "", err
		}
	}

	return "CLAUDE.md + .claude/commands/", nil
}

func generateCursor(data templates.Data) (string, error) {
	content, err := templates.Render("cursor.tmpl", data)
	if err != nil {
		return "", err
	}
	path := ".cursorrules"
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func generateCopilot(data templates.Data) (string, error) {
	content, err := templates.Render("copilot.md.tmpl", data)
	if err != nil {
		return "", err
	}
	path := ".github/copilot-instructions.md"
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func generateWindsurf(data templates.Data) (string, error) {
	content, err := templates.Render("windsurf.tmpl", data)
	if err != nil {
		return "", err
	}
	path := ".windsurfrules"
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

func init() {
	aiSetupCmd.Flags().String("tool", "", "Generate config for a specific tool (claude, cursor, copilot, windsurf)")
	aiSetupCmd.Flags().Bool("include-flags", false, "Include current flag list in generated context")
	rootCmd.AddCommand(aiSetupCmd)
}
