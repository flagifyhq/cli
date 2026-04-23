package cmd

import (
	"fmt"
	"sort"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Flagify authentication profiles",
}

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "List authenticated profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := config.LoadOrMigrate()
		if err != nil {
			return err
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(profileViewList(store))
		}

		if len(store.Accounts) == 0 {
			fmt.Println(ui.Info("No profiles yet. Run 'flagify auth login' to add one."))
			return nil
		}

		headers := []string{"", "PROFILE", "EMAIL", "SCOPE", "STATUS"}
		rows := make([][]string, 0, len(store.Accounts))
		for _, name := range sortedAccountNames(store.Accounts) {
			acc := store.Accounts[name]
			marker := " "
			if name == store.Current {
				marker = ui.Cyan("●")
			}
			email := "—"
			if acc.User != nil && acc.User.Email != "" {
				email = acc.User.Email
			}
			status := ui.Green("logged in")
			if acc.AccessToken == "" {
				status = ui.Dim("logged out")
			}
			rows = append(rows, []string{marker, name, email, formatScope(acc.Defaults), status})
		}
		fmt.Println(ui.Table(headers, rows))
		return nil
	},
}

var authSwitchCmd = &cobra.Command{
	Use:   "switch <profile>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		store, err := config.LoadOrMigrate()
		if err != nil {
			return err
		}
		if _, ok := store.Accounts[name]; !ok {
			return fmt.Errorf("profile %q not found. Run 'flagify auth list' to see available profiles", name)
		}
		if store.Current == name {
			fmt.Println(ui.Info(fmt.Sprintf("Profile %s is already active.", ui.Bold(name))))
			return nil
		}
		store.Current = name
		if err := config.SaveStore(store); err != nil {
			return err
		}
		fmt.Println(ui.Success(fmt.Sprintf("Switched to profile %s", ui.Bold(name))))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of a profile (keeps defaults)",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		profile, _ := cmd.Flags().GetString("profile")
		return runLogout(profile, all)
	},
}

var authRemoveCmd = &cobra.Command{
	Use:   "remove <profile>",
	Short: "Delete a profile and any bindings pointing to it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		yes, _ := cmd.Flags().GetBool("yes")

		store, err := config.LoadOrMigrate()
		if err != nil {
			return err
		}
		if _, ok := store.Accounts[name]; !ok {
			return fmt.Errorf("profile %q not found", name)
		}

		confirmed, err := ui.Confirm(
			fmt.Sprintf("Remove profile %s? This deletes tokens and defaults locally.", ui.Bold(name)),
			yes,
		)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}

		delete(store.Accounts, name)
		removed := config.PurgeBindingsForProfile(store, name)
		if store.Current == name {
			store.Current = firstAccountName(store.Accounts)
		}
		if err := config.SaveStore(store); err != nil {
			return err
		}

		msg := fmt.Sprintf("Removed profile %s", ui.Bold(name))
		if removed > 0 {
			msg += fmt.Sprintf(" %s", ui.Dim(fmt.Sprintf("(%d binding(s) cleaned up)", removed)))
		}
		fmt.Println(ui.Success(msg))
		return nil
	},
}

var authRenameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a profile and update any bindings that point to it",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, newName := args[0], args[1]
		if oldName == newName {
			return nil
		}

		store, err := config.LoadOrMigrate()
		if err != nil {
			return err
		}
		acc, ok := store.Accounts[oldName]
		if !ok {
			return fmt.Errorf("profile %q not found", oldName)
		}
		if _, exists := store.Accounts[newName]; exists {
			return fmt.Errorf("profile %q already exists", newName)
		}

		store.Accounts[newName] = acc
		delete(store.Accounts, oldName)
		if store.Current == oldName {
			store.Current = newName
		}
		for path, b := range store.Bindings {
			if b.Profile == oldName {
				store.Bindings[path] = config.Binding{Profile: newName}
			}
		}
		if err := config.SaveStore(store); err != nil {
			return err
		}

		fmt.Println(ui.Success(fmt.Sprintf("Renamed profile %s to %s", ui.Bold(oldName), ui.Bold(newName))))
		return nil
	},
}

// runLogout signs out the selected profile. profile==""/all==false means "the
// current profile". all==true wins.
func runLogout(profile string, all bool) error {
	store, err := config.LoadOrMigrate()
	if err != nil {
		return err
	}

	if all {
		if len(store.Accounts) == 0 {
			fmt.Println(ui.Info("No profiles to log out of."))
			return nil
		}
		for _, acc := range store.Accounts {
			acc.AccessToken = ""
			acc.RefreshToken = ""
		}
		if err := config.SaveStore(store); err != nil {
			return err
		}
		fmt.Println(ui.Success("Logged out of every profile."))
		return nil
	}

	target := profile
	if target == "" {
		target = store.Current
	}
	if target == "" {
		fmt.Println(ui.Info("No profile is currently active. Nothing to do."))
		return nil
	}
	acc, ok := store.Accounts[target]
	if !ok {
		return fmt.Errorf("profile %q not found", target)
	}
	acc.AccessToken = ""
	acc.RefreshToken = ""
	if err := config.SaveStore(store); err != nil {
		return err
	}
	fmt.Println(ui.Success(fmt.Sprintf("Logged out of profile %s", ui.Bold(target))))
	return nil
}

type profileView struct {
	Name        string `json:"name"`
	Current     bool   `json:"current"`
	Email       string `json:"email,omitempty"`
	APIUrl      string `json:"apiUrl,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
	Project     string `json:"project,omitempty"`
	Environment string `json:"environment,omitempty"`
	LoggedIn    bool   `json:"loggedIn"`
}

func profileViewList(store *config.Store) []profileView {
	names := sortedAccountNames(store.Accounts)
	out := make([]profileView, 0, len(names))
	for _, n := range names {
		acc := store.Accounts[n]
		pv := profileView{
			Name:        n,
			Current:     n == store.Current,
			APIUrl:      acc.APIUrl,
			Workspace:   acc.Defaults.Workspace,
			Project:     acc.Defaults.Project,
			Environment: acc.Defaults.Environment,
			LoggedIn:    acc.AccessToken != "",
		}
		if acc.User != nil {
			pv.Email = acc.User.Email
		}
		out = append(out, pv)
	}
	return out
}

func sortedAccountNames(m map[string]*config.Account) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstAccountName(m map[string]*config.Account) string {
	names := sortedAccountNames(m)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func formatScope(d config.Defaults) string {
	ws := orDash(d.Workspace)
	pr := orDash(d.Project)
	env := orDash(d.Environment)
	return fmt.Sprintf("%s / %s / %s", ws, pr, env)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func init() {
	ui.AddFormatFlag(authListCmd)
	authLogoutCmd.Flags().String("profile", "", "Profile to log out of (defaults to current)")
	authLogoutCmd.Flags().Bool("all", false, "Log out of every profile")

	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authSwitchCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authRemoveCmd)
	authCmd.AddCommand(authRenameCmd)

	rootCmd.AddCommand(authCmd)
}
