package ui

import (
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

func Confirm(message string, skip bool) (bool, error) {
	if skip || !term.IsTerminal(int(os.Stdout.Fd())) {
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
