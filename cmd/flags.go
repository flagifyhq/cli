package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/spf13/cobra"
)

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
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			return fmt.Errorf("--project is required")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		flags, err := client.ListFlags(project)
		if err != nil {
			return fmt.Errorf("failed to list flags: %w", err)
		}

		if len(flags) == 0 {
			fmt.Println("No flags found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "KEY\tNAME\tTYPE\tENVIRONMENTS")
		for _, f := range flags {
			envSummary := ""
			for _, e := range f.Environments {
				status := "off"
				if e.Enabled {
					status = "on"
				}
				if envSummary != "" {
					envSummary += ", "
				}
				envSummary += e.EnvironmentKey + ":" + status
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Key, f.Name, f.Type, envSummary)
		}
		w.Flush()
		return nil
	},
}

var flagsCreateCmd = &cobra.Command{
	Use:   "create [key]",
	Short: "Create a new feature flag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		project, _ := cmd.Flags().GetString("project")
		flagType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		if project == "" {
			return fmt.Errorf("--project is required")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		body := map[string]any{
			"key":  key,
			"name": key, // default name = key
			"type": flagType,
		}
		if description != "" {
			body["description"] = description
		}

		// Set default value based on type
		switch flagType {
		case "boolean":
			body["defaultValue"] = true
		case "string":
			body["defaultValue"] = ""
		case "number":
			body["defaultValue"] = json.Number("0")
		case "json":
			body["defaultValue"] = map[string]any{}
		}

		flag, err := client.CreateFlag(project, body)
		if err != nil {
			return fmt.Errorf("failed to create flag: %w", err)
		}

		fmt.Printf("Created flag %q (%s) with %d environments\n", flag.Key, flag.Type, len(flag.Environments))
		return nil
	},
}

var flagsToggleCmd = &cobra.Command{
	Use:   "toggle [key]",
	Short: "Toggle a boolean flag on/off",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		project, _ := cmd.Flags().GetString("project")
		env, _ := cmd.Flags().GetString("environment")
		if project == "" {
			return fmt.Errorf("--project is required")
		}
		if env == "" {
			env = "development"
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		// List flags to find the one we want
		flags, err := client.ListFlags(project)
		if err != nil {
			return fmt.Errorf("failed to list flags: %w", err)
		}

		var targetFlag *api.Flag
		for i, f := range flags {
			if f.Key == key {
				targetFlag = &flags[i]
				break
			}
		}
		if targetFlag == nil {
			return fmt.Errorf("flag %q not found in project", key)
		}

		// Find the flag environment
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

		// Toggle
		newState := !targetFE.Enabled
		if err := client.ToggleFlag(targetFE.ID, newState); err != nil {
			return fmt.Errorf("failed to toggle flag: %w", err)
		}

		state := "OFF"
		if newState {
			state = "ON"
		}
		fmt.Printf("Flag %q is now %s in %s\n", key, state, env)
		return nil
	},
}

func init() {
	flagsCreateCmd.Flags().StringP("type", "t", "boolean", "Flag type (boolean, string, number, json)")
	flagsCreateCmd.Flags().String("description", "", "Flag description")

	flagsCmd.AddCommand(flagsListCmd)
	flagsCmd.AddCommand(flagsCreateCmd)
	flagsCmd.AddCommand(flagsToggleCmd)
	rootCmd.AddCommand(flagsCmd)
}
