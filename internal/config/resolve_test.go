package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *config.Store {
	t.Helper()
	return &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken: "wt",
				APIUrl:      "https://api.flagify.dev",
				Defaults: config.Defaults{
					Workspace:   "acme",
					WorkspaceID: "ws_work",
					Project:     "api",
					ProjectID:   "pr_work",
					Environment: "development",
				},
			},
			"personal": {
				AccessToken: "pt",
				Defaults: config.Defaults{
					WorkspaceID: "ws_personal",
					ProjectID:   "pr_personal",
					Environment: "staging",
				},
			},
		},
		Bindings: map[string]config.Binding{},
	}
}

func writeProjectFile(t *testing.T, dir string, pfd config.ProjectFileData) {
	t.Helper()
	_, err := config.WriteProjectFile(dir, pfd)
	require.NoError(t, err)
}

// --- Profile resolution ---------------------------------------------------

func TestResolveProfile_FlagWins(t *testing.T) {
	rc, err := config.Resolve(config.ResolveInput{
		Flags: config.FlagValues{Profile: "personal"},
		Env:   config.EnvValues{Profile: "work"},
		Store: newTestStore(t),
	})
	require.NoError(t, err)
	assert.Equal(t, "personal", rc.Profile)
	assert.Equal(t, config.SourceFlag, rc.Sources["profile"])
}

func TestResolveProfile_EnvBeatsBindingAndCurrent(t *testing.T) {
	store := newTestStore(t)
	rc, err := config.Resolve(config.ResolveInput{
		Env:   config.EnvValues{Profile: "personal"},
		Store: store,
	})
	require.NoError(t, err)
	assert.Equal(t, "personal", rc.Profile)
	assert.Equal(t, config.SourceEnv, rc.Sources["profile"])
}

func TestResolveProfile_BindingWins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{WorkspaceID: "ws_x", ProjectID: "pr_x"})

	store := newTestStore(t)
	require.NoError(t, config.BindProfile(store, tmpDir, "personal"))

	rc, err := config.Resolve(config.ResolveInput{Store: store, CWD: tmpDir})
	require.NoError(t, err)
	assert.Equal(t, "personal", rc.Profile)
	assert.Equal(t, config.SourceBinding, rc.Sources["profile"])
}

func TestResolveProfile_PreferredProfileWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{
		WorkspaceID:      "ws_x",
		ProjectID:        "pr_x",
		PreferredProfile: "personal",
	})

	rc, err := config.Resolve(config.ResolveInput{Store: newTestStore(t), CWD: tmpDir})
	require.NoError(t, err)
	assert.Equal(t, "personal", rc.Profile)
	assert.Equal(t, config.SourceProjectFile, rc.Sources["profile"])
}

func TestResolveProfile_GhostPreferredFallsThrough(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{
		WorkspaceID:      "ws_x",
		PreferredProfile: "nonexistent",
	})

	rc, err := config.Resolve(config.ResolveInput{Store: newTestStore(t), CWD: tmpDir})
	require.NoError(t, err)
	// Should fall through to current profile.
	assert.Equal(t, "work", rc.Profile)
	assert.Equal(t, config.SourceProfile, rc.Sources["profile"])
}

func TestResolveProfile_GhostBindingFallsThrough(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{WorkspaceID: "ws_x"})

	store := newTestStore(t)
	store.Bindings = map[string]config.Binding{}
	// Manually inject a binding to a non-existent profile.
	abs, _ := filepath.Abs(tmpDir)
	store.Bindings[abs] = config.Binding{Profile: "ghost"}

	rc, err := config.Resolve(config.ResolveInput{Store: store, CWD: tmpDir})
	require.NoError(t, err)
	assert.Equal(t, "work", rc.Profile, "ghost binding should not win — fall through to current")
}

func TestResolveProfile_CurrentWhenNoProjectFile(t *testing.T) {
	rc, err := config.Resolve(config.ResolveInput{Store: newTestStore(t)})
	require.NoError(t, err)
	assert.Equal(t, "work", rc.Profile)
	assert.Equal(t, config.SourceProfile, rc.Sources["profile"])
}

func TestResolveProfile_SingleAccountUnambiguous(t *testing.T) {
	store := &config.Store{
		Version:  config.StoreVersion,
		Accounts: map[string]*config.Account{"solo": {AccessToken: "x"}},
		// Deliberately no Current set.
	}
	rc, err := config.Resolve(config.ResolveInput{Store: store})
	require.NoError(t, err)
	assert.Equal(t, "solo", rc.Profile)
}

func TestResolveProfile_AmbiguousWhenMultiple(t *testing.T) {
	store := &config.Store{
		Version: config.StoreVersion,
		// No Current, no flag, no env — two accounts is ambiguous.
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "w"},
			"personal": {AccessToken: "p"},
		},
	}
	_, err := config.Resolve(config.ResolveInput{Store: store})
	assert.ErrorIs(t, err, config.ErrAmbiguousProfile)
}

func TestResolveProfile_EmptyStoreNoError(t *testing.T) {
	rc, err := config.Resolve(config.ResolveInput{Store: &config.Store{Version: config.StoreVersion}})
	require.NoError(t, err)
	assert.Empty(t, rc.Profile)
}

// --- Per-field precedence -------------------------------------------------

func TestResolveField_FlagBeatsEnvBeatsProjectBeatsProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{
		WorkspaceID: "ws_project_file",
		ProjectID:   "pr_project_file",
		Environment: "staging",
	})

	store := newTestStore(t) // work defaults: ws_work, pr_work, development

	rc, err := config.Resolve(config.ResolveInput{
		Flags: config.FlagValues{Environment: "production"},
		Env:   config.EnvValues{ProjectID: "pr_env"},
		Store: store,
		CWD:   tmpDir,
	})
	require.NoError(t, err)

	// environment: --flag wins
	assert.Equal(t, "production", rc.Environment)
	assert.Equal(t, config.SourceFlag, rc.Sources["environment"])

	// projectId: env beats project file
	assert.Equal(t, "pr_env", rc.ProjectID)
	assert.Equal(t, config.SourceEnv, rc.Sources["projectId"])

	// workspaceId: project file beats profile default
	assert.Equal(t, "ws_project_file", rc.WorkspaceID)
	assert.Equal(t, config.SourceProjectFile, rc.Sources["workspaceId"])
}

func TestResolveField_ProfileDefaultWhenNothingElse(t *testing.T) {
	rc, err := config.Resolve(config.ResolveInput{Store: newTestStore(t)})
	require.NoError(t, err)

	assert.Equal(t, "ws_work", rc.WorkspaceID)
	assert.Equal(t, config.SourceProfile, rc.Sources["workspaceId"])
	assert.Equal(t, "development", rc.Environment)
	assert.Equal(t, config.SourceProfile, rc.Sources["environment"])
}

func TestResolveField_IDAndSlugIndependent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{ProjectID: "pr_file"})

	rc, err := config.Resolve(config.ResolveInput{
		Flags: config.FlagValues{Project: "api-slug-from-flag"},
		Store: newTestStore(t),
		CWD:   tmpDir,
	})
	require.NoError(t, err)

	// Slug came from --flag, ID from project file — independent.
	assert.Equal(t, "api-slug-from-flag", rc.Project)
	assert.Equal(t, config.SourceFlag, rc.Sources["project"])
	assert.Equal(t, "pr_file", rc.ProjectID)
	assert.Equal(t, config.SourceProjectFile, rc.Sources["projectId"])
}

func TestResolveAPIUrl_Precedence(t *testing.T) {
	t.Run("flag", func(t *testing.T) {
		rc, err := config.Resolve(config.ResolveInput{
			Flags: config.FlagValues{APIUrl: "https://flag.example"},
			Env:   config.EnvValues{APIUrl: "https://env.example"},
			Store: newTestStore(t),
		})
		require.NoError(t, err)
		assert.Equal(t, "https://flag.example", rc.APIUrl)
		assert.Equal(t, config.SourceFlag, rc.Sources["apiUrl"])
	})

	t.Run("env", func(t *testing.T) {
		rc, err := config.Resolve(config.ResolveInput{
			Env:   config.EnvValues{APIUrl: "https://env.example"},
			Store: newTestStore(t),
		})
		require.NoError(t, err)
		assert.Equal(t, "https://env.example", rc.APIUrl)
		assert.Equal(t, config.SourceEnv, rc.Sources["apiUrl"])
	})

	t.Run("profile", func(t *testing.T) {
		rc, err := config.Resolve(config.ResolveInput{Store: newTestStore(t)})
		require.NoError(t, err)
		assert.Equal(t, "https://api.flagify.dev", rc.APIUrl)
		assert.Equal(t, config.SourceProfile, rc.Sources["apiUrl"])
	})

	t.Run("default when no account", func(t *testing.T) {
		rc, err := config.Resolve(config.ResolveInput{})
		require.NoError(t, err)
		assert.Equal(t, config.DefaultAPIUrl, rc.APIUrl)
		assert.Equal(t, config.SourceDefault, rc.Sources["apiUrl"])
	})
}

func TestResolveConsoleUrl_DefaultWithoutAccount(t *testing.T) {
	rc, err := config.Resolve(config.ResolveInput{})
	require.NoError(t, err)
	assert.Equal(t, config.DefaultConsoleUrl, rc.ConsoleUrl)
	assert.Equal(t, config.SourceDefault, rc.Sources["consoleUrl"])
}

// --- Project file walker --------------------------------------------------

func TestFindProjectFile_FromSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	writeProjectFile(t, tmpDir, config.ProjectFileData{WorkspaceID: "ws_1", ProjectID: "pr_1"})

	sub := filepath.Join(tmpDir, "src", "deep", "nested")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	pf, err := config.FindProjectFile(sub)
	require.NoError(t, err)
	require.NotNil(t, pf)
	assert.Equal(t, tmpDir, pf.Dir)
	assert.Equal(t, "ws_1", pf.Data.WorkspaceID)
}

func TestFindProjectFile_StopsAtHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Place a project file *inside* HOME — the walker must refuse to even look.
	flagifyDir := filepath.Join(fakeHome, config.ProjectDirname)
	require.NoError(t, os.MkdirAll(flagifyDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(flagifyDir, config.ProjectFileBasename),
		[]byte(`{"version":1,"workspaceId":"bad"}`), 0o600,
	))

	pf, err := config.FindProjectFile(fakeHome)
	require.NoError(t, err)
	assert.Nil(t, pf, "walker must not read a project file located at $HOME itself")
}

func TestFindProjectFile_MissingReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	pf, err := config.FindProjectFile(tmpDir)
	require.NoError(t, err)
	assert.Nil(t, pf)
}

func TestFindProjectFile_InvalidJSONErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	flagifyDir := filepath.Join(tmpDir, config.ProjectDirname)
	require.NoError(t, os.MkdirAll(flagifyDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(flagifyDir, config.ProjectFileBasename),
		[]byte("{not-json"), 0o600,
	))

	_, err := config.FindProjectFile(tmpDir)
	require.Error(t, err)
}

func TestWriteProjectFile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	in := config.ProjectFileData{
		WorkspaceID:      "ws_1",
		Workspace:        "acme",
		ProjectID:        "pr_1",
		Project:          "api",
		Environment:      "development",
		PreferredProfile: "work",
	}
	written, err := config.WriteProjectFile(tmpDir, in)
	require.NoError(t, err)
	assert.Equal(t, config.ProjectFileVersion, written.Data.Version)

	read, err := config.FindProjectFile(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, read)
	assert.Equal(t, in.WorkspaceID, read.Data.WorkspaceID)
	assert.Equal(t, in.PreferredProfile, read.Data.PreferredProfile)
	assert.Equal(t, config.ProjectFileVersion, read.Data.Version)
}

// --- Bindings -------------------------------------------------------------

func TestBinding_CanonicalizesPath(t *testing.T) {
	store := &config.Store{Version: config.StoreVersion, Accounts: map[string]*config.Account{"work": {}}}

	rel := "."
	abs, _ := filepath.Abs(rel)

	require.NoError(t, config.BindProfile(store, rel, "work"))
	b, ok := config.BindingFor(store, rel)
	require.True(t, ok)
	assert.Equal(t, "work", b.Profile)

	// Lookup via the absolute form must match.
	b2, ok := config.BindingFor(store, abs)
	require.True(t, ok)
	assert.Equal(t, b, b2)
}

func TestBinding_PurgeForProfile(t *testing.T) {
	store := &config.Store{Version: config.StoreVersion}
	require.NoError(t, config.BindProfile(store, t.TempDir(), "work"))
	require.NoError(t, config.BindProfile(store, t.TempDir(), "work"))
	require.NoError(t, config.BindProfile(store, t.TempDir(), "personal"))

	removed := config.PurgeBindingsForProfile(store, "work")
	assert.Equal(t, 2, removed)
	assert.Len(t, store.Bindings, 1)
}

func TestBinding_UnbindMissingIsNoOp(t *testing.T) {
	store := &config.Store{Version: config.StoreVersion}
	require.NoError(t, config.UnbindProfile(store, t.TempDir()))
}

// --- HasToken -------------------------------------------------------------

func TestResolvedConfig_HasToken(t *testing.T) {
	rc := &config.ResolvedConfig{}
	assert.False(t, rc.HasToken())

	rc.Account = &config.Account{}
	assert.False(t, rc.HasToken())

	rc.Account.AccessToken = "t"
	assert.True(t, rc.HasToken())
}
