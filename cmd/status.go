package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the resolved Flagify context for this invocation",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(statusPayloadFrom(rc))
		}

		printStatusHuman(rc)
		return nil
	},
}

type statusField struct {
	Value  string `json:"value,omitempty"`
	Source string `json:"source,omitempty"`
}

type statusPayload struct {
	Profile     statusField `json:"profile"`
	Email       string      `json:"email,omitempty"`
	Workspace   statusField `json:"workspace"`
	WorkspaceID statusField `json:"workspaceId"`
	Project     statusField `json:"project"`
	ProjectID   statusField `json:"projectId"`
	Environment statusField `json:"environment"`
	APIUrl      statusField `json:"apiUrl"`
	ConsoleUrl  statusField `json:"consoleUrl"`
	ProjectFile string      `json:"projectFile,omitempty"`
	Binding     string      `json:"binding,omitempty"`
	StorePath   string      `json:"storePath"`
}

func statusPayloadFrom(rc *config.ResolvedConfig) statusPayload {
	field := func(key, value string) statusField {
		return statusField{Value: value, Source: string(rc.Sources[key])}
	}
	storePath, _ := config.Path()
	p := statusPayload{
		Profile:     field("profile", rc.Profile),
		Workspace:   field("workspace", rc.Workspace),
		WorkspaceID: field("workspaceId", rc.WorkspaceID),
		Project:     field("project", rc.Project),
		ProjectID:   field("projectId", rc.ProjectID),
		Environment: field("environment", rc.Environment),
		APIUrl:      field("apiUrl", rc.APIUrl),
		ConsoleUrl:  field("consoleUrl", rc.ConsoleUrl),
		StorePath:   storePath,
	}
	if rc.Account != nil && rc.Account.User != nil {
		p.Email = rc.Account.User.Email
	}
	if rc.ProjectFile != nil {
		p.ProjectFile = rc.ProjectFile.Path
	}
	// Surface a binding only if one was actually in play; empty string otherwise.
	if string(rc.Sources["profile"]) == string(config.SourceBinding) {
		p.Binding = rc.Profile
	}
	return p
}

func printStatusHuman(rc *config.ResolvedConfig) {
	row := func(label, value, source string) {
		if value == "" {
			value = ui.Dim("—")
		}
		src := ""
		if source != "" {
			src = ui.Dim("(" + source + ")")
		}
		fmt.Printf("  %s  %s  %s\n", ui.Label(padRight(label, 14)), value, src)
	}

	row("Profile", rc.Profile, string(rc.Sources["profile"]))
	if rc.Account != nil && rc.Account.User != nil && rc.Account.User.Email != "" {
		line := rc.Account.User.Email
		if name := rc.Account.User.Name; name != "" {
			line = fmt.Sprintf("%s %s", ui.Bold(name), ui.Dim("<"+rc.Account.User.Email+">"))
		}
		row("User", line, "profile")
	}
	row("Workspace", formatScoped(rc.Workspace, rc.WorkspaceID), string(rc.Sources["workspace"]))
	row("Project", formatScoped(rc.Project, rc.ProjectID), string(rc.Sources["project"]))
	row("Environment", rc.Environment, string(rc.Sources["environment"]))
	row("API URL", rc.APIUrl, string(rc.Sources["apiUrl"]))
	row("Console URL", rc.ConsoleUrl, string(rc.Sources["consoleUrl"]))
	if rc.ProjectFile != nil {
		row("Project file", rc.ProjectFile.Path, "")
	}
	storePath, _ := config.Path()
	row("Global store", displayPath(storePath), "")
}

func formatScoped(slug, id string) string {
	switch {
	case slug == "" && id == "":
		return ""
	case slug == "":
		return id
	case id == "":
		return slug
	default:
		return fmt.Sprintf("%s %s", slug, ui.Dim("("+id+")"))
	}
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	if path == home {
		return "~"
	}
	return path
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func init() {
	ui.AddFormatFlag(statusCmd)
	rootCmd.AddCommand(statusCmd)
}
