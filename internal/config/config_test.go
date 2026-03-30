package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigSaveAndLoad(t *testing.T) {
	// Use temp dir for config
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	cfg := &config.Config{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		APIUrl:       "http://localhost:7070",
		Project:      "my-project",
	}

	err := config.Save(cfg)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(tmpDir, ".flagify", "config.json")
	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Load and verify
	loaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "test-access", loaded.AccessToken)
	assert.Equal(t, "test-refresh", loaded.RefreshToken)
	assert.Equal(t, "http://localhost:7070", loaded.APIUrl)
	assert.Equal(t, "my-project", loaded.Project)
}

func TestConfigLoadMissing(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.AccessToken)
}

func TestIsLoggedIn(t *testing.T) {
	cfg := &config.Config{}
	assert.False(t, cfg.IsLoggedIn())

	cfg.AccessToken = "some-token"
	assert.True(t, cfg.IsLoggedIn())
}

func TestGetToken(t *testing.T) {
	// New format
	cfg := &config.Config{AccessToken: "new-token"}
	assert.Equal(t, "new-token", cfg.GetToken())

	// Legacy format
	cfg = &config.Config{Token: "old-token"}
	assert.Equal(t, "old-token", cfg.GetToken())

	// New takes precedence
	cfg = &config.Config{AccessToken: "new", Token: "old"}
	assert.Equal(t, "new", cfg.GetToken())
}
