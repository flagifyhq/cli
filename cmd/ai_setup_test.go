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
	// Webhook section landed and explains env scope + signature path.
	assert.Contains(t, string(content), "flagify webhooks")
	assert.Contains(t, string(content), "verifyFlagifySignature")
	assert.Contains(t, string(content), "environmentId")
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
	assert.Contains(t, string(content), "flagify webhooks")
	assert.Contains(t, string(content), "verifyFlagifySignature")
	assert.Contains(t, string(content), "environmentId")
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
	assert.Contains(t, string(content), "flagify webhooks")
	assert.Contains(t, string(content), "verifyFlagifySignature")
	assert.Contains(t, string(content), "environmentId")
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
	assert.Contains(t, string(content), "flagify webhooks list")
	assert.Contains(t, string(content), "verifyFlagifySignature")
	assert.Contains(t, string(content), "environmentId")
	assert.Contains(t, string(content), "FLAGIFY_WEBHOOK_SECRET")
	assert.Contains(t, string(content), "/flagify")

	skillContent, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "flagify", "SKILL.md"))
	require.NoError(t, err)
	skill := string(skillContent)
	assert.Contains(t, skill, "name: flagify")
	assert.Contains(t, skill, "flagify flags")
	assert.Contains(t, skill, "flagify targeting")
	assert.Contains(t, skill, "flagify types")
	assert.Contains(t, skill, "flagify keys")
	assert.Contains(t, skill, "flagify webhooks")

	// Old commands must NOT be created
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-create.md"))
	assert.Error(t, err)
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-toggle.md"))
	assert.Error(t, err)
	_, err = os.ReadFile(filepath.Join(dir, ".claude", "commands", "flagify-list.md"))
	assert.Error(t, err)
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

	_, err = os.ReadFile(filepath.Join(dir, ".claude", "skills", "flagify", "SKILL.md"))
	require.NoError(t, err)
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

func TestGenerateWithFlagsContextClaude(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	data := templates.Data{
		FlagsContext: "\n## Active flags\n\n| Flag | Type |\n|------|------|\n| `dark-mode` | boolean |\n",
	}

	_, err := generateClaude(data)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "flagify", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "dark-mode")
	assert.Contains(t, string(content), "Active flags")
	assert.Contains(t, string(content), "may be outdated")
}

func TestSkillYAMLFrontmatter(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	_, err := generateClaude(emptyData)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "flagify", "SKILL.md"))
	require.NoError(t, err)
	skill := string(content)
	assert.Contains(t, skill, "name: flagify")
	assert.Contains(t, skill, "description:")
	assert.Contains(t, skill, "argument-hint:")
}

func TestIsValidTool(t *testing.T) {
	assert.True(t, isValidTool("claude"))
	assert.True(t, isValidTool("cursor"))
	assert.True(t, isValidTool("copilot"))
	assert.True(t, isValidTool("windsurf"))
	assert.False(t, isValidTool("vscode"))
	assert.False(t, isValidTool(""))
}
