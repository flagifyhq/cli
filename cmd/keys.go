package cmd

import (
	"fmt"
	"strings"

	"github.com/flagifyhq/cli/internal/api"
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

		yes, _ := cmd.Flags().GetBool("yes")
		confirmed, err := ui.Confirm(fmt.Sprintf("Generate API keys for %s?", ui.Cyan(env)), yes)
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

		keys, err := client.GenerateKeysByEnv(project, env)
		if err != nil {
			return handleAccessError(err, rc)
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

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		keys, err := client.ListKeysByEnv(project, env)
		if err != nil {
			return handleAccessError(err, rc)
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
	Use:   "revoke [prefix]",
	Short: "Revoke a single API key by prefix (or --id), or all keys with --all",
	Long: `Revoke an API key. Pass the key prefix (shown in 'keys list') as a positional argument,
or use --id to target a specific key by ULID when prefixes collide. Use --all to revoke
every active key in the environment.`,
	Args: cobra.MaximumNArgs(1),
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

		all, _ := cmd.Flags().GetBool("all")
		idFlag, _ := cmd.Flags().GetString("id")
		yes, _ := cmd.Flags().GetBool("yes")
		hasPrefix := len(args) > 0

		selectors := 0
		if hasPrefix {
			selectors++
		}
		if idFlag != "" {
			selectors++
		}
		if all {
			selectors++
		}
		switch selectors {
		case 0:
			return fmt.Errorf("provide a key prefix (see 'flagify keys list'), --id <ulid>, or --all to revoke every key in %s", env)
		case 1:
			// ok
		default:
			return fmt.Errorf("--all, --id, and a positional prefix are mutually exclusive")
		}

		client, err := getClientFromResolved(rc)
		if err != nil {
			return err
		}

		if all {
			confirmed, err := ui.Confirm(fmt.Sprintf("Revoke ALL API keys for %s? This cannot be undone.", ui.Cyan(env)), yes)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println(ui.Info("Cancelled."))
				return nil
			}
			if err := client.RevokeKeysByEnv(project, env); err != nil {
				return handleAccessError(err, rc)
			}
			fmt.Println(ui.Success(fmt.Sprintf("Revoked all API keys for %s", ui.Cyan(env))))
			return nil
		}

		var target *api.APIKey
		var label string

		if idFlag != "" {
			target = &api.APIKey{ID: strings.TrimSpace(idFlag)}
			label = ui.Cyan(target.ID)
		} else {
			prefix := strings.TrimSpace(args[0])
			keys, err := client.ListKeysByEnv(project, env)
			if err != nil {
				return handleAccessError(err, rc)
			}
			for i := range keys {
				if keys[i].Prefix == prefix && keys[i].RevokedAt == nil {
					target = &keys[i]
					break
				}
			}
			if target == nil {
				return fmt.Errorf("no active API key with prefix %q in %s", prefix, env)
			}
			label = ui.Cyan(target.Prefix)
		}

		typeLabel := ui.Dim(target.Type)
		if target.Type == "secret" {
			typeLabel = ui.Red(target.Type)
		}

		confirmed, err := ui.Confirm(fmt.Sprintf("Revoke %s key %s in %s? This cannot be undone.", typeLabel, label, ui.Cyan(env)), yes)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println(ui.Info("Cancelled."))
			return nil
		}

		if err := client.RevokeKeyByEnv(project, env, target.ID); err != nil {
			return handleAccessError(err, rc)
		}
		fmt.Println(ui.Success(fmt.Sprintf("Revoked API key %s in %s", label, ui.Cyan(env))))
		return nil
	},
}

func init() {
	ui.AddFormatFlag(keysListCmd)
	ui.AddFormatFlag(keysGenerateCmd)

	keysRevokeCmd.Flags().Bool("all", false, "Revoke every active API key in the environment")
	keysRevokeCmd.Flags().String("id", "", "Revoke the key with this ULID (use when prefixes collide)")

	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}
