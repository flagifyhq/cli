package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects in a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		workspaceID := rc.WorkspaceIdentifier()
		if workspaceID == "" {
			return fmt.Errorf("--workspace is required (or run 'flagify workspaces pick')")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		projects, err := client.ListProjects(workspaceID)
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(projects)
		}

		if len(projects) == 0 {
			fmt.Println(ui.Info("No projects found."))
			return nil
		}

		rows := make([][]string, len(projects))
		for i, p := range projects {
			rows[i] = []string{ui.Dim(p.ID), p.Name, p.Slug}
		}
		fmt.Println(ui.Table([]string{"ID", "Name", "Slug"}, rows))
		return nil
	},
}

var projectsGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get project details with environments",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing project ID. Usage: flagify projects get <id>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		project, err := client.GetProject(args[0])
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(project)
		}

		fmt.Println(ui.KeyValue("ID:", ui.Dim(project.ID)))
		fmt.Println(ui.KeyValue("Name:", project.Name))
		fmt.Println(ui.KeyValue("Slug:", project.Slug))

		if len(project.Environments) > 0 {
			fmt.Printf("\n  %s\n", ui.Bold("Environments"))
			rows := make([][]string, len(project.Environments))
			for i, e := range project.Environments {
				rows[i] = []string{ui.Dim(e.ID), e.Key, e.Name}
			}
			fmt.Println(ui.Table([]string{"ID", "Key", "Name"}, rows))
		}

		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a project and all its environments, flags, and segments",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing project ID. Usage: flagify projects delete <id>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		projID := args[0]

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Delete project %s? This will also delete all its environments, flags, segments, and API keys. This cannot be undone.", ui.Bold(projID)), yes)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}

		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		if err := client.DeleteProject(projID); err != nil {
			return handleAccessError(err, rc)
		}

		// If the deleted project was the active profile's default, scrub it so
		// future commands don't send a dangling ID to the API.
		if rc.Account != nil && rc.Account.Defaults.ProjectID == projID {
			store, err := config.LoadStore()
			if err == nil && rc.Profile != "" {
				if acc, ok := store.Accounts[rc.Profile]; ok {
					acc.Defaults.Project = ""
					acc.Defaults.ProjectID = ""
					_ = config.SaveStore(store)
				}
			}
		}

		fmt.Println(ui.Success("Deleted project " + projID))
		return nil
	},
}

var projectsPickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Interactively select a default project for the active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		if rc.Profile == "" {
			return fmt.Errorf("no active profile — run 'flagify login' first")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		workspaceID := rc.WorkspaceIdentifier()
		workspaceSlug := rc.Workspace
		if workspaceID == "" {
			ws, err := picker.PickWorkspace(client)
			if err != nil {
				return err
			}
			workspaceID = ws.ID
			workspaceSlug = ws.Slug
		}

		project, err := picker.PickProject(client, workspaceID)
		if err != nil {
			return err
		}

		// Persist on the resolved profile — never on "current" unconditionally.
		store, err := config.LoadStore()
		if err != nil {
			return err
		}
		acc, ok := store.Accounts[rc.Profile]
		if !ok {
			return fmt.Errorf("profile %q not found in local store", rc.Profile)
		}
		acc.Defaults.Workspace = workspaceSlug
		acc.Defaults.WorkspaceID = workspaceID
		acc.Defaults.Project = project.Slug
		acc.Defaults.ProjectID = project.ID
		if err := config.SaveStore(store); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Project set to %s %s", ui.Bold(project.Name), ui.Dim("("+project.Slug+")"))))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(projectsListCmd)
	ui.AddFormatFlag(projectsGetCmd)

	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsPickCmd)
	rootCmd.AddCommand(projectsCmd)
}
