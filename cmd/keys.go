package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate API key pair for an environment",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("--environment is required (or run 'flagify environments pick')")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Generate API keys for %s?", ui.Cyan(env)), yes)
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

		keys, err := client.GenerateKeysByEnv(project, env)
		if err != nil {
			return handleAccessError(err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(map[string]any{
				"environment":    env,
				"publishableKey": keys.PublishableKey,
				"secretKey":      keys.SecretKey,
			})
		}

		fmt.Println(ui.Success(fmt.Sprintf("Generated API keys for %s", ui.Cyan(env))))
		fmt.Println()
		fmt.Println(ui.KeyValue("Publishable:", keys.PublishableKey))
		fmt.Println(ui.KeyValue("Secret:", keys.SecretKey))
		fmt.Println()
		fmt.Println(ui.Warning("Save these keys now — the secret key won't be shown again."))
		return nil
	},
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys for an environment",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("--environment is required (or run 'flagify environments pick')")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		keys, err := client.ListKeysByEnv(project, env)
		if err != nil {
			return handleAccessError(err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(keys)
		}

		if len(keys) == 0 {
			fmt.Println(ui.Info("No API keys found."))
			return nil
		}

		rows := make([][]string, len(keys))
		for i, k := range keys {
			status := ui.Green("active")
			if k.RevokedAt != nil {
				status = ui.Dim("revoked")
			}
			created := k.CreatedAt.Format("2006-01-02 15:04")
			rows[i] = []string{k.Prefix, ui.Dim(k.Type), created, status}
		}
		fmt.Println(ui.Table([]string{"Prefix", "Type", "Created", "Status"}, rows))
		return nil
	},
}

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke all API keys for an environment",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("--environment is required (or run 'flagify environments pick')")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Revoke ALL API keys for %s? This cannot be undone.", ui.Cyan(env)), yes)
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

		if err := client.RevokeKeysByEnv(project, env); err != nil {
			return handleAccessError(err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Revoked all API keys for %s", ui.Cyan(env))))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(keysListCmd)
	ui.AddFormatFlag(keysGenerateCmd)

	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}
