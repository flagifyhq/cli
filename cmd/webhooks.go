package cmd

import (
	"fmt"
	"strings"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Manage outbound webhooks",
	Long: `Manage outbound webhooks for the active project.

Webhooks deliver signed HTTP POSTs to your URL when flags or targeting
rules change. The signing secret is returned exactly once on create —
save it; you can not retrieve it later.`,
}

var webhooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all webhooks in a project",
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

		webhooks, err := client.ListWebhooks(project)
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(webhooks)
		}

		if len(webhooks) == 0 {
			fmt.Println(ui.Info("No webhooks found."))
			return nil
		}

		rows := make([][]string, len(webhooks))
		for i, wh := range webhooks {
			status := "active"
			if !wh.Active {
				status = "paused"
			}
			if wh.DisabledAt != nil {
				status = "auto-disabled"
			}
			events := "all"
			if len(wh.Events) > 0 {
				events = strings.Join(wh.Events, ", ")
			}
			rows[i] = []string{wh.Name, wh.URL, events, ui.Dim(status), ui.Dim(wh.ID)}
		}
		fmt.Println(ui.Table([]string{"Name", "URL", "Events", "Status", "ID"}, rows))
		return nil
	},
}

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook",
	Long: `Create a new webhook subscribed to one or more events.

The signing secret is printed exactly once. Save it in an environment
variable (e.g. FLAGIFY_WEBHOOK_SECRET) on the receiver — Flagify cannot
recover it later.

Example:
  flagify webhooks create \
    --name "Slack #releases" \
    --url https://hooks.slack.com/services/... \
    --events flag.created,flag.toggled,flag.deleted`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}

		name, _ := cmd.Flags().GetString("name")
		url, _ := cmd.Flags().GetString("url")
		eventsRaw, _ := cmd.Flags().GetString("events")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if url == "" {
			return fmt.Errorf("--url is required")
		}

		events := []string{}
		if eventsRaw != "" {
			for _, e := range strings.Split(eventsRaw, ",") {
				if trimmed := strings.TrimSpace(e); trimmed != "" {
					events = append(events, trimmed)
				}
			}
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		wh, err := client.CreateWebhook(project, map[string]any{
			"name":   name,
			"url":    url,
			"events": events,
		})
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(wh)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Created webhook %s", ui.Cyan(wh.Name))))
		fmt.Println()
		fmt.Println(ui.KeyValue("ID:", wh.ID))
		fmt.Println(ui.KeyValue("URL:", wh.URL))
		if len(wh.Events) > 0 {
			fmt.Println(ui.KeyValue("Events:", strings.Join(wh.Events, ", ")))
		} else {
			fmt.Println(ui.KeyValue("Events:", "all"))
		}
		fmt.Println(ui.KeyValue("Secret:", wh.Secret))
		fmt.Println()
		fmt.Println(ui.Warning("Save the secret now — it won't be shown again."))
		return nil
	},
}

var webhooksGetCmd = &cobra.Command{
	Use:   "get <webhook-id>",
	Short: "Get a webhook by ID",
	Args:  cobra.ExactArgs(1),
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

		wh, err := client.GetWebhook(project, args[0])
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(wh)
		}

		fmt.Println(ui.KeyValue("Name:", wh.Name))
		fmt.Println(ui.KeyValue("URL:", wh.URL))
		if len(wh.Events) > 0 {
			fmt.Println(ui.KeyValue("Events:", strings.Join(wh.Events, ", ")))
		} else {
			fmt.Println(ui.KeyValue("Events:", "all"))
		}
		status := "active"
		if !wh.Active {
			status = "paused"
		}
		if wh.DisabledAt != nil {
			status = "auto-disabled"
		}
		fmt.Println(ui.KeyValue("Status:", status))
		fmt.Println(ui.KeyValue("ID:", wh.ID))
		return nil
	},
}

var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <webhook-id>",
	Short: "Delete a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Delete webhook %s? This cannot be undone.", ui.Cyan(args[0])), yes)
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

		if err := client.DeleteWebhook(project, args[0]); err != nil {
			return handleAccessError(err, rc)
		}

		fmt.Println(ui.Success("Webhook deleted."))
		return nil
	},
}

var webhooksDeliveriesCmd = &cobra.Command{
	Use:   "deliveries <webhook-id>",
	Short: "Show recent delivery attempts for a webhook",
	Args:  cobra.ExactArgs(1),
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

		deliveries, err := client.ListWebhookDeliveries(project, args[0])
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(deliveries)
		}

		if len(deliveries) == 0 {
			fmt.Println(ui.Info("No deliveries yet."))
			return nil
		}

		rows := make([][]string, len(deliveries))
		for i, d := range deliveries {
			code := "-"
			if d.ResponseCode != nil {
				code = fmt.Sprintf("%d", *d.ResponseCode)
			}
			rows[i] = []string{
				d.CreatedAt.Format("2006-01-02 15:04:05"),
				d.EventAction,
				ui.Dim(d.Status),
				fmt.Sprintf("%d", d.Attempt),
				code,
			}
		}
		fmt.Println(ui.Table([]string{"When", "Event", "Status", "Attempt", "HTTP"}, rows))
		return nil
	},
}

func init() {
	webhooksCreateCmd.Flags().String("name", "", "Webhook name (required)")
	webhooksCreateCmd.Flags().String("url", "", "Receiver URL (https) (required)")
	webhooksCreateCmd.Flags().String("events", "", "Comma-separated event list (empty = all events)")
	webhooksDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	webhooksCmd.AddCommand(webhooksListCmd)
	webhooksCmd.AddCommand(webhooksCreateCmd)
	webhooksCmd.AddCommand(webhooksGetCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
	webhooksCmd.AddCommand(webhooksDeliveriesCmd)
	rootCmd.AddCommand(webhooksCmd)
}
