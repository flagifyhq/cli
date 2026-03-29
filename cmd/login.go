package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Opening browser for authentication..., bitch")
		cfg, _ := config.Load()

		cfg.Token = "thisisatesttoken"

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		// TODO: OAuth flow
		fmt.Println("Successfully logged in! (this is a placeholder, implement OAuth flow")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
