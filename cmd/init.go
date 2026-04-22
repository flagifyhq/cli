package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .flagify/project.json committable file for this repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}

		preferred, _ := cmd.Flags().GetString("preferred-profile")
		if preferred == "" {
			preferred = rc.Profile
		}

		pfd := config.ProjectFileData{
			Version:          config.ProjectFileVersion,
			WorkspaceID:      rc.WorkspaceID,
			Workspace:        rc.Workspace,
			ProjectID:        rc.ProjectID,
			Project:          rc.Project,
			Environment:      rc.Environment,
			PreferredProfile: preferred,
		}

		if pfd.WorkspaceID == "" && pfd.Workspace == "" {
			return fmt.Errorf("workspace is required: pass --workspace-id or --workspace, or run 'flagify login' so your profile has defaults")
		}
		if pfd.ProjectID == "" && pfd.Project == "" {
			return fmt.Errorf("project is required: pass --project-id or --project")
		}
		if pfd.Environment == "" {
			pfd.Environment = "development"
		}

		if printOnly, _ := cmd.Flags().GetBool("print"); printOnly {
			data, err := json.MarshalIndent(pfd, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cwdAbs, err := filepath.Abs(cwd)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")
		yes, _ := cmd.Flags().GetBool("yes")

		existing, err := config.FindProjectFile(cwd)
		if err != nil {
			return err
		}

		// Only treat a match as "here" if the project file sits in this exact
		// directory. A project file inherited from a parent is a different story.
		if existing != nil && existing.Dir == cwdAbs {
			if projectFilesEqual(existing.Data, pfd) {
				fmt.Println(ui.Info(fmt.Sprintf("Already initialized: %s", existing.Path)))
				return nil
			}
			if !force {
				if !ui.IsTTY() {
					return fmt.Errorf(
						"project file %s already exists and would change — rerun with --force to overwrite in non-interactive shells",
						existing.Path,
					)
				}
				confirmed, err := ui.Confirm(
					fmt.Sprintf("Overwrite %s with new scope?", existing.Path),
					yes,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Println(ui.Info("Cancelled."))
					return nil
				}
			}
		}

		written, err := config.WriteProjectFile(cwd, pfd)
		if err != nil {
			return err
		}
		fmt.Println(ui.Success(fmt.Sprintf("Wrote %s", displayPath(written.Path))))
		return nil
	},
}

// projectFilesEqual compares the two committable shapes field by field.
// Version is intentionally normalized — a missing version on disk still matches
// a freshly-minted v1 record.
func projectFilesEqual(a, b config.ProjectFileData) bool {
	norm := func(d config.ProjectFileData) config.ProjectFileData {
		if d.Version == 0 {
			d.Version = config.ProjectFileVersion
		}
		return d
	}
	return norm(a) == norm(b)
}

func init() {
	initCmd.Flags().String("preferred-profile", "", "preferredProfile hint to write in .flagify/project.json (defaults to resolved profile)")
	initCmd.Flags().Bool("print", false, "Print the JSON that would be written without touching disk")
	initCmd.Flags().Bool("force", false, "Overwrite an existing project file without prompting")
	rootCmd.AddCommand(initCmd)
}
