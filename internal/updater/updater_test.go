package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
		wantErr bool
	}{
		{"current older", "1.2.0", "v1.3.0", -1, false},
		{"current older no v", "v1.2.0", "1.3.0", -1, false},
		{"equal", "v1.3.0", "v1.3.0", 0, false},
		{"current newer", "v2.0.0", "v1.9.9", 1, false},
		{"malformed current", "not-a-version", "v1.0.0", 0, true},
		{"malformed latest", "v1.0.0", "banana", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.current, tt.latest)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CompareVersions(%q,%q) = %d, want %d", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod(t *testing.T) {
	// Isolate from the host environment so go-bin detection is deterministic.
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(t.TempDir(), "nonexistent-gopath"))

	tests := []struct {
		name string
		path string
		want InstallMethod
	}{
		{"homebrew intel", "/usr/local/Cellar/flagify/1.2.0/bin/flagify", MethodHomebrew},
		{"homebrew arm", "/opt/homebrew/Cellar/flagify/1.2.0/bin/flagify", MethodHomebrew},
		{"linuxbrew", "/home/linuxbrew/.linuxbrew/Cellar/flagify/1.2.0/bin/flagify", MethodHomebrew},
		{"npm", "/usr/lib/node_modules/@flagify/cli/bin/flagify", MethodNPM},
		{"direct", "/usr/local/bin/flagify", MethodDirect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectInstallMethod(tt.path); got != tt.want {
				t.Fatalf("DetectInstallMethod(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethod_GoBin(t *testing.T) {
	gobin := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobin)
	t.Setenv("GOPATH", "")

	path := filepath.Join(gobin, "flagify")
	if got := DetectInstallMethod(path); got != MethodGoInstall {
		t.Fatalf("DetectInstallMethod(%q) = %v, want MethodGoInstall", path, got)
	}
}

func TestDetectInstallMethod_GoPathBin(t *testing.T) {
	gopath := filepath.Join(t.TempDir(), "gopath")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", gopath)

	path := filepath.Join(gopath, "bin", "flagify")
	if got := DetectInstallMethod(path); got != MethodGoInstall {
		t.Fatalf("DetectInstallMethod(%q) = %v, want MethodGoInstall", path, got)
	}
}

func TestAssetName(t *testing.T) {
	supported := map[[2]string]string{
		{"darwin", "amd64"}:  "flagify_darwin_amd64.tar.gz",
		{"darwin", "arm64"}:  "flagify_darwin_arm64.tar.gz",
		{"linux", "amd64"}:   "flagify_linux_amd64.tar.gz",
		{"linux", "arm64"}:   "flagify_linux_arm64.tar.gz",
		{"windows", "amd64"}: "flagify_windows_amd64.zip",
		{"windows", "arm64"}: "flagify_windows_arm64.zip",
	}
	for key, want := range supported {
		t.Run(key[0]+"_"+key[1], func(t *testing.T) {
			got, err := AssetName(key[0], key[1])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Fatalf("AssetName(%q,%q) = %q, want %q", key[0], key[1], got, want)
			}
		})
	}

	if _, err := AssetName("plan9", "mips"); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestLatestRelease(t *testing.T) {
	origURL := LatestReleaseURL
	t.Cleanup(func() { LatestReleaseURL = origURL })

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") == "" {
				t.Errorf("missing User-Agent header")
			}
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Errorf("wrong Accept header: %q", r.Header.Get("Accept"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"tag_name":"v1.5.0","assets":[{"name":"flagify_linux_amd64.tar.gz","browser_download_url":"http://x/a"}]}`))
		}))
		defer srv.Close()
		LatestReleaseURL = srv.URL

		release, err := LatestRelease(context.Background(), srv.Client())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release.TagName != "v1.5.0" || len(release.Assets) != 1 {
			t.Fatalf("unexpected release: %+v", release)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		LatestReleaseURL = srv.URL

		if _, err := LatestRelease(context.Background(), srv.Client()); err != ErrReleaseNotFound {
			t.Fatalf("got %v, want ErrReleaseNotFound", err)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()
		LatestReleaseURL = srv.URL

		if _, err := LatestRelease(context.Background(), srv.Client()); err != ErrRateLimited {
			t.Fatalf("got %v, want ErrRateLimited", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		LatestReleaseURL = srv.URL

		if _, err := LatestRelease(context.Background(), srv.Client()); err == nil {
			t.Fatal("expected error for 500")
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"tag_name":"","assets":[]}`))
		}))
		defer srv.Close()
		LatestReleaseURL = srv.URL

		if _, err := LatestRelease(context.Background(), srv.Client()); err != ErrMalformedRelease {
			t.Fatalf("got %v, want ErrMalformedRelease", err)
		}
	})
}
