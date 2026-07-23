package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/updater"
)

// stubGitHub points updater.LatestReleaseURL at an httptest server returning the
// given tag, restoring the original URL on cleanup.
func stubGitHub(t *testing.T, tag string) {
	t.Helper()
	orig := updater.LatestReleaseURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"` + tag + `","assets":[{"name":"flagify_linux_amd64.tar.gz","browser_download_url":"http://x/a"},{"name":"checksums.txt","browser_download_url":"http://x/c"}]}`))
	}))
	updater.LatestReleaseURL = srv.URL
	t.Cleanup(func() {
		updater.LatestReleaseURL = orig
		srv.Close()
	})
}

// setVersion temporarily overrides the injected build version.
func setVersion(t *testing.T, v string) {
	t.Helper()
	orig := Version
	Version = v
	t.Cleanup(func() { Version = orig })
}

// resetUpdateFlags clears the persisted flag state between runs.
func resetUpdateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		updateCmd.Flags().Set("check", "false")
		updateCmd.Flags().Set("force", "false")
	})
}

// setSeams overrides the update.go test seams and restores them on cleanup.
func setSeams(t *testing.T, interactive bool, execPath string, install func(context.Context, *updater.Release, string, bool) error) {
	t.Helper()
	origI, origR, origInstall := isInteractive, resolveExecPath, installRelease
	isInteractive = func() bool { return interactive }
	resolveExecPath = func() (string, error) { return execPath, nil }
	if install != nil {
		installRelease = install
	}
	t.Cleanup(func() {
		isInteractive, resolveExecPath, installRelease = origI, origR, origInstall
	})
}

func TestUpdate_CheckNewVersion(t *testing.T) {
	resetUpdateFlags(t)
	setVersion(t, "v1.0.0")
	stubGitHub(t, "v1.5.0")

	out, err := runRoot(t, "update", "--check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "A new version is available") {
		t.Fatalf("output missing new-version message: %q", out)
	}
}

func TestUpdate_CheckUpToDate(t *testing.T) {
	resetUpdateFlags(t)
	setVersion(t, "v1.5.0")
	stubGitHub(t, "v1.5.0")

	out, err := runRoot(t, "update", "--check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "latest version") {
		t.Fatalf("output missing up-to-date message: %q", out)
	}
}

func TestUpdate_DevBuildSkipsNetwork(t *testing.T) {
	resetUpdateFlags(t)
	setVersion(t, "dev")
	// Point the URL at a server that fails the test if hit.
	orig := updater.LatestReleaseURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dev build must not call GitHub")
	}))
	updater.LatestReleaseURL = srv.URL
	t.Cleanup(func() { updater.LatestReleaseURL = orig; srv.Close() })

	out, err := runRoot(t, "update")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "development build") {
		t.Fatalf("output missing dev-build message: %q", out)
	}
}

func TestUpdate_NonTTYWithoutYesRefuses(t *testing.T) {
	resetUpdateFlags(t)
	setVersion(t, "v1.0.0")
	stubGitHub(t, "v1.5.0")

	installCalled := false
	setSeams(t, false, "/usr/local/bin/flagify", func(context.Context, *updater.Release, string, bool) error {
		installCalled = true
		return nil
	})

	_, err := runRoot(t, "update")
	if err == nil {
		t.Fatal("expected an error in non-TTY without --yes")
	}
	if !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("wrong error: %v", err)
	}
	if installCalled {
		t.Fatal("install must not run when the non-TTY gate refuses")
	}
}

func TestUpdate_NonTTYWithYesProceeds(t *testing.T) {
	resetUpdateFlags(t)
	setVersion(t, "v1.0.0")
	stubGitHub(t, "v1.5.0")

	installCalled := false
	setSeams(t, false, "/usr/local/bin/flagify", func(_ context.Context, _ *updater.Release, target string, _ bool) error {
		installCalled = true
		if target != "/usr/local/bin/flagify" {
			t.Errorf("unexpected target: %q", target)
		}
		return nil
	})

	out, err := runRoot(t, "update", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installCalled {
		t.Fatal("install should run with --yes in non-TTY")
	}
	if !strings.Contains(out, "Successfully updated") {
		t.Fatalf("output missing success message: %q", out)
	}
}

func TestUpdate_ManagerRedirect(t *testing.T) {
	tests := []struct {
		name     string
		execPath string
		want     string
	}{
		{"homebrew", "/opt/homebrew/Cellar/flagify/1.0.0/bin/flagify", "brew upgrade flagify"},
		{"npm", "/usr/lib/node_modules/@flagify/cli/bin/flagify", "npm update -g @flagify/cli"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetUpdateFlags(t)
			setVersion(t, "v1.0.0")
			stubGitHub(t, "v1.5.0")

			installCalled := false
			setSeams(t, true, tt.execPath, func(context.Context, *updater.Release, string, bool) error {
				installCalled = true
				return nil
			})

			out, err := runRoot(t, "update", "--yes")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.want) {
				t.Fatalf("output missing manager command %q: %q", tt.want, out)
			}
			if installCalled {
				t.Fatal("manager-managed install must not self-replace")
			}
		})
	}
}
