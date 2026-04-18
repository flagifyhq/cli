package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
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

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(workspaces)
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

var workspacesPickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Interactively select a default workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		ws, err := picker.PickWorkspace(client)
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cfg.Workspace = ws.Slug
		cfg.WorkspaceID = ws.ID
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Workspace set to %s %s", ui.Bold(ws.Name), ui.Dim("("+ws.Slug+")"))))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(workspacesListCmd)

	workspacesCmd.AddCommand(workspacesListCmd)
	workspacesCmd.AddCommand(workspacesPickCmd)
	rootCmd.AddCommand(workspacesCmd)
}
