package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

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
			fmt.Println("No workspaces found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSLUG\tPLAN")
		for _, ws := range workspaces {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ws.ID, ws.Name, ws.Slug, ws.Plan)
		}
		w.Flush()
		return nil
	},
}

func init() {
	workspacesCmd.AddCommand(workspacesListCmd)
	rootCmd.AddCommand(workspacesCmd)
}
