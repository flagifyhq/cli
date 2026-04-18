package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var segmentsCmd = &cobra.Command{
	Use:   "segments",
	Short: "Manage user segments",
}

var segmentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all segments in a project",
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

		segments, err := client.ListSegments(project)
		if err != nil {
			return fmt.Errorf("failed to list segments: %w", err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(segments)
		}

		if len(segments) == 0 {
			fmt.Println(ui.Info("No segments found."))
			return nil
		}

		rows := make([][]string, len(segments))
		for i, s := range segments {
			rulesSummary := fmt.Sprintf("%d rules", len(s.Rules))
			rows[i] = []string{s.Name, ui.Dim(s.MatchType), rulesSummary, ui.Dim(s.ID)}
		}
		fmt.Println(ui.Table([]string{"Name", "Match", "Rules", "ID"}, rows))
		return nil
	},
}

var segmentsCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new segment",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing segment name. Usage: flagify segments create <name>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		project := resolveFlag(cmd, "project", cfg.ProjectID)
		matchType, _ := cmd.Flags().GetString("match")
		rulesRaw, _ := cmd.Flags().GetString("rules")
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick')")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Create segment %s (%s)?", ui.Bold(name), matchType), yes)
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
			"name":      name,
			"matchType": matchType,
		}

		if rulesRaw != "" {
			var rules []map[string]any
			if err := json.Unmarshal([]byte(rulesRaw), &rules); err != nil {
				return fmt.Errorf("invalid --rules JSON: %w", err)
			}
			body["rules"] = rules
		}

		seg, err := client.CreateSegment(project, body)
		if err != nil {
			return fmt.Errorf("failed to create segment: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Created segment %s %s with %d rules",
			ui.Bold(seg.Name), ui.Dim("("+seg.MatchType+")"), len(seg.Rules))))
		return nil
	},
}

var segmentsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a segment",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing segment ID. Usage: flagify segments delete <id>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		segID := args[0]

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Delete segment %s? This cannot be undone.", ui.Bold(segID)), yes)
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

		if err := client.DeleteSegment(segID); err != nil {
			return fmt.Errorf("failed to delete segment: %w", err)
		}

		fmt.Println(ui.Success("Deleted segment " + segID))
		return nil
	},
}

func init() {
	segmentsCreateCmd.Flags().String("match", "ALL", "Match type (ALL or ANY)")
	segmentsCreateCmd.Flags().String("rules", "", `Rules as JSON array, e.g. '[{"attribute":"plan","operator":"equals","value":"pro"}]'`)
	ui.AddFormatFlag(segmentsListCmd)

	segmentsCmd.AddCommand(segmentsListCmd)
	segmentsCmd.AddCommand(segmentsCreateCmd)
	segmentsCmd.AddCommand(segmentsDeleteCmd)
	rootCmd.AddCommand(segmentsCmd)
}
