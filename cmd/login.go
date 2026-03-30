package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()

		if cfg.IsLoggedIn() {
			fmt.Println("Already logged in. Use 'flagify logout' to sign out first.")
			return nil
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()
		password := string(passwordBytes)

		client := api.NewClient("")
		if cfg.APIUrl != "" {
			client.SetBaseURL(cfg.APIUrl)
		}

		hostname, _ := os.Hostname()
		deviceID := "cli-" + hostname

		result, err := client.Login(email, password, deviceID)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		cfg.AccessToken = result.Tokens.AccessToken
		cfg.RefreshToken = result.Tokens.RefreshToken
		cfg.Token = "" // clear legacy

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		name := ""
		if n, ok := result.User["name"].(string); ok {
			name = n
		}
		if name != "" {
			fmt.Printf("Logged in as %s (%s)\n", name, email)
		} else {
			fmt.Printf("Logged in as %s\n", email)
		}
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of Flagify",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		cfg.AccessToken = ""
		cfg.RefreshToken = ""
		cfg.Token = ""

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("Logged out.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
