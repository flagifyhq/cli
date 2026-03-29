package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Opening browser for authentication...")
		// TODO: OAuth flow
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
