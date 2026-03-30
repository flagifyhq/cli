package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

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
		workspace, _ := cmd.Flags().GetString("workspace")
		if workspace == "" {
			return fmt.Errorf("--workspace is required")
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
			fmt.Println("No projects found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSLUG")
		for _, p := range projects {
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.ID, p.Name, p.Slug)
		}
		w.Flush()
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

		fmt.Printf("ID:        %s\n", project.ID)
		fmt.Printf("Name:      %s\n", project.Name)
		fmt.Printf("Slug:      %s\n", project.Slug)

		if len(project.Environments) > 0 {
			fmt.Println("\nEnvironments:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  ID\tKEY\tNAME")
			for _, e := range project.Environments {
				fmt.Fprintf(w, "  %s\t%s\t%s\n", e.ID, e.Key, e.Name)
			}
			w.Flush()
		}

		return nil
	},
}

func init() {
	projectsListCmd.Flags().StringP("workspace", "w", "", "Workspace ID")

	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsGetCmd)
	rootCmd.AddCommand(projectsCmd)
}
