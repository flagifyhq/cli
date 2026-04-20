package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flagifyhq/cli/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyData = templates.Data{}

func TestGenerateCursor(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	path, err := generateCursor(emptyData)
	require.NoError(t, err)
	assert.Equal(t, ".cursorrules", path)

	content, err := os.ReadFile(filepath.Join(dir, ".cursorrules"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Flagify Feature Flags")
	assert.Contains(t, string(content), "isEnabled(")
	assert.Contains(t, string(content), "@flagify/react")
	assert.Contains(t, string(content), "flagify types")
}

func TestGenerateWindsurf(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	path, err := generateWindsurf(emptyData)
	require.NoError(t, err)
	assert.Equal(t, ".windsurfrules", path)

	content, err := os.ReadFile(filepath.Join(dir, ".windsurfrules"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Flagify Feature Flags")
	assert.Contains(t, string(content), "useFlag")
	assert.Contains(t, string(content), "flagify types")
}

func TestGenerateCopilot(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	path, err := generateCopilot(emptyData)
	require.NoError(t, err)
	assert.Equal(t, ".github/copilot-instructions.md", path)

	content, err := os.ReadFile(filepath.Join(dir, ".github", "copilot-instructions.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Flagify Feature Flags")
	assert.Contains(t, string(content), "isEnabled")
	assert.Contains(t, string(content), "flagify types")
}

func TestGenerateClaude(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	path, err := generateClaude(emptyData)
	require.NoError(t, err)
	assert.Contains(t, path, "CLAUDE.md")

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Feature Flags")
	assert.Contains(t, string(content), "@flagify/node")
	assert.Contains(t, string(content), "flagify types")
	assert.Contains(t, string(content), "FLAG_KEYS")

	// Check slash commands were created
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-create.md"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-toggle.md"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-list.md"))
	require.NoError(t, err)
}

func TestGenerateClaudeAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	existing := "# My Project\n\nSome existing content.\n"
	os.WriteFile("CLAUDE.md", []byte(existing), 0644)

	_, err := generateClaude(emptyData)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "My Project")
	assert.Contains(t, string(content), "Feature Flags")
}

func TestGenerateWithFlagsContext(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	data := templates.Data{
		FlagsContext: "\n## Active flags\n\n| Flag | Type |\n|------|------|\n| `dark-mode` | boolean |\n",
	}

	_, err := generateCursor(data)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".cursorrules"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "dark-mode")
	assert.Contains(t, string(content), "Active flags")
}

func TestIsValidTool(t *testing.T) {
	assert.True(t, isValidTool("claude"))
	assert.True(t, isValidTool("cursor"))
	assert.True(t, isValidTool("copilot"))
	assert.True(t, isValidTool("windsurf"))
	assert.False(t, isValidTool("vscode"))
	assert.False(t, isValidTool(""))
}
