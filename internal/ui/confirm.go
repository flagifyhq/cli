package ui

import (
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

func Confirm(message string, skip bool) (bool, error) {
	if skip || !IsTTY() {
		return true, nil
	}
	var confirmed bool
	err := huh.NewConfirm().
		Title(message).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		Run()
	return confirmed, err
}

// IsTTY reports whether stdout is an interactive terminal. Used to decide
// whether a destructive action can prompt for confirmation or must fail loud.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
