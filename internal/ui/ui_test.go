package ui_test

import (
	"testing"

	"github.com/flagifyhq/cli/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestSuccess(t *testing.T) {
	result := ui.Success("done")
	assert.Contains(t, result, "done")
	assert.Contains(t, result, "✓")
}

func TestInfo(t *testing.T) {
	result := ui.Info("hello")
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "●")
}

func TestWarning(t *testing.T) {
	result := ui.Warning("careful")
	assert.Contains(t, result, "careful")
	assert.Contains(t, result, "⚠")
}

func TestKeyValue(t *testing.T) {
	result := ui.KeyValue("Name:", "Flagify")
	assert.Contains(t, result, "Name:")
	assert.Contains(t, result, "Flagify")
}

func TestTableEmpty(t *testing.T) {
	result := ui.Table([]string{"ID", "Name"}, [][]string{})
	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Name")
}

func TestTableWithRows(t *testing.T) {
	rows := [][]string{
		{"1", "Alpha"},
		{"2", "Beta"},
	}
	result := ui.Table([]string{"ID", "Name"}, rows)
	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Alpha")
	assert.Contains(t, result, "Beta")
}
