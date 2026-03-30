package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

func resolveEnvironmentID(client *api.Client, projectID, envKey string) (string, error) {
	project, err := client.GetProject(projectID)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}
	for _, e := range project.Environments {
		if e.Key == envKey {
			return e.ID, nil
		}
	}
	return "", fmt.Errorf("environment %q not found in project", envKey)
}

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
		project := resolveFlag(cmd, "project", cfg.Project)
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

		envID, err := resolveEnvironmentID(client, project, env)
		if err != nil {
			return err
		}

		keys, err := client.GenerateKeys(envID)
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
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
		project := resolveFlag(cmd, "project", cfg.Project)
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

		envID, err := resolveEnvironmentID(client, project, env)
		if err != nil {
			return err
		}

		keys, err := client.ListKeys(envID)
		if err != nil {
			return fmt.Errorf("failed to list keys: %w", err)
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
		project := resolveFlag(cmd, "project", cfg.Project)
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

		envID, err := resolveEnvironmentID(client, project, env)
		if err != nil {
			return err
		}

		if err := client.RevokeKeys(envID); err != nil {
			return fmt.Errorf("failed to revoke keys: %w", err)
		}

		fmt.Println(ui.Success(fmt.Sprintf("Revoked all API keys for %s", ui.Cyan(env))))
		return nil
	},
}

func init() {
	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}
