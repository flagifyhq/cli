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
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		workspaceID := resolveFlag(cmd, "workspace", cfg.WorkspaceID)
		if workspaceID == "" {
			return fmt.Errorf("--workspace is required (or run 'flagify workspaces pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		projects, err := client.ListProjects(workspaceID)
		if err != nil {
			return handleAccessError(err)
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
		client, err := getClient()
		if err != nil {
			return err
		}

		project, err := client.GetProject(args[0])
		if err != nil {
			return handleAccessError(err)
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

		client, err := getClient()
		if err != nil {
			return err
		}

		if err := client.DeleteProject(projID); err != nil {
			return handleAccessError(err)
		}

		cfg, err := config.Load()
		if err == nil && cfg.ProjectID == projID {
			cfg.Project = ""
			cfg.ProjectID = ""
			_ = config.Save(cfg)
		}

		fmt.Println(ui.Success("Deleted project " + projID))
		return nil
	},
}

var projectsPickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Interactively select a default project",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

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

		project, err := picker.PickProject(client, workspaceID)
		if err != nil {
			return err
		}

		cfg.Project = project.Slug
		cfg.ProjectID = project.ID
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Project set to %s %s", ui.Bold(project.Name), ui.Dim("("+project.Slug+")"))))
		return nil
	},
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsPickCmd)
	rootCmd.AddCommand(projectsCmd)
}
