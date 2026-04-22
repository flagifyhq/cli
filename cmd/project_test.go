package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

func TestProjectBind_RequiresProfile(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	_, err := runRoot(t, "project", "bind")
	if err == nil {
		t.Fatalf("expected error when --profile is missing")
	}
	if !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("error should mention --profile, got: %v", err)
	}
}

func TestProjectBind_UnknownProfileFails(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	_, err := runRoot(t, "project", "bind", "--profile", "ghost")
	if err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

func TestProjectBind_RecordsBindingForProjectFileDir(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt"},
			"personal": {AccessToken: "pt"},
		},
	})

	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	if _, err := config.WriteProjectFile(repo, config.ProjectFileData{WorkspaceID: "ws_1", ProjectID: "pr_1"}); err != nil {
		t.Fatalf("write project file: %v", err)
	}

	// Bind from a subdirectory; the walker must find the project file and bind
	// against the repo root (pf.Dir), not cwd.
	sub := filepath.Join(repo, "src", "deep")
	_ = os.MkdirAll(sub, 0o755)
	chdir(t, sub)

	if _, err := runRoot(t, "project", "bind", "--profile", "personal"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// BindingFor canonicalizes (EvalSymlinks on macOS, etc.) — assert via the
	// exported lookup so we don't couple the test to the raw map key format.
	s := loadStoreForTest(t)
	b, ok := config.BindingFor(s, repo)
	if !ok {
		t.Fatalf("binding not recorded for repo root, store has: %+v", s.Bindings)
	}
	if b.Profile != "personal" {
		t.Fatalf("expected profile=personal, got %q", b.Profile)
	}
	if len(s.Bindings) != 1 {
		t.Fatalf("expected exactly one binding, got %d", len(s.Bindings))
	}
}

func TestProjectSet_UpdatesField(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	_, err := config.WriteProjectFile(repo, config.ProjectFileData{
		WorkspaceID: "ws_1",
		ProjectID:   "pr_1",
		Environment: "development",
	})
	if err != nil {
		t.Fatalf("seed project file: %v", err)
	}
	chdir(t, repo)

	if _, err := runRoot(t, "project", "set", "environment", "staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pf, err := config.FindProjectFile(repo)
	if err != nil || pf == nil {
		t.Fatalf("reload: %v", err)
	}
	if pf.Data.Environment != "staging" {
		t.Fatalf("expected env=staging, got %q", pf.Data.Environment)
	}
}

func TestProjectSet_UnknownFieldFails(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	_, _ = config.WriteProjectFile(repo, config.ProjectFileData{WorkspaceID: "ws_1", ProjectID: "pr_1"})
	chdir(t, repo)

	_, err := runRoot(t, "project", "set", "unknown-field", "x")
	if err == nil {
		t.Fatalf("expected error for unknown field")
	}
}

func TestProjectSet_NoProjectFileFails(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	_, err := runRoot(t, "project", "set", "environment", "staging")
	if err == nil {
		t.Fatalf("expected error when no project file exists")
	}
	if !strings.Contains(err.Error(), "flagify init") {
		t.Fatalf("error should hint at 'flagify init', got: %v", err)
	}
}
