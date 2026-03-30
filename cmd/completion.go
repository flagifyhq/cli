package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate an autocompletion script for your shell.

  # Bash
  flagify completion bash > /usr/local/etc/bash_completion.d/flagify

  # Zsh
  flagify completion zsh > "${fpath[1]}/_flagify"

  # Fish
  flagify completion fish > ~/.config/fish/completions/flagify.fish

  # PowerShell
  flagify completion powershell > flagify.ps1
`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing shell name. Usage: flagify completion <bash|zsh|fish|powershell>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
