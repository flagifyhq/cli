package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Shim-compatible tests (preserved from v1) -----------------------------

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &config.Config{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		APIUrl:       "http://localhost:7070",
		Workspace:    "ws-123",
		Project:      "my-project",
		Environment:  "staging",
	}

	err := config.Save(cfg)
	require.NoError(t, err)

	path := filepath.Join(tmpDir, ".flagify", "config.json")
	_, err = os.Stat(path)
	assert.NoError(t, err)

	loaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "test-access", loaded.AccessToken)
	assert.Equal(t, "test-refresh", loaded.RefreshToken)
	assert.Equal(t, "http://localhost:7070", loaded.APIUrl)
	assert.Equal(t, "ws-123", loaded.Workspace)
	assert.Equal(t, "my-project", loaded.Project)
	assert.Equal(t, "staging", loaded.Environment)
}

func TestConfigLoadMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

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
	cfg := &config.Config{AccessToken: "new-token"}
	assert.Equal(t, "new-token", cfg.GetToken())

	cfg = &config.Config{Token: "old-token"}
	assert.Equal(t, "old-token", cfg.GetToken())

	cfg = &config.Config{AccessToken: "new", Token: "old"}
	assert.Equal(t, "new", cfg.GetToken())
}

func TestPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path, err := config.Path()
	require.NoError(t, err)
	assert.Contains(t, path, ".flagify")
	assert.Contains(t, path, "config.json")
}

func TestConfigJSONCamelCase(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	data := `{"accessToken":"tk","refreshToken":"rt","apiUrl":"http://localhost:7070","project":"proj"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(data), 0o600))

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "tk", cfg.AccessToken)
	assert.Equal(t, "rt", cfg.RefreshToken)
	assert.Equal(t, "http://localhost:7070", cfg.APIUrl)
	assert.Equal(t, "proj", cfg.Project)
}

// --- v2 store tests --------------------------------------------------------

func TestStoreRoundTripV2(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	in := &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken:  "wt",
				RefreshToken: "wr",
				APIUrl:       "https://api.flagify.dev",
				User:         &config.UserInfo{ID: "u1", Email: "mario@acme.com", Name: "Mario"},
				Defaults: config.Defaults{
					Workspace:   "acme",
					WorkspaceID: "ws_1",
					Project:     "api",
					ProjectID:   "pr_1",
					Environment: "development",
				},
			},
			"personal": {
				AccessToken:  "pt",
				RefreshToken: "pr",
				APIUrl:       "https://api.flagify.dev",
			},
		},
		Bindings: map[string]config.Binding{
			"/Users/mario/dev/acme-api": {Profile: "work"},
		},
	}
	require.NoError(t, config.SaveStore(in))

	out, err := config.LoadStore()
	require.NoError(t, err)
	assert.Equal(t, config.StoreVersion, out.Version)
	assert.Equal(t, "work", out.Current)
	require.Contains(t, out.Accounts, "work")
	require.Contains(t, out.Accounts, "personal")
	assert.Equal(t, "wt", out.Accounts["work"].AccessToken)
	assert.Equal(t, "pr_1", out.Accounts["work"].Defaults.ProjectID)
	assert.Equal(t, "mario@acme.com", out.Accounts["work"].User.Email)
	assert.Equal(t, "work", out.Bindings["/Users/mario/dev/acme-api"].Profile)
}

func TestLoadOrMigrateV1(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.json")
	raw := `{"accessToken":"tk","refreshToken":"rt","apiUrl":"http://localhost:7070","workspace":"acme","workspaceId":"ws_1","project":"api","projectId":"pr_1","environment":"staging"}`
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	store, err := config.LoadOrMigrate()
	require.NoError(t, err)
	assert.Equal(t, config.StoreVersion, store.Version)
	assert.Equal(t, config.DefaultProfile, store.Current)

	acc := store.ActiveAccount()
	require.NotNil(t, acc)
	assert.Equal(t, "tk", acc.AccessToken)
	assert.Equal(t, "rt", acc.RefreshToken)
	assert.Equal(t, "http://localhost:7070", acc.APIUrl)
	assert.Equal(t, "acme", acc.Defaults.Workspace)
	assert.Equal(t, "ws_1", acc.Defaults.WorkspaceID)
	assert.Equal(t, "pr_1", acc.Defaults.ProjectID)
	assert.Equal(t, "staging", acc.Defaults.Environment)

	// Backup was created alongside the original.
	_, err = os.Stat(path + ".bak")
	assert.NoError(t, err)

	// File on disk is now v2 JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var probe struct {
		Version  int                 `json:"version"`
		Accounts map[string]struct{} `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(data, &probe))
	assert.Equal(t, config.StoreVersion, probe.Version)
	assert.Contains(t, probe.Accounts, config.DefaultProfile)
}

func TestLoadOrMigrateIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.json")
	raw := `{"accessToken":"tk"}`
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	_, err := config.LoadOrMigrate()
	require.NoError(t, err)

	before, err := os.ReadFile(path)
	require.NoError(t, err)

	// Second call must not re-migrate or re-write (bytes identical).
	_, err = config.LoadOrMigrate()
	require.NoError(t, err)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))

	// No timestamped backup was created on the second pass.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	backups := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "config.json.bak") {
			backups++
		}
	}
	assert.Equal(t, 1, backups, "exactly one backup should exist after re-migration")
}

func TestBackupDoesNotOverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.json")

	// Pre-existing .bak from some earlier session that must not be clobbered.
	prior := []byte("PRIOR-BACKUP")
	require.NoError(t, os.WriteFile(path+".bak", prior, 0o600))

	// Fresh v1 file that will trigger migration.
	require.NoError(t, os.WriteFile(path, []byte(`{"accessToken":"tk"}`), 0o600))

	_, err := config.LoadOrMigrate()
	require.NoError(t, err)

	got, err := os.ReadFile(path + ".bak")
	require.NoError(t, err)
	assert.Equal(t, string(prior), string(got), ".bak must be preserved byte-for-byte")

	// Timestamped backup exists alongside.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	hasTimestamped := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "config.json.bak.") {
			hasTimestamped = true
		}
	}
	assert.True(t, hasTimestamped, "a .bak.<timestamp> file must exist when .bak is occupied")
}

func TestEmptyV1FileDoesNotBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))

	_, err := config.LoadOrMigrate()
	require.NoError(t, err)

	_, err = os.Stat(path + ".bak")
	assert.True(t, os.IsNotExist(err), "empty v1 file must not produce a backup")
}

func TestLoadStoreDoesNotWrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := filepath.Join(tmpDir, ".flagify")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.json")
	raw := `{"accessToken":"tk"}`
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	mtime := info.ModTime()

	store, err := config.LoadStore()
	require.NoError(t, err)
	assert.Equal(t, "tk", store.ActiveAccount().AccessToken)

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, mtime, after.ModTime(), "LoadStore must not write")

	_, err = os.Stat(path + ".bak")
	assert.True(t, os.IsNotExist(err), "LoadStore must not create a backup")
}

func TestSaveStorePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions don't apply on Windows")
	}
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	s := &config.Store{Current: "work", Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}}}
	require.NoError(t, config.SaveStore(s))

	path, err := config.Path()
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

func TestShimSavePreservesV2Structure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Seed a v2 store with two accounts — shim Save() on Load'd cfg must not
	// wipe the sibling account or flip Current.
	seeded := &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt"},
			"personal": {AccessToken: "pt"},
		},
	}
	require.NoError(t, config.SaveStore(seeded))

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Environment = "production"
	require.NoError(t, config.Save(cfg))

	store, err := config.LoadStore()
	require.NoError(t, err)
	assert.Equal(t, "work", store.Current, "shim Save must not change Current")
	require.Contains(t, store.Accounts, "personal")
	assert.Equal(t, "pt", store.Accounts["personal"].AccessToken, "sibling profile must be untouched")
	assert.Equal(t, "production", store.Accounts["work"].Defaults.Environment)
}

func TestShimSaveSeedsDefaultProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &config.Config{AccessToken: "first-token"}
	require.NoError(t, config.Save(cfg))

	store, err := config.LoadStore()
	require.NoError(t, err)
	assert.Equal(t, config.DefaultProfile, store.Current)
	require.Contains(t, store.Accounts, config.DefaultProfile)
	assert.Equal(t, "first-token", store.Accounts[config.DefaultProfile].AccessToken)
}
