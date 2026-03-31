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

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
