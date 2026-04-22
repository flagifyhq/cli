package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		user, err := client.GetMe()
		if err != nil {
			return fmt.Errorf("failed to get user info: %w", err)
		}

		if ui.IsJSON(cmd) {
			return ui.PrintJSON(user)
		}

		name := ""
		if user.Name != nil {
			name = *user.Name
		}

		if name != "" {
			fmt.Println(ui.Success(fmt.Sprintf("%s %s", ui.Bold(name), ui.Dim("("+user.Email+")"))))
		} else {
			fmt.Println(ui.Success(ui.Bold(user.Email)))
		}

		return nil
	},
}

// authWhoamiCmd mirrors whoamiCmd under `flagify auth whoami` so users who
// think in "auth" namespaces can discover it there too.
var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user (alias for 'flagify whoami')",
	RunE:  whoamiCmd.RunE,
}

func init() {
	ui.AddFormatFlag(whoamiCmd)
	ui.AddFormatFlag(authWhoamiCmd)
	rootCmd.AddCommand(whoamiCmd)
	authCmd.AddCommand(authWhoamiCmd)
}
