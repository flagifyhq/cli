package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/picker"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var kebabCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func toKebabCase(s string) string {
	var result []rune
	for i, r := range s {
		if r == '_' || r == ' ' {
			result = append(result, '-')
		} else if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(rune(s[i-1])) {
				result = append(result, '-')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return strings.Trim(string(result), "-")
}

func resolveFlag(cmd *cobra.Command, name string, configValue string) string {
	val, _ := cmd.Flags().GetString(name)
	if val != "" {
		return val
	}
	return configValue
}

// handleAccessError checks if an API error is a 403 Forbidden and clears
// workspace/project/environment from config since the user lost access.
func handleAccessError(err error) error {
	if apiErr, ok := err.(*api.APIError); ok && apiErr.StatusCode == 403 {
		cfg, loadErr := config.Load()
		if loadErr == nil {
			cfg.Workspace = ""
			cfg.WorkspaceID = ""
			cfg.Project = ""
			cfg.ProjectID = ""
			cfg.Environment = ""
			config.Save(cfg)
		}
		return fmt.Errorf("access denied — you are not a member of this workspace. Config cleared, run 'flagify projects pick'")
	}
	return err
}

func getClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in. Run 'flagify login' first")
	}
	client := api.NewClient(cfg.GetToken())
	if cfg.APIUrl != "" {
		client.SetBaseURL(cfg.APIUrl)
	}
	if cfg.RefreshToken != "" {
		client.SetRefreshToken(cfg.RefreshToken)
		client.OnTokenRefresh = func(accessToken, refreshToken string) {
			cfg.AccessToken = accessToken
			cfg.RefreshToken = refreshToken
			config.Save(cfg)
		}
	}
	return client, nil
}

var flagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "Manage feature flags",
}

var flagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flags in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err)
		}

		if len(flags) == 0 {
			fmt.Println(ui.Info("No flags found."))
			return nil
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(flags)
		}

		rows := make([][]string, len(flags))
		for i, f := range flags {
			envSummary := ""
			for _, e := range f.Environments {
				status := ui.Dim("off")
				if e.Enabled {
					status = ui.Green("on")
				}
				if envSummary != "" {
					envSummary += ", "
				}
				envSummary += e.EnvironmentKey + ":" + status
				if len(e.Variants) > 0 {
					envSummary += fmt.Sprintf(" (%dv)", len(e.Variants))
				}
			}
			rows[i] = []string{f.Key, f.Name, ui.Dim(f.Type), envSummary}
		}
		fmt.Println(ui.Table([]string{"Key", "Name", "Type", "Environments"}, rows))
		return nil
	},
}

var flagsCreateCmd = &cobra.Command{
	Use:   "create [key]",
	Short: "Create a new feature flag",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing flag key. Usage: flagify flags create <key>")
		}
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		key := args[0]
		if !kebabCaseRe.MatchString(key) {
			suggestion := toKebabCase(key)
			return fmt.Errorf("flag key %q is not valid kebab-case. Try: %s", key, suggestion)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		projectName := cfg.Project
		if projectName == "" {
			projectName = project
		}
		flagType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Create flag %s in project %s?", ui.Bold(key), ui.Cyan(projectName)), yes)
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

		body := map[string]any{
			"key":  key,
			"name": key,
			"type": flagType,
		}
		if description != "" {
			body["description"] = description
		}

		switch flagType {
		case "boolean":
			body["defaultValue"] = true
		case "string":
			body["defaultValue"] = ""
		case "number":
			body["defaultValue"] = 0
		case "json":
			body["defaultValue"] = map[string]any{}
		}

		flag, err := client.CreateFlag(project, body)
		if err != nil {
			return handleAccessError(err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Created flag %s %s with %d environments",
			ui.Bold(flag.Key), ui.Dim("("+flag.Type+")"), len(flag.Environments))))
		return nil
	},
}

var flagsToggleCmd = &cobra.Command{
	Use:   "toggle [key]",
	Short: "Toggle a boolean flag on/off",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		all, _ := cmd.Flags().GetBool("all")
		env := resolveFlag(cmd, "environment", cfg.Environment)
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}
		if !all && env == "" {
			env = "development"
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err)
		}

		var targetFlag *api.Flag
		if len(args) == 0 {
			picked, err := picker.PickFlag(flags, "")
			if err != nil {
				return err
			}
			targetFlag = picked
		} else {
			key := args[0]
			for i, f := range flags {
				if f.Key == key {
					targetFlag = &flags[i]
					break
				}
			}
			if targetFlag == nil {
				return fmt.Errorf("flag %q not found in project", key)
			}
		}

		key := targetFlag.Key

		if all {
			if len(targetFlag.Environments) == 0 {
				return fmt.Errorf("no environments found for flag %q", key)
			}

			// Determine new state from first environment
			newState := !targetFlag.Environments[0].Enabled
			newStateStr := "OFF"
			if newState {
				newStateStr = "ON"
			}

			yes, _ := cmd.Flags().GetBool("yes")
			envNames := make([]string, len(targetFlag.Environments))
			for i, fe := range targetFlag.Environments {
				envNames[i] = fe.EnvironmentKey
			}
			confirmed, err := ui.Confirm(
				fmt.Sprintf("Toggle %s to %s in all environments (%s)?", ui.Bold(key), newStateStr, fmt.Sprintf("%v", envNames)),
				yes,
			)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println(ui.Info("Cancelled."))
				return nil
			}

			for _, fe := range targetFlag.Environments {
				if err := client.ToggleFlagByKey(project, key, fe.EnvironmentKey, newState); err != nil {
					return handleAccessError(err)
				}
				state := ui.Red("OFF")
				if newState {
					state = ui.Green("ON")
				}
				fmt.Println(ui.Success(fmt.Sprintf("Flag %s is now %s in %s", ui.Bold(key), state, ui.Cyan(fe.EnvironmentKey))))
			}
			return nil
		}

		var targetFE *api.FlagEnv
		for i, fe := range targetFlag.Environments {
			if fe.EnvironmentKey == env {
				targetFE = &targetFlag.Environments[i]
				break
			}
		}
		if targetFE == nil {
			return fmt.Errorf("environment %q not found for flag %q", env, key)
		}

		newState := !targetFE.Enabled
		newStateStr := "OFF"
		if newState {
			newStateStr = "ON"
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Toggle %s to %s in %s?", ui.Bold(key), newStateStr, ui.Cyan(env)), yes)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}

		if err := client.ToggleFlagByKey(project, key, env, newState); err != nil {
			return handleAccessError(err)
		}

		state := ui.Red("OFF")
		if newState {
			state = ui.Green("ON")
		}
		fmt.Println(ui.Success(fmt.Sprintf("Flag %s is now %s in %s", ui.Bold(key), state, ui.Cyan(env))))
		return nil
	},
}

var flagsGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get details for a specific flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err)
		}

		var flag *api.Flag
		for i, f := range flags {
			if f.Key == key {
				flag = &flags[i]
				break
			}
		}
		if flag == nil {
			return fmt.Errorf("flag %q not found in project", key)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(flag)
		}

		fmt.Println(ui.KeyValue("Key", ui.Bold(flag.Key)))
		fmt.Println(ui.KeyValue("Name", flag.Name))
		fmt.Println(ui.KeyValue("Type", ui.Dim(flag.Type)))
		fmt.Println()
		if len(flag.Environments) > 0 {
			rows := make([][]string, len(flag.Environments))
			for i, e := range flag.Environments {
				status := ui.Red("OFF")
				if e.Enabled {
					status = ui.Green("ON")
				}
				variants := "-"
				if len(e.Variants) > 0 {
					variants = fmt.Sprintf("%d variants", len(e.Variants))
				}
				rows[i] = []string{e.EnvironmentKey, status, variants}
			}
			fmt.Println(ui.Table([]string{"Environment", "Status", "Variants"}, rows))
		}
		return nil
	},
}

var flagsHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Detect configuration issues across flags",
	Long: `Detect configuration issues across all flags in the project:

  • env_mismatch                — flag on in prod but off in the preceding env,
                                  or value drift between prod and pre-prod.
  • rule_value_matches_default  — targeting rule valueOverride equals the flag's
                                  defaultValue, making the rule a no-op.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		issues, err := client.GetFlagHealth(project)
		if err != nil {
			return handleAccessError(err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(issues)
		}

		if len(issues) == 0 {
			fmt.Println(ui.Success("No configuration issues detected."))
			return nil
		}

		rows := make([][]string, len(issues))
		for i, issue := range issues {
			sev := issue.Severity
			switch issue.Severity {
			case "critical":
				sev = ui.Red(issue.Severity)
			case "warning":
				sev = ui.Warn(issue.Severity)
			}
			env := issue.Environment
			if env == "" {
				env = ui.Dim("—")
			}
			rows[i] = []string{issue.FlagKey, env, sev, issue.Type, issue.Message}
		}
		fmt.Println(ui.Table([]string{"Flag", "Environment", "Severity", "Type", "Message"}, rows))

		hasFixHints := false
		for _, issue := range issues {
			if issue.Fix != "" {
				hasFixHints = true
				break
			}
		}
		footer := fmt.Sprintf("%d issue(s) detected.", len(issues))
		if hasFixHints {
			footer += " Fix hints available in JSON output (--format json)."
		} else {
			footer += " Use --format json for the full payload."
		}
		fmt.Printf("\n%s\n", ui.Dim(footer))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(flagsListCmd)
	ui.AddFormatFlag(flagsGetCmd)
	ui.AddFormatFlag(flagsHealthCmd)
	flagsCreateCmd.Flags().StringP("type", "t", "boolean", "Flag type (boolean, string, number, json)")
	flagsCreateCmd.Flags().String("description", "", "Flag description")
	flagsToggleCmd.Flags().BoolP("all", "a", false, "Toggle in all environments")

	flagsCmd.AddCommand(flagsListCmd)
	flagsCmd.AddCommand(flagsGetCmd)
	flagsCmd.AddCommand(flagsCreateCmd)
	flagsCmd.AddCommand(flagsToggleCmd)
	flagsCmd.AddCommand(flagsHealthCmd)
	rootCmd.AddCommand(flagsCmd)
}
