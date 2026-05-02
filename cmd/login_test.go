package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

func writeProjectFile(t *testing.T, dir string, data config.ProjectFileData) {
	t.Helper()
	if _, err := config.WriteProjectFile(dir, data); err != nil {
		t.Fatalf("write project file: %v", err)
	}
}

func TestAdoptCandidate_NoProjectFileReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if got := adoptCandidate(repo, "work"); got != nil {
		t.Fatalf("expected nil when no project file is present, got %+v", got)
	}
}

func TestAdoptCandidate_EmptyProfileReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})

	if got := adoptCandidate(repo, ""); got != nil {
		t.Fatalf("expected nil when profile is empty, got %+v", got)
	}
}

func TestAdoptCandidate_AlreadyPinnedReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "work",
	})

	if got := adoptCandidate(repo, "work"); got != nil {
		t.Fatalf("expected nil when preferredProfile already matches, got %+v", got)
	}
}

func TestAdoptCandidate_MismatchReturnsProjectFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})

	got := adoptCandidate(repo, "work")
	if got == nil {
		t.Fatalf("expected project file when pin differs, got nil")
	}
	if got.Data.PreferredProfile != "default" {
		t.Fatalf("unexpected project file: %+v", got.Data)
	}
	if got.Dir != repo {
		// t.TempDir() may canonicalize through /private on macOS; compare with EvalSymlinks.
		expected, _ := filepath.EvalSymlinks(repo)
		actual, _ := filepath.EvalSymlinks(got.Dir)
		if expected != actual {
			t.Fatalf("expected dir %q, got %q", repo, got.Dir)
		}
	}
}

func TestAdoptCandidate_EmptyPinMatchesAnyProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:     config.ProjectFileVersion,
		WorkspaceID: "ws_1",
		ProjectID:   "pr_1",
		Environment: "development",
	})

	got := adoptCandidate(repo, "work")
	if got == nil {
		t.Fatalf("expected adoptCandidate to return project file with empty pin")
	}
	if got.Data.PreferredProfile != "" {
		t.Fatalf("expected empty preferredProfile, got %q", got.Data.PreferredProfile)
	}
}

func TestAdoptCandidate_WalksUpFromSubdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})
	subdir := filepath.Join(repo, "apps", "marketing")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := adoptCandidate(subdir, "work")
	if got == nil {
		t.Fatalf("expected adoptCandidate to walk up and find project file from %q", subdir)
	}
	if got.Data.PreferredProfile != "default" {
		t.Fatalf("unexpected data: %+v", got.Data)
	}
}
