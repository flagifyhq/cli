package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
	"github.com/flagifyhq/cli/internal/ui"
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

	rc, err := config.Resolve(config.ResolveInput{
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
	if err != nil {
		return nil, err
	}

	// Preventive guard: catch a workspace the authenticated account is not a
	// member of before the command's business call turns it into a 403 loop.
	// A client that cannot be built here (not logged in) skips the guard — the
	// command's own getClientFromResolved reports the login error.
	if !skipsMembershipValidation(cmd) && rc.WorkspaceIdentifier() != "" {
		if client, clientErr := getClientFromResolved(rc); clientErr == nil {
			if err := validateWorkspaceMembership(rc, client, boolFlag(cmd, "yes")); err != nil {
				return nil, err
			}
		}
	}

	return rc, nil
}

// skipsMembershipValidation exempts the commands the preventive membership
// guard must never gate: identity/diagnostic commands (whoami, status), the
// commands that create or repair the scope itself (init, the interactive
// pickers), and workspaces list — the account-wide view users need to see
// which workspaces they CAN access. Gating any of these would turn the
// recovery path itself into the loop the guard exists to break.
func skipsMembershipValidation(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "whoami", "status", "init", "pick":
		return true
	case "list":
		return cmd.Parent() != nil && cmd.Parent().Name() == "workspaces"
	}
	return false
}

// confirmProjectFileClear seams ui.Confirm so tests can force a decline —
// ui.Confirm auto-confirms whenever stdout is not a TTY.
var confirmProjectFileClear = ui.Confirm

// validateWorkspaceMembership verifies the resolved workspace is one the
// authenticated account can actually reach, using the account-wide
// ListWorkspaces call. On a mismatch: in a TTY it offers a single re-pick
// (used for this invocation only) and then offers to reconcile a stale
// project-file binding; without a TTY it reconciles (auto-confirmed) and
// returns an actionable error so the next invocation resolves cleanly
// instead of replaying the mismatch. A failing ListWorkspaces skips the
// guard entirely — a defensive validation must not cost availability.
func validateWorkspaceMembership(rc *config.ResolvedConfig, client *api.Client, skipConfirm bool) error {
	identifier := rc.WorkspaceIdentifier()
	if identifier == "" {
		return nil
	}

	workspaces, err := client.ListWorkspaces()
	if err != nil {
		return nil
	}
	for _, ws := range workspaces {
		if ws.ID == identifier || ws.Slug == identifier {
			return nil
		}
	}

	if !ui.IsTTY() {
		clearProjectFileBinding(rc, identifier, skipConfirm)
		return fmt.Errorf(
			"access denied — this account is not a member of workspace %q. Re-scope with '-w <slug>' or '--workspace-id <ulid>', or run 'flagify projects pick' from a terminal",
			identifier)
	}

	fmt.Println(ui.Warning(fmt.Sprintf(
		"This account is not a member of workspace %s — pick one you can access.", ui.Bold(identifier))))
	picked, pickErr := picker.PickWorkspace(client)
	if pickErr != nil {
		return fmt.Errorf("workspace re-pick failed: %w", pickErr)
	}

	rc.Workspace = picked.Slug
	rc.WorkspaceID = picked.ID
	delete(rc.Sources, "workspace")
	delete(rc.Sources, "workspaceId")
	// Project identifiers carried by the stale binding are meaningless under
	// the re-picked workspace; flag/env-provided values stay untouched.
	if rc.Sources["projectId"] == config.SourceProjectFile {
		rc.ProjectID = ""
		delete(rc.Sources, "projectId")
	}
	if rc.Sources["project"] == config.SourceProjectFile {
		rc.Project = ""
		delete(rc.Sources, "project")
	}

	clearProjectFileBinding(rc, identifier, skipConfirm)
	return nil
}

// clearProjectFileBinding reconciles a stale workspace binding out of the
// committeable .flagify/project.json, so the next invocation does not replay
// the same mismatch. The file is only rewritten after confirmation
// (auto-confirm without a TTY, --yes respected) and always through
// config.ClearWorkspaceBinding so sibling fields are never half-cleared.
// Ephemeral env-token identities never touch the file. Reports whether the
// file was rewritten.
func clearProjectFileBinding(rc *config.ResolvedConfig, staleIdentifier string, skipConfirm bool) bool {
	if !projectFileBindsWorkspace(rc, staleIdentifier) || rc.EnvAccessToken != "" {
		return false
	}
	confirmed, err := confirmProjectFileClear(fmt.Sprintf(
		"Clear the stale workspace binding from %s? The file will keep its environment and preferred profile.",
		rc.ProjectFile.Path), skipConfirm)
	if err != nil || !confirmed {
		return false
	}
	cleared := config.ClearWorkspaceBinding(rc.ProjectFile.Data)
	if _, writeErr := config.WriteProjectFile(rc.ProjectFile.Dir, cleared); writeErr != nil {
		return false
	}
	return true
}

// projectFileBindsWorkspace reports whether the located project file is the
// origin of the failing workspace identifier — only then is clearing it a
// reconciliation rather than data loss.
func projectFileBindsWorkspace(rc *config.ResolvedConfig, identifier string) bool {
	if rc == nil || rc.ProjectFile == nil || identifier == "" {
		return false
	}
	pfd := rc.ProjectFile.Data
	return pfd.WorkspaceID == identifier || pfd.Workspace == identifier
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

// boolFlag is stringFlag's boolean sibling. It reads the flag value directly
// so persistent flags resolve even on commands whose flag sets were never
// merged by an Execute pass (e.g. rootCmd inside handleAccessError). Missing
// flags return false.
func boolFlag(cmd *cobra.Command, name string) bool {
	f := cmd.Flag(name)
	if f == nil {
		return false
	}
	return f.Value.String() == "true"
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
		return nil, fmt.Errorf("not logged in. Run 'flagify auth login' first")
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

// handleAccessError is the backstop behind the preventive membership guard:
// it intercepts a 403 from the API (e.g. membership changed between the
// guard's check and the real call) by clearing the active profile's defaults
// (workspace / project / environment) without a prompt, and — when the
// committeable project file is the origin of the failing workspace — offering
// to reconcile it too, so the stale binding is not replayed on the next
// invocation. The returned message states exactly what was cleared. The
// profile to touch is the one the resolver picked for this invocation —
// never "current" unconditionally; ephemeral env-token identities clear
// nothing.
func handleAccessError(err error, rc *config.ResolvedConfig) error {
	apiErr, ok := err.(*api.APIError)
	if !ok || apiErr.StatusCode != 403 {
		return err
	}

	clearedStore := false
	if rc != nil && rc.Profile != "" && rc.EnvAccessToken == "" {
		store, loadErr := config.LoadStore()
		if loadErr == nil {
			if acc, ok := store.Accounts[rc.Profile]; ok {
				acc.Defaults = config.Defaults{}
				if config.SaveStore(store) == nil {
					clearedStore = true
				}
			}
		}
	}

	fileStale := rc != nil && rc.EnvAccessToken == "" &&
		projectFileBindsWorkspace(rc, rc.WorkspaceIdentifier())
	clearedProjectFile := false
	if fileStale {
		clearedProjectFile = clearProjectFileBinding(rc, rc.WorkspaceIdentifier(), boolFlag(rootCmd, "yes"))
	}

	msg := "access denied — you are not a member of this workspace."
	var cleared []string
	if clearedStore {
		cleared = append(cleared, "the profile's saved defaults")
	}
	if clearedProjectFile {
		cleared = append(cleared, "the workspace binding in "+rc.ProjectFile.Path)
	}
	if len(cleared) > 0 {
		msg += " Cleared " + strings.Join(cleared, " and ") + "."
	}
	if fileStale && !clearedProjectFile {
		msg += " Note: " + rc.ProjectFile.Path + " still references this workspace."
	}
	msg += " Run 'flagify projects pick' or 'flagify auth switch <profile>'"
	return errors.New(msg)
}
