package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

func resetInitFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		initCmd.Flags().Set("preferred-profile", "")
		initCmd.Flags().Set("print", "false")
		initCmd.Flags().Set("force", "false")
	})
}

func TestInit_CreatesProjectFileFromFlags(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	chdir(t, repo)

	_, err := runRoot(t, "init",
		"--workspace-id", "ws_1",
		"--project-id", "pr_1",
		"--environment", "development",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pf, err := config.FindProjectFile(repo)
	if err != nil || pf == nil {
		t.Fatalf("project file not created: %v %+v", err, pf)
	}
	if pf.Data.WorkspaceID != "ws_1" || pf.Data.ProjectID != "pr_1" || pf.Data.Environment != "development" {
		t.Fatalf("project file wrong: %+v", pf.Data)
	}
	if pf.Data.PreferredProfile != "work" {
		t.Fatalf("preferred profile should default to resolved profile, got %q", pf.Data.PreferredProfile)
	}
	if pf.Data.Version != config.ProjectFileVersion {
		t.Fatalf("version not set: %d", pf.Data.Version)
	}
}

func TestInit_IdempotentWhenUnchanged(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	args := []string{"init", "--workspace-id", "ws_1", "--project-id", "pr_1", "--environment", "development"}

	if _, err := runRoot(t, args...); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	pf, _ := config.FindProjectFile(repo)
	beforeInfo, err := os.Stat(pf.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	out, err := runRoot(t, args...)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if !strings.Contains(out, "Already initialized") {
		t.Fatalf("expected idempotent message, got: %q", out)
	}

	afterInfo, err := os.Stat(pf.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if afterInfo.ModTime() != beforeInfo.ModTime() {
		t.Fatalf("second init must not rewrite the file when nothing changed")
	}
}

func TestInit_FailsOnOverwriteWithoutForceInNonTTY(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	if _, err := runRoot(t, "init", "--workspace-id", "ws_1", "--project-id", "pr_1"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Different environment ⇒ would change the file. Tests run under non-TTY.
	_, err := runRoot(t, "init", "--workspace-id", "ws_1", "--project-id", "pr_1", "--environment", "staging")
	if err == nil {
		t.Fatalf("expected error without --force in non-TTY shell")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error should mention --force, got: %v", err)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	if _, err := runRoot(t, "init", "--workspace-id", "ws_1", "--project-id", "pr_1"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := runRoot(t, "init", "--workspace-id", "ws_1", "--project-id", "pr_1", "--environment", "staging", "--force"); err != nil {
		t.Fatalf("force run failed: %v", err)
	}
	pf, _ := config.FindProjectFile(repo)
	if pf.Data.Environment != "staging" {
		t.Fatalf("expected env=staging after --force, got %q", pf.Data.Environment)
	}
}

func TestInit_PrintDoesNotWrite(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	out, err := runRoot(t, "init", "--workspace-id", "ws_1", "--project-id", "pr_1", "--print")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nothing written to disk.
	if _, err := os.Stat(filepath.Join(repo, config.ProjectDirname, config.ProjectFileBasename)); !os.IsNotExist(err) {
		t.Fatalf("--print must not touch disk (err=%v)", err)
	}

	// Stdout contains a valid JSON with the requested fields.
	start := strings.Index(out, "{")
	var pfd config.ProjectFileData
	if err := json.Unmarshal([]byte(out[start:]), &pfd); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %q", err, out)
	}
	if pfd.WorkspaceID != "ws_1" || pfd.ProjectID != "pr_1" {
		t.Fatalf("print JSON wrong: %+v", pfd)
	}
}

func TestInit_RequiresWorkspaceAndProject(t *testing.T) {
	resetContextFlags(t)
	resetInitFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	repo := filepath.Join(home, "repo")
	_ = os.MkdirAll(repo, 0o755)
	chdir(t, repo)

	_, err := runRoot(t, "init")
	if err == nil {
		t.Fatalf("expected error when workspace is missing")
	}
	if !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("error should mention workspace, got: %v", err)
	}
}
