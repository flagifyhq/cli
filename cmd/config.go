package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
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
			apiURL = "https://api.flagify.dev (default)"
		}
		fmt.Printf("API URL:      %s\n", apiURL)

		project := cfg.Project
		if project == "" {
			project = "(not set)"
		}
		fmt.Printf("Project:      %s\n", project)

		if cfg.IsLoggedIn() {
			fmt.Printf("Logged in:    yes\n")
		} else {
			fmt.Printf("Logged in:    no\n")
		}

		path, _ := config.Path()
		fmt.Printf("Config file:  %s\n", path)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
