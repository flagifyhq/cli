package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var environmentsCmd = &cobra.Command{
	Use:   "environments",
	Short: "Manage environments",
}

var environmentsPickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Interactively select a default environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		projectID := resolveFlag(cmd, "project", cfg.ProjectID)
		if projectID == "" {
			workspaceID := resolveFlag(cmd, "workspace", cfg.WorkspaceID)
			if workspaceID == "" {
				ws, err := picker.PickWorkspace(client)
				if err != nil {
					return err
				}
				workspaceID = ws.ID
				cfg.Workspace = ws.Slug
				cfg.WorkspaceID = ws.ID
			}

			proj, err := picker.PickProject(client, workspaceID)
			if err != nil {
				return err
			}
			projectID = proj.ID
			cfg.Project = proj.Slug
			cfg.ProjectID = proj.ID
		}

		env, err := picker.PickEnvironment(client, projectID)
		if err != nil {
			return err
		}

		cfg.Environment = env.Key
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Environment set to %s %s", ui.Bold(env.Name), ui.Dim("("+env.Key+")"))))
		return nil
	},
}

func init() {
	environmentsCmd.AddCommand(environmentsPickCmd)
	rootCmd.AddCommand(environmentsCmd)
}
