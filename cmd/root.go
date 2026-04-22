package cmd

import (
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "flagify",
	Short:         "Flagify CLI — manage feature flags from the terminal",
	Long:          "The official Flagify CLI for creating, listing, and managing feature flags.",
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("profile", "", "Flagify profile to use (defaults to the active profile)")
	rootCmd.PersistentFlags().StringP("workspace", "w", "", "Workspace slug")
	rootCmd.PersistentFlags().String("workspace-id", "", "Workspace ULID (wins over --workspace when both are set)")
	rootCmd.PersistentFlags().StringP("project", "p", "", "Project slug")
	rootCmd.PersistentFlags().String("project-id", "", "Project ULID (wins over --project)")
	rootCmd.PersistentFlags().StringP("environment", "e", "", "Environment key (matches the environment slug in the API; defaults to development|staging|production)")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "Skip confirmation prompts")

	ui.ApplyCustomHelp(rootCmd)
}
