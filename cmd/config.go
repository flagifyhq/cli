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

		apiURL := cfg.APIUrl
		if apiURL == "" {
			apiURL = "https://api.flagify.dev " + ui.Dim("(default)")
		}
		fmt.Println(ui.KeyValue("API URL:", apiURL))

		project := cfg.Project
		if project == "" {
			project = ui.Dim("(not set)")
		}
		fmt.Println(ui.KeyValue("Project:", project))

		loggedIn := ui.Red("no")
		if cfg.IsLoggedIn() {
			loggedIn = ui.Green("yes")
		}
		fmt.Println(ui.KeyValue("Logged in:", loggedIn))

		path, _ := config.Path()
		fmt.Println(ui.KeyValue("Config file:", ui.Dim(path)))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
