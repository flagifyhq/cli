package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var flagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "Manage feature flags",
}

var flagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flags in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			return fmt.Errorf("--project is required")
		}
		fmt.Printf("Listing flags for project: %s\n", project)
		// TODO: call API
		return nil
	},
}

var flagsCreateCmd = &cobra.Command{
	Use:   "create [key]",
	Short: "Create a new feature flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		project, _ := cmd.Flags().GetString("project")
		flagType, _ := cmd.Flags().GetString("type")
		if project == "" {
			return fmt.Errorf("--project is required")
		}
		fmt.Printf("Creating flag %q (type: %s) in project: %s\n", key, flagType, project)
		// TODO: call API
		return nil
	},
}

var flagsToggleCmd = &cobra.Command{
	Use:   "toggle [key]",
	Short: "Toggle a boolean flag on/off",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			return fmt.Errorf("--project is required")
		}
		fmt.Printf("Toggling flag %q in project: %s\n", key, project)
		// TODO: call API
		return nil
	},
}

func init() {
	flagsCreateCmd.Flags().StringP("type", "t", "boolean", "Flag type (boolean, string, number, json)")
	flagsCreateCmd.Flags().String("description", "", "Flag description")

	flagsCmd.AddCommand(flagsListCmd)
	flagsCmd.AddCommand(flagsCreateCmd)
	flagsCmd.AddCommand(flagsToggleCmd)
	rootCmd.AddCommand(flagsCmd)
}
