package cmd

import (
	"fmt"
	"os"

	"github.com/flagifyhq/cli/internal/api"
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

// getClient builds an API client for this invocation. Equivalent to
// getClientFromResolved(resolveContext(cmd)) — almost every RunE uses this form.
func getClient(cmd *cobra.Command) (*api.Client, error) {
	rc, err := resolveContext(cmd)
	if err != nil {
		return nil, err
	}
	return getClientFromResolved(rc)
}

// getClientFromResolved builds an API client that honors the resolved profile
// for the refresh-token callback. When tokens come from env vars (ephemeral
// override), no refresh callback is wired — the user asked for a one-shot
// identity and we refuse to persist anything.
func getClientFromResolved(rc *config.ResolvedConfig) (*api.Client, error) {
	if rc == nil {
		return nil, fmt.Errorf("no resolved context")
	}

	// Ephemeral env-provided token. Never persists refreshed tokens.
	if rc.EnvAccessToken != "" {
		client := api.NewClient(rc.EnvAccessToken)
		if rc.APIUrl != "" {
			client.SetBaseURL(rc.APIUrl)
		}
		if rc.EnvRefreshToken != "" {
			client.SetRefreshToken(rc.EnvRefreshToken)
		}
		return client, nil
	}

	if rc.Account == nil || rc.Account.AccessToken == "" {
		return nil, fmt.Errorf("not logged in. Run 'flagify login' first")
	}

	client := api.NewClient(rc.Account.AccessToken)
	if rc.APIUrl != "" {
		client.SetBaseURL(rc.APIUrl)
	}

	if rc.Account.RefreshToken != "" {
		client.SetRefreshToken(rc.Account.RefreshToken)
		profile := rc.Profile
		client.OnTokenRefresh = func(access, refresh string) {
			// Reload before writing so concurrent writes to sibling profiles are preserved.
			store, err := config.LoadStore()
			if err != nil {
				return
			}
			acc, ok := store.Accounts[profile]
			if !ok {
				// Profile was removed or renamed mid-flight; don't resurrect it.
				return
			}
			acc.AccessToken = access
			acc.RefreshToken = refresh
			_ = config.SaveStore(store)
		}
	}

	return client, nil
}

// handleAccessError intercepts a 403 from the API by clearing the active
// profile's defaults (workspace / project / environment) and returning a
// friendlier error. The profile to touch is the one the resolver picked for
// this invocation — never "current" unconditionally.
func handleAccessError(err error, rc *config.ResolvedConfig) error {
	apiErr, ok := err.(*api.APIError)
	if !ok || apiErr.StatusCode != 403 {
		return err
	}
	if rc != nil && rc.Profile != "" && rc.EnvAccessToken == "" {
		store, loadErr := config.LoadStore()
		if loadErr == nil {
			if acc, ok := store.Accounts[rc.Profile]; ok {
				acc.Defaults = config.Defaults{}
				_ = config.SaveStore(store)
			}
		}
	}
	return fmt.Errorf("access denied — you are not a member of this workspace. Config cleared, run 'flagify projects pick' or 'flagify auth switch <profile>'")
}
