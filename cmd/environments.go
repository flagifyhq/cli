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
	Short: "Interactively select a default environment for the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		if rc.Profile == "" {
			return fmt.Errorf("no active profile — run 'flagify auth login' first")
		}
		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		projectID := rc.ProjectIdentifier()
		workspaceID := rc.WorkspaceIdentifier()
		workspaceSlug := rc.Workspace
		projectSlug := rc.Project

		if projectID == "" {
			if workspaceID == "" {
				ws, err := picker.PickWorkspace(client)
				if err != nil {
					return err
				}
				workspaceID = ws.ID
				workspaceSlug = ws.Slug
			}
			proj, err := picker.PickProject(client, workspaceID)
			if err != nil {
				return err
			}
			projectID = proj.ID
			projectSlug = proj.Slug
		}

		env, err := picker.PickEnvironment(client, projectID)
		if err != nil {
			return err
		}

		// Read and write in one locked transaction, after the picker: the user
		// may have spent a while choosing, and another invocation could have
		// rotated this profile's tokens in the meantime. Only the defaults are
		// touched, so that rotation survives.
		if err := config.UpdateStore(func(store *config.Store) error {
			acc, ok := store.Accounts[rc.Profile]
			if !ok {
				return fmt.Errorf("profile %q not found in local store", rc.Profile)
			}
			if workspaceSlug != "" {
				acc.Defaults.Workspace = workspaceSlug
			}
			if workspaceID != "" {
				acc.Defaults.WorkspaceID = workspaceID
			}
			if projectSlug != "" {
				acc.Defaults.Project = projectSlug
			}
			if projectID != "" {
				acc.Defaults.ProjectID = projectID
			}
			acc.Defaults.Environment = env.Key
			return nil
		}); err != nil {
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
