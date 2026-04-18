package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	ErrFlagNotFound = errors.New("flag not found")
	ErrEnvNotFound  = errors.New("environment not found for flag")
)

var targetingCmd = &cobra.Command{
	Use:   "targeting",
	Short: "Manage targeting rules for feature flags",
}

var targetingListCmd = &cobra.Command{
	Use:   "list <flag-key>",
	Short: "List targeting rules for a flag",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing flag key. Usage: flagify targeting list <flag-key>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		flagKey := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		env := resolveFlag(cmd, "environment", cfg.Environment)
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}
		if env == "" {
			env = "development"
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		feID, err := findFlagEnvID(client, project, flagKey, env)
		if err != nil {
			return decorateLookupError(err, flagKey, env)
		}

		rules, err := client.GetTargetingRules(feID)
		if err != nil {
			return fmt.Errorf("failed to get targeting rules: %w", err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(map[string]any{
				"flag":        flagKey,
				"environment": env,
				"rules":       rules,
			})
		}

		if len(rules) == 0 {
			fmt.Println(ui.Info(fmt.Sprintf("No targeting rules for %s in %s.",
				ui.Bold(flagKey), ui.Cyan(env))))
			return nil
		}

		rows := make([][]string, len(rules))
		for i, r := range rules {
			target := ""
			if r.SegmentID != nil {
				target = "segment:" + *r.SegmentID
			}
			if len(r.Conditions) > 0 {
				if target != "" {
					target += " + "
				}
				target += fmt.Sprintf("%d conditions", len(r.Conditions))
			}
			if target == "" {
				target = ui.Dim("catch-all")
			}

			rollout := ""
			if r.RolloutPercentage != nil {
				rollout = fmt.Sprintf("%d%%", *r.RolloutPercentage)
			}

			status := ui.Green("on")
			if !r.Enabled {
				status = ui.Dim("off")
			}

			override := ""
			if r.ValueOverride != nil {
				override = fmt.Sprintf("%v", r.ValueOverride)
			}

			rows[i] = []string{fmt.Sprintf("%d", r.Priority), target, override, rollout, status}
		}
		fmt.Println(ui.Table([]string{"#", "Target", "Value", "Rollout", "Status"}, rows))
		return nil
	},
}

var targetingSetCmd = &cobra.Command{
	Use:   "set <flag-key>",
	Short: "Set targeting rules for a flag (replaces all existing rules)",
	Long:  "Set targeting rules from a JSON string. Replaces all existing rules for the flag in the specified environment.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing flag key. Usage: flagify targeting set <flag-key> --rules '<json>'")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		flagKey := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		env := resolveFlag(cmd, "environment", cfg.Environment)
		rulesRaw, _ := cmd.Flags().GetString("rules")
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}
		if env == "" {
			env = "development"
		}
		if rulesRaw == "" {
			return fmt.Errorf("--rules is required. Pass a JSON array of rules")
		}

		var rules []map[string]any
		if err := json.Unmarshal([]byte(rulesRaw), &rules); err != nil {
			return fmt.Errorf("invalid --rules JSON: %w", err)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(
			fmt.Sprintf("Replace all targeting rules for %s in %s with %d rules?",
				ui.Bold(flagKey), ui.Cyan(env), len(rules)),
			yes,
		)
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

		feID, err := findFlagEnvID(client, project, flagKey, env)
		if err != nil {
			return decorateLookupError(err, flagKey, env)
		}

		result, err := client.SetTargetingRules(feID, map[string]any{"rules": rules})
		if err != nil {
			return fmt.Errorf("failed to set targeting rules: %w", err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(map[string]any{
				"flag":        flagKey,
				"environment": env,
				"rules":       result,
			})
		}

		fmt.Println(ui.Success(fmt.Sprintf("Set %d targeting rules for %s in %s",
			len(result), ui.Bold(flagKey), ui.Cyan(env))))
		return nil
	},
}

// decorateLookupError appends a "what to run next" hint when the lookup
// failed for a known reason. The original error is still wrapped so
// `errors.Is(err, ErrFlagNotFound)` keeps working in scripts that branch
// on the failure mode.
func decorateLookupError(err error, flagKey, envKey string) error {
	switch {
	case errors.Is(err, ErrFlagNotFound):
		return fmt.Errorf("%w. Run `flagify flags list` to see available flag keys", err)
	case errors.Is(err, ErrEnvNotFound):
		return fmt.Errorf("%w. Run `flagify projects get` to see environments configured for this project", err)
	default:
		return err
	}
}

func findFlagEnvID(client *api.Client, projectID, flagKey, envKey string) (string, error) {
	flags, err := client.ListFlags(projectID)
	if err != nil {
		return "", fmt.Errorf("failed to load flags: %w", err)
	}

	for _, f := range flags {
		if f.Key == flagKey {
			for _, fe := range f.Environments {
				if fe.EnvironmentKey == envKey {
					return fe.ID, nil
				}
			}
			return "", fmt.Errorf("environment %q not configured for flag %q: %w", envKey, flagKey, ErrEnvNotFound)
		}
	}
	return "", fmt.Errorf("flag %q not found in project: %w", flagKey, ErrFlagNotFound)
}

func init() {
	targetingSetCmd.Flags().String("rules", "", `Rules as JSON array (required)`)
	ui.AddFormatFlag(targetingListCmd)
	ui.AddFormatFlag(targetingSetCmd)

	targetingCmd.AddCommand(targetingListCmd)
	targetingCmd.AddCommand(targetingSetCmd)
	rootCmd.AddCommand(targetingCmd)
}
