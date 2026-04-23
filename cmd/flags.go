package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/flagifyhq/cli/internal/api"
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

var flagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "Manage feature flags",
}

var flagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flags in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err, rc)
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
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}
		projectLabel := rc.Project
		if projectLabel == "" {
			projectLabel = project
		}
		flagType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Create flag %s in project %s?", ui.Bold(key), ui.Cyan(projectLabel)), yes)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}

		client, err := getClientFromResolved(rc)
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
			return handleAccessError(err, rc)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Created flag %s %s with %d environments",
			ui.Bold(flag.Key), ui.Dim("("+flag.Type+")"), len(flag.Environments))))
		return nil
	},
}

var flagsToggleCmd = &cobra.Command{
	Use:   "toggle [key]",
	Short: "Toggle a boolean flag on/off",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		all, _ := cmd.Flags().GetBool("all")
		env := rc.Environment
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}
		if !all && env == "" {
			env = "development"
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err, rc)
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
					return handleAccessError(err, rc)
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
			return handleAccessError(err, rc)
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
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return handleAccessError(err, rc)
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
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		issues, err := client.GetFlagHealth(project)
		if err != nil {
			return handleAccessError(err, rc)
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
