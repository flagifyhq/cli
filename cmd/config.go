package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if ui.IsJSON(cmd) {
			path, _ := config.Path()
			apiURL := cfg.APIUrl
			if apiURL == "" {
				apiURL = "https://api.flagify.dev"
			}
			return ui.PrintJSON(map[string]any{
				"loggedIn":    cfg.IsLoggedIn(),
				"apiUrl":      apiURL,
				"consoleUrl":  cfg.ConsoleUrl,
				"workspace":   cfg.Workspace,
				"workspaceId": cfg.WorkspaceID,
				"project":     cfg.Project,
				"projectId":   cfg.ProjectID,
				"environment": cfg.Environment,
				"configFile":  path,
			})
		}

		fmt.Println()
		fmt.Println(ui.Bold("  Configuration"))
		fmt.Println()

		loggedIn := ui.Red("no")
		if cfg.IsLoggedIn() {
			loggedIn = ui.Green("yes")
		}
		fmt.Println(ui.KeyValue("Logged in:", loggedIn))

		apiURL := cfg.APIUrl
		if apiURL == "" {
			apiURL = "https://api.flagify.dev " + ui.Dim("(default)")
		}
		fmt.Println(ui.KeyValue("API URL:", apiURL))

		consoleURL := cfg.ConsoleUrl
		if consoleURL == "" {
			consoleURL = ui.Dim("(not set)")
		}
		fmt.Println(ui.KeyValue("Console URL:", consoleURL))

		fmt.Println()
		fmt.Println(ui.Bold("  Scope"))
		fmt.Println()

		workspace := cfg.Workspace
		if workspace == "" {
			workspace = ui.Dim("(not set)")
		} else if cfg.WorkspaceID != "" {
			workspace += " " + ui.Dim("("+cfg.WorkspaceID+")")
		}
		fmt.Println(ui.KeyValue("Workspace:", workspace))

		project := cfg.Project
		if project == "" {
			project = ui.Dim("(not set)")
		} else if cfg.ProjectID != "" {
			project += " " + ui.Dim("("+cfg.ProjectID+")")
		}
		fmt.Println(ui.KeyValue("Project:", project))

		environment := cfg.Environment
		if environment == "" {
			environment = ui.Dim("(not set)")
		}
		fmt.Println(ui.KeyValue("Environment:", environment))

		fmt.Println()
		path, _ := config.Path()
		fmt.Println(ui.KeyValue("Config file:", ui.Dim(path)))
		fmt.Println()

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long:  "Set a configuration value. Valid keys: api-url, console-url, workspace, project, environment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var oldValue string
		switch key {
		case "api-url":
			oldValue = cfg.APIUrl
			cfg.APIUrl = value
		case "console-url":
			oldValue = cfg.ConsoleUrl
			cfg.ConsoleUrl = value
		case "workspace":
			oldValue = cfg.Workspace
			cfg.Workspace = value
		case "project":
			oldValue = cfg.Project
			cfg.Project = value
		case "environment":
			oldValue = cfg.Environment
			cfg.Environment = value
		default:
			return fmt.Errorf("unknown config key %q. Valid keys: api-url, console-url, workspace, project, environment", key)
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if oldValue == "" {
			fmt.Println(ui.Success(fmt.Sprintf("%s set to %s", ui.Bold(key), ui.Cyan(value))))
		} else {
			fmt.Println(ui.Success(fmt.Sprintf("%s updated: %s %s %s", ui.Bold(key), ui.Dim(oldValue), ui.Arrow(), ui.Cyan(value))))
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Long:  "Get a configuration value. Valid keys: api-url, console-url, workspace, project, environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var value string
		switch key {
		case "api-url":
			value = cfg.APIUrl
		case "console-url":
			value = cfg.ConsoleUrl
		case "workspace":
			value = cfg.Workspace
		case "project":
			value = cfg.Project
		case "environment":
			value = cfg.Environment
		default:
			return fmt.Errorf("unknown config key %q. Valid keys: api-url, console-url, workspace, project, environment", key)
		}

		if value == "" {
			fmt.Println(ui.Dim("(not set)"))
		} else {
			fmt.Println(value)
		}
		return nil
	},
}

func init() {
	ui.AddFormatFlag(configCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}
