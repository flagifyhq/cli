package cmd

import (
	"fmt"
	"os"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Current configuration:")
		fmt.Println()

		token := cfg.Token
		if token != "" {
			token = token[:4] + "***********" + token[len(token)-4:]
		} else {
			token = "(not set)"
		}
		fmt.Printf("  Token:       %s\n", token)

		apiURL := cfg.APIUrl
		if apiURL == "" {
			apiURL = "(default: https://api.flagify.dev)"
		}
		fmt.Printf("  API URL:     %s\n", apiURL)

		project := cfg.Project
		if project == "" {
			project = "(not set)"
		}
		fmt.Printf("  Project:     %s\n", project)
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
