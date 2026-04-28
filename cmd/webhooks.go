package cmd

import (
	"fmt"
	"strings"

	"github.com/flagifyhq/cli/internal/api"
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
	Short: "List webhooks in a project (optionally filtered by environment)",
	Long: `List webhooks in the active project.

By default this returns the project-aggregate view across every
environment. Pass --environment (or set it via 'flagify config set
environment ...') to restrict the result to a single environment, e.g.
when reviewing only the production hooks.`,
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

		webhooks, err := client.ListWebhooks(project, rc.Environment)
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(webhooks)
		}

		if len(webhooks) == 0 {
			if rc.Environment != "" {
				fmt.Println(ui.Info(fmt.Sprintf("No webhooks found in %s.", ui.Cyan(rc.Environment))))
			} else {
				fmt.Println(ui.Info("No webhooks found."))
			}
			return nil
		}

		// Show the env column only on the aggregate view; the env-filtered
		// view collapses it to keep the table narrow on small terminals.
		if rc.Environment == "" {
			// One extra GET per command invocation maps env ULIDs back
			// to human names so users see "production" instead of a
			// raw ULID in the table.
			envNames := loadEnvNames(client, project)
			rows := make([][]string, len(webhooks))
			for i, wh := range webhooks {
				rows[i] = []string{
					wh.Name,
					wh.URL,
					formatEvents(wh.Events),
					ui.Dim(webhookStatus(wh)),
					ui.Dim(resolveEnvName(envNames, wh.EnvironmentID)),
					ui.Dim(wh.ID),
				}
			}
			fmt.Println(ui.Table([]string{"Name", "URL", "Events", "Status", "Environment", "ID"}, rows))
			return nil
		}

		rows := make([][]string, len(webhooks))
		for i, wh := range webhooks {
			rows[i] = []string{wh.Name, wh.URL, formatEvents(wh.Events), ui.Dim(webhookStatus(wh)), ui.Dim(wh.ID)}
		}
		fmt.Println(ui.Table([]string{"Name", "URL", "Events", "Status", "ID"}, rows))
		return nil
	},
}

func webhookStatus(wh api.Webhook) string {
	if wh.DisabledAt != nil {
		return "auto-disabled"
	}
	if !wh.Active {
		return "paused"
	}
	return "active"
}

func formatEvents(events []string) string {
	if len(events) == 0 {
		return "all"
	}
	return strings.Join(events, ", ")
}

// resolveEnvName returns the human-readable env name for a ULID, with
// the ULID itself as a fallback when the lookup fails. Used by every
// webhook read path so users see "production" instead of
// "01HXYZ..." in tables and key/value output. The map is built once
// per command invocation by `loadEnvNames`.
func resolveEnvName(envs map[string]string, id string) string {
	if name, ok := envs[id]; ok {
		return name
	}
	return id
}

// loadEnvNames fetches the project's environments and returns a
// id→name map. A failure is non-fatal: the caller falls back to
// rendering raw ULIDs, which is uglier but still correct.
func loadEnvNames(client *api.Client, projectID string) map[string]string {
	out := map[string]string{}
	envs, err := client.ListEnvironments(projectID)
	if err != nil {
		return out
	}
	for _, e := range envs {
		out[e.ID] = e.Name
	}
	return out
}

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new webhook in an environment",
	Long: `Create a new webhook subscribed to one or more events. Webhooks
are environment-scoped: each subscription targets a single environment
(e.g. production), so a project can ship distinct hooks for dev, staging,
and prod without cross-talk.

The signing secret is printed exactly once. Save it in an environment
variable (e.g. FLAGIFY_WEBHOOK_SECRET) on the receiver — Flagify cannot
recover it later.

Example:
  flagify webhooks create \
    --environment production \
    --name "Slack #releases" \
    --url https://hooks.slack.com/services/... \
    --events flag.created,flag.toggled,flag.archived`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := resolveContext(cmd)
		if err != nil {
			return err
		}
		project := rc.ProjectIdentifier()
		env := rc.Environment
		if project == "" {
			return fmt.Errorf("--project is required (or run 'flagify projects pick' / 'flagify init')")
		}
		if env == "" {
			return fmt.Errorf("--environment is required (or run 'flagify environments pick')")
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

		wh, err := client.CreateWebhook(project, env, map[string]any{
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

		fmt.Println(ui.Success(fmt.Sprintf("Created webhook %s in %s", ui.Cyan(wh.Name), ui.Cyan(env))))
		fmt.Println()
		fmt.Println(ui.KeyValue("ID:", wh.ID))
		fmt.Println(ui.KeyValue("URL:", wh.URL))
		fmt.Println(ui.KeyValue("Environment:", wh.EnvironmentID))
		fmt.Println(ui.KeyValue("Events:", formatEvents(wh.Events)))
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

		envNames := loadEnvNames(client, project)
		fmt.Println(ui.KeyValue("Name:", wh.Name))
		fmt.Println(ui.KeyValue("URL:", wh.URL))
		fmt.Println(ui.KeyValue("Environment:", resolveEnvName(envNames, wh.EnvironmentID)))
		fmt.Println(ui.KeyValue("Events:", formatEvents(wh.Events)))
		fmt.Println(ui.KeyValue("Status:", webhookStatus(*wh)))
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

		// Pull the parent webhook so we can show which env this hook
		// is bound to in the deliveries header. Cheap (single GET) and
		// avoids forcing the user to cross-reference `webhooks get`.
		wh, whErr := client.GetWebhook(project, args[0])

		deliveries, err := client.ListWebhookDeliveries(project, args[0])
		if err != nil {
			return handleAccessError(err, rc)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(deliveries)
		}

		// Header line surfaces the env so a user troubleshooting "which
		// env's prod hook is failing?" can see at a glance.
		if whErr == nil && wh != nil {
			envNames := loadEnvNames(client, project)
			fmt.Println(ui.KeyValue("Webhook:", wh.Name))
			fmt.Println(ui.KeyValue("Environment:", resolveEnvName(envNames, wh.EnvironmentID)))
			fmt.Println()
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
