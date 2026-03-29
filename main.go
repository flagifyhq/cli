package main

import (
	"os"

	"github.com/flagifyhq/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
