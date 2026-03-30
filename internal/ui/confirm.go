package ui

import "github.com/charmbracelet/huh"

func Confirm(message string, skip bool) (bool, error) {
	if skip {
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
