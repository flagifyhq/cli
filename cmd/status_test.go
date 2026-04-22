package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

// chdir cds into dir for the duration of the test and restores cwd after.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func resetContextFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.PersistentFlags().Set("profile", "")
		rootCmd.PersistentFlags().Set("workspace", "")
		rootCmd.PersistentFlags().Set("workspace-id", "")
		rootCmd.PersistentFlags().Set("project", "")
		rootCmd.PersistentFlags().Set("project-id", "")
		rootCmd.PersistentFlags().Set("environment", "")
		rootCmd.PersistentFlags().Set("yes", "false")
	})
}

func TestStatus_JSONIncludesSources(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken: "wt",
				APIUrl:      "https://api.flagify.dev",
				User:        &config.UserInfo{Email: "mario@acme.com"},
				Defaults: config.Defaults{
					Workspace:   "acme",
					WorkspaceID: "ws_1",
					Project:     "api",
					ProjectID:   "pr_1",
					Environment: "development",
				},
			},
		},
	})
	chdir(t, home)

	out, err := runRoot(t, "status", "--format", "json", "--environment", "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON object in output: %q", out)
	}
	var p statusPayload
	if err := json.Unmarshal([]byte(out[start:]), &p); err != nil {
		t.Fatalf("parse: %v\nraw: %q", err, out)
	}

	if p.Profile.Value != "work" || p.Profile.Source != string(config.SourceProfile) {
		t.Fatalf("profile wrong: %+v", p.Profile)
	}
	if p.Environment.Value != "production" || p.Environment.Source != string(config.SourceFlag) {
		t.Fatalf("--environment should win via flag: %+v", p.Environment)
	}
	if p.WorkspaceID.Value != "ws_1" || p.WorkspaceID.Source != string(config.SourceProfile) {
		t.Fatalf("workspaceId wrong: %+v", p.WorkspaceID)
	}
	if p.APIUrl.Source != string(config.SourceProfile) {
		t.Fatalf("apiUrl should come from profile: %+v", p.APIUrl)
	}
	if p.Email != "mario@acme.com" {
		t.Fatalf("email missing: %+v", p)
	}
}

func TestStatus_ProjectFilePathSurfacedInJSON(t *testing.T) {
	resetContextFlags(t)
	home := seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})

	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := config.WriteProjectFile(repo, config.ProjectFileData{
		WorkspaceID: "ws_1",
		ProjectID:   "pr_1",
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}
	chdir(t, repo)

	out, err := runRoot(t, "status", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	start := strings.Index(out, "{")
	var p statusPayload
	if err := json.Unmarshal([]byte(out[start:]), &p); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.ProjectFile == "" {
		t.Fatalf("projectFile should be populated when present")
	}
	if p.Environment.Source != string(config.SourceProjectFile) {
		t.Fatalf("environment should come from project file: %+v", p.Environment)
	}
}
