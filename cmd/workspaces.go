package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var workspacesCmd = &cobra.Command{
	Use:   "workspaces",
	Short: "Manage workspaces",
}

var workspacesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		workspaces, err := client.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("failed to list workspaces: %w", err)
		}

		if len(workspaces) == 0 {
			fmt.Println(ui.Info("No workspaces found."))
			return nil
		}

		rows := make([][]string, len(workspaces))
		for i, ws := range workspaces {
			rows[i] = []string{ui.Dim(ws.ID), ws.Name, ws.Slug, ws.Plan}
		}
		fmt.Println(ui.Table([]string{"ID", "Name", "Slug", "Plan"}, rows))
		return nil
	},
}

func init() {
	workspacesCmd.AddCommand(workspacesListCmd)
	rootCmd.AddCommand(workspacesCmd)
}
