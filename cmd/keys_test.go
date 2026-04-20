package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// writeTestConfig creates a minimal logged-in config under a temp HOME so
// getClient() succeeds and SetBaseURL points at an httptest server.
func writeTestConfig(t *testing.T, apiURL string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".flagify")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	cfg := map[string]any{
		"accessToken": "test-token",
		"apiUrl":      apiURL,
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// runRoot executes the root command with the given argv and returns (output, error).
// Flags set by a previous run are reset on exit to keep tests independent.
func runRoot(t *testing.T, argv ...string) (string, error) {
	stdout, stderr, err := runRootCapture(t, argv...)
	return stdout + stderr, err
}

// runRootCapture executes the root command and returns stdout and stderr
// separately so tests can assert on warnings emitted via fmt.Fprintln(os.Stderr, …).
func runRootCapture(t *testing.T, argv ...string) (string, string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(argv)

	origStdout, origStderr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = origStdout, origStderr
	}()

	err := rootCmd.Execute()

	outW.Close()
	errW.Close()
	capturedStdout, _ := io.ReadAll(outR)
	capturedStderr, _ := io.ReadAll(errR)

	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		keysRevokeCmd.Flags().Set("all", "false")
		keysRevokeCmd.Flags().Set("id", "")
	})

	return buf.String() + string(capturedStdout), string(capturedStderr), err
}

func TestKeysRevoke_RequiresSelector(t *testing.T) {
	writeTestConfig(t, "http://127.0.0.1:0")

	_, err := runRoot(t, "keys", "revoke", "-p", "proj_01", "-e", "development", "-y")
	if err == nil {
		t.Fatal("expected error when no prefix/--id/--all provided")
	}
	if !strings.Contains(err.Error(), "--all") || !strings.Contains(err.Error(), "prefix") {
		t.Fatalf("error should mention both options, got: %v", err)
	}
}

func TestKeysRevoke_MutuallyExclusiveFlags(t *testing.T) {
	writeTestConfig(t, "http://127.0.0.1:0")

	_, err := runRoot(t, "keys", "revoke", "pk_dev_abc", "--all", "-p", "proj_01", "-e", "development", "-y")
	if err == nil {
		t.Fatal("expected error when combining prefix and --all")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error should mention mutual exclusion, got: %v", err)
	}
}

func TestKeysRevoke_PrefixNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/keys") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "key_1", "environmentId": "env_1", "type": "publishable", "prefix": "pk_dev_zzz"},
			})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	writeTestConfig(t, srv.URL)

	_, err := runRoot(t, "keys", "revoke", "pk_dev_nope", "-p", "proj_01", "-e", "development", "-y")
	if err == nil {
		t.Fatal("expected error when prefix does not match any active key")
	}
	if !strings.Contains(err.Error(), "pk_dev_nope") {
		t.Fatalf("error should mention the missing prefix, got: %v", err)
	}
}

func TestKeysRevoke_PrefixHappyPathHitsProjectScopedRevoke(t *testing.T) {
	var listCalls, revokeCalls atomic.Int32
	var revokedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/projects/proj_01/environments/development/keys":
			listCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "key_ulid_1", "environmentId": "env_1", "type": "publishable", "prefix": "pk_dev_abc"},
				{"id": "key_ulid_2", "environmentId": "env_1", "type": "secret", "prefix": "sk_dev_xyz"},
			})
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/keys/") && strings.HasSuffix(r.URL.Path, "/revoke"):
			revokeCalls.Add(1)
			revokedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	writeTestConfig(t, srv.URL)

	if _, err := runRoot(t, "keys", "revoke", "pk_dev_abc", "-p", "proj_01", "-e", "development", "-y"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listCalls.Load() != 1 {
		t.Fatalf("expected 1 list call, got %d", listCalls.Load())
	}
	if revokeCalls.Load() != 1 {
		t.Fatalf("expected 1 revoke call, got %d", revokeCalls.Load())
	}
	want := "/v1/projects/proj_01/environments/development/keys/key_ulid_1/revoke"
	if revokedPath != want {
		t.Fatalf("revoke path = %q, want %q", revokedPath, want)
	}
}
