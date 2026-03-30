package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	cyan   = lipgloss.Color("#00D4FF")
	green  = lipgloss.Color("#00CC88")
	red    = lipgloss.Color("#FF6B6B")
	yellow = lipgloss.Color("#FFCC00")
	dim    = lipgloss.Color("#666666")

	boldStyle    = lipgloss.NewStyle().Bold(true)
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	successStyle = lipgloss.NewStyle().Foreground(green)
	dimStyle     = lipgloss.NewStyle().Foreground(dim)
	warnStyle    = lipgloss.NewStyle().Foreground(yellow)
	redStyle     = lipgloss.NewStyle().Foreground(red)
	cyanStyle    = lipgloss.NewStyle().Foreground(cyan)
)

func Bold(s string) string    { return boldStyle.Render(s) }
func Label(s string) string   { return labelStyle.Render(s) }
func Green(s string) string   { return successStyle.Render(s) }
func Dim(s string) string     { return dimStyle.Render(s) }
func Warn(s string) string    { return warnStyle.Render(s) }
func Red(s string) string     { return redStyle.Render(s) }
func Cyan(s string) string    { return cyanStyle.Render(s) }

func Success(msg string) string { return fmt.Sprintf("%s %s", successStyle.Render("✓"), msg) }
func Info(msg string) string    { return fmt.Sprintf("%s %s", dimStyle.Render("●"), msg) }
func Warning(msg string) string { return fmt.Sprintf("%s %s", warnStyle.Render("⚠"), msg) }
func Error(msg string) string   { return redStyle.Render("✗ " + msg) }
func Arrow() string             { return cyanStyle.Render("→") }

func Table(headers []string, rows [][]string) string {
	allRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		allRows = append(allRows, row)
	}

	t := table.New().
		Headers(headers...).
		Rows(allRows...).
		BorderStyle(lipgloss.NewStyle().Foreground(dim)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(cyan).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return t.Render()
}

func KeyValue(label, value string) string {
	return fmt.Sprintf("  %s  %s", labelStyle.Render(fmt.Sprintf("%-12s", label)), value)
}
