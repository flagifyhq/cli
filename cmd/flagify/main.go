package main

import (
	"fmt"
	"os"

	"github.com/flagifyhq/cli/cmd"
	"github.com/flagifyhq/cli/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
}
