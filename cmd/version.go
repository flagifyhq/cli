package cmd

import (
	"fmt"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s %s\n", ui.Bold("flagify"), ui.Cyan(Version), ui.Dim("("+Commit+")"))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
