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
		workspace := resolveFlag(cmd, "workspace", cfg.Workspace)
		if workspace == "" {
			return fmt.Errorf("--workspace is required (or run 'flagify workspaces pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		projects, err := client.ListProjects(workspace)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
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
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		project, err := client.GetProject(args[0])
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
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

		workspace := resolveFlag(cmd, "workspace", cfg.Workspace)
		if workspace == "" {
			ws, err := picker.PickWorkspace(client)
			if err != nil {
				return err
			}
			workspace = ws.ID
			cfg.Workspace = workspace
		}

		project, err := picker.PickProject(client, workspace)
		if err != nil {
			return err
		}

		cfg.Project = project.ID
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Project set to %s %s", ui.Bold(project.Name), ui.Dim("("+project.ID+")"))))
		return nil
	},
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	projectsCmd.AddCommand(projectsPickCmd)
	rootCmd.AddCommand(projectsCmd)
}
