package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "flagify",
	Short: "Flagify CLI — manage feature flags from the terminal",
	Long:  "The official Flagify CLI for creating, listing, and managing feature flags.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("workspace", "w", "", "Workspace ID")
	rootCmd.PersistentFlags().StringP("project", "p", "", "Project key")
	rootCmd.PersistentFlags().StringP("environment", "e", "", "Environment (dev, staging, prod)")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "Skip confirmation prompts")
}
