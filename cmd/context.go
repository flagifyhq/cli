package cmd

import (
	"os"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/spf13/cobra"
)

// resolveContext is the cobra bridge to config.Resolve — it pulls flag values
// from the command, snapshots FLAGIFY_* env vars, loads the migrating store,
// and runs the resolver with cwd as the project-file starting point.
//
// Every top-level command that needs the resolved scope calls this exactly
// once per invocation so the Sources map reflects the real inputs.
func resolveContext(cmd *cobra.Command) (*config.ResolvedConfig, error) {
	store, err := config.LoadOrMigrate()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return config.Resolve(config.ResolveInput{
		Flags: config.FlagValues{
			Profile:     stringFlag(cmd, "profile"),
			Workspace:   stringFlag(cmd, "workspace"),
			WorkspaceID: stringFlag(cmd, "workspace-id"),
			Project:     stringFlag(cmd, "project"),
			ProjectID:   stringFlag(cmd, "project-id"),
			Environment: stringFlag(cmd, "environment"),
		},
		Env:   config.EnvFromOS(),
		Store: store,
		CWD:   cwd,
	})
}

// stringFlag returns the string value for a flag name regardless of whether
// it was declared locally or inherited from a persistent flag. Missing flags
// return "" so callers can treat them as "not provided".
func stringFlag(cmd *cobra.Command, name string) string {
	if cmd.Flag(name) == nil {
		return ""
	}
	v, _ := cmd.Flags().GetString(name)
	return v
}
