package cmd

import (
	"fmt"
	"os"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Inspect and update the .flagify/project.json committable file",
}

var projectBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Bind this repo to a local profile (does not modify the project file)",
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, _ := cmd.Flags().GetString("profile")
		if profile == "" {
			return fmt.Errorf("--profile is required — run 'flagify auth list' to see local profiles")
		}

		store, err := config.LoadOrMigrate()
		if err != nil {
			return err
		}
		if _, ok := store.Accounts[profile]; !ok {
			return fmt.Errorf("profile %q not found. Run 'flagify auth login --profile %s' first", profile, profile)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		// Bind the project-file's containing directory if one is visible. This
		// matches what the resolver later uses as the binding key.
		bindDir := cwd
		if pf, err := config.FindProjectFile(cwd); err != nil {
			return err
		} else if pf != nil {
			bindDir = pf.Dir
		}

		if err := config.BindProfile(store, bindDir, profile); err != nil {
			return err
		}
		if err := config.SaveStore(store); err != nil {
			return err
		}

		fmt.Println(ui.Success(fmt.Sprintf("Bound %s to profile %s", displayPath(bindDir), ui.Bold(profile))))
		return nil
	},
}

var projectStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the project file, binding, and resolved scope (alias for 'flagify status')",
	RunE:  statusCmd.RunE,
}

var projectSetCmd = &cobra.Command{
	Use:   "set <field> <value>",
	Short: "Update a field in .flagify/project.json",
	Long: `Update a single field of the .flagify/project.json committable file.

Valid fields:
  environment, project, project-id, workspace, workspace-id, preferred-profile`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		field, value := args[0], args[1]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		pf, err := config.FindProjectFile(cwd)
		if err != nil {
			return err
		}
		if pf == nil {
			return fmt.Errorf("no .flagify/project.json found — run 'flagify init' first")
		}

		switch field {
		case "environment":
			pf.Data.Environment = value
		case "project":
			pf.Data.Project = value
		case "project-id":
			pf.Data.ProjectID = value
		case "workspace":
			pf.Data.Workspace = value
		case "workspace-id":
			pf.Data.WorkspaceID = value
		case "preferred-profile":
			pf.Data.PreferredProfile = value
		default:
			return fmt.Errorf(
				"unknown field %q (valid: environment, project, project-id, workspace, workspace-id, preferred-profile)",
				field,
			)
		}

		if _, err := config.WriteProjectFile(pf.Dir, pf.Data); err != nil {
			return err
		}
		fmt.Println(ui.Success(fmt.Sprintf("Updated %s in %s", field, displayPath(pf.Path))))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(projectStatusCmd)

	projectCmd.AddCommand(projectBindCmd)
	projectCmd.AddCommand(projectStatusCmd)
	projectCmd.AddCommand(projectSetCmd)
	rootCmd.AddCommand(projectCmd)
}
