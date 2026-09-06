package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
)

// workspacesTestClient serves a fixed ListWorkspaces payload from a local
// httptest server and returns a client pointed at it.
func workspacesTestClient(t *testing.T, workspaces []api.Workspace) *api.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(workspaces)
	}))
	t.Cleanup(srv.Close)
	client := api.NewClient("test-token")
	client.SetBaseURL(srv.URL)
	return client
}

// stubConfirm replaces the project-file confirmation seam for one test and
// restores it on cleanup. Returns a pointer to the last skip value received.
func stubConfirm(t *testing.T, answer bool) *bool {
	t.Helper()
	orig := confirmProjectFileClear
	lastSkip := new(bool)
	confirmProjectFileClear = func(_ string, skip bool) (bool, error) {
		*lastSkip = skip
		return answer || skip, nil
	}
	t.Cleanup(func() { confirmProjectFileClear = orig })
	return lastSkip
}

func readProjectFileForTest(t *testing.T, dir string) config.ProjectFileData {
	t.Helper()
	pf, err := config.FindProjectFile(dir)
	if err != nil {
		t.Fatalf("read project file: %v", err)
	}
	if pf == nil {
		t.Fatalf("project file missing under %s", dir)
	}
	return pf.Data
}

func TestGetClientFromResolved_NoAccountFails(t *testing.T) {
	rc := &config.ResolvedConfig{Profile: "work"}
	_, err := getClientFromResolved(rc)
	if err == nil {
		t.Fatalf("expected error when rc has no account")
	}
}

func TestGetClientFromResolved_EnvTokenIsEphemeral(t *testing.T) {
	// Env token must build a client but NOT wire a refresh callback —
	// persisting refreshed env tokens would defeat the ephemeral contract.
	rc := &config.ResolvedConfig{
		Profile:         "work",
		APIUrl:          "http://127.0.0.1:0",
		EnvAccessToken:  "env-access",
		EnvRefreshToken: "env-refresh",
	}
	client, err := getClientFromResolved(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client must not be nil")
	}
	if client.OnTokenRefresh != nil {
		t.Fatalf("env token must not wire OnTokenRefresh (ephemeral contract)")
	}
}

func TestGetClientFromResolved_RefreshCallbackUpdatesCapturedProfile(t *testing.T) {
	// Seed: work + personal both logged in. A client built for "work" must
	// refresh work only, even if current flips to personal mid-flight.
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt-old", RefreshToken: "wr-old"},
			"personal": {AccessToken: "pt", RefreshToken: "pr"},
		},
	})

	rc := &config.ResolvedConfig{
		Profile: "work",
		Account: &config.Account{AccessToken: "wt-old", RefreshToken: "wr-old"},
	}
	client, err := getClientFromResolved(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.OnTokenRefresh == nil {
		t.Fatal("refresh callback must be wired when Account has a refresh token")
	}

	// Simulate current flipping to personal between request and refresh.
	store := loadStoreForTest(t)
	store.Current = "personal"
	if err := config.SaveStore(store); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Fire the refresh.
	if err := client.OnTokenRefresh("wt-new", "wr-new"); err != nil {
		t.Fatalf("persist refreshed tokens: %v", err)
	}

	after := loadStoreForTest(t)
	if after.Accounts["work"].AccessToken != "wt-new" {
		t.Fatalf("work access token not updated: %+v", after.Accounts["work"])
	}
	if after.Accounts["personal"].AccessToken != "pt" {
		t.Fatalf("personal profile must not be touched: %+v", after.Accounts["personal"])
	}
	if after.Current != "personal" {
		t.Fatalf("refresh must not flip Current back: got %q", after.Current)
	}
}

func TestGetClientFromResolved_RefreshOnDeletedProfileFails(t *testing.T) {
	// Profile removed concurrently. The callback must surface the persistence
	// failure and must not resurrect a ghost profile.
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "personal",
		Accounts: map[string]*config.Account{"personal": {AccessToken: "pt"}},
	})

	rc := &config.ResolvedConfig{
		Profile: "work", // profile that exists in rc but not in store
		Account: &config.Account{AccessToken: "wt-old", RefreshToken: "wr-old"},
	}
	client, err := getClientFromResolved(rc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	persistErr := client.OnTokenRefresh("wt-new", "wr-new")
	if persistErr == nil || !strings.Contains(persistErr.Error(), "no longer exists") {
		t.Fatalf("expected deleted-profile error, got %v", persistErr)
	}
	if !strings.Contains(persistErr.Error(), "flagify auth login --profile 'work'") {
		t.Fatalf("expected profile-specific login command, got %v", persistErr)
	}

	after := loadStoreForTest(t)
	if _, resurrected := after.Accounts["work"]; resurrected {
		t.Fatalf("deleted profile must not be recreated by refresh")
	}
	if after.Accounts["personal"].AccessToken != "pt" {
		t.Fatalf("sibling profile must be untouched")
	}
}

func TestGetClientFromResolved_RefreshFailureIncludesProfileRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		code := "access_expired"
		message := "access token expired"
		if r.URL.Path == "/v1/auth/refresh" {
			code = "session_revoked"
			message = "session has been revoked"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
	}))
	defer server.Close()

	client, err := getClientFromResolved(&config.ResolvedConfig{
		Profile: "client acme",
		APIUrl:  server.URL,
		Account: &config.Account{AccessToken: "expired-access", RefreshToken: "revoked-refresh"},
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	var result map[string]string
	err = client.Get("/v1/test", &result)
	if err == nil {
		t.Fatal("expected refresh failure")
	}
	if !strings.Contains(err.Error(), "session_revoked") {
		t.Fatalf("expected refresh cause, got %v", err)
	}
	if !strings.Contains(err.Error(), "flagify auth login --profile 'client acme'") {
		t.Fatalf("expected profile-specific recovery command, got %v", err)
	}
}

func TestProfileLoginCommandShellQuotesProfile(t *testing.T) {
	got := profileLoginCommand("work's profile")
	want := `flagify auth login --profile 'work'"'"'s profile'`
	if got != want {
		t.Fatalf("unexpected login command: got %q, want %q", got, want)
	}
}

func TestHandleAccessError_ClearsResolvedProfileDefaults(t *testing.T) {
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "personal",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken: "wt",
				Defaults:    config.Defaults{WorkspaceID: "ws_1", ProjectID: "pr_1", Environment: "staging"},
			},
			"personal": {
				AccessToken: "pt",
				Defaults:    config.Defaults{WorkspaceID: "ws_p", ProjectID: "pr_p", Environment: "development"},
			},
		},
	})

	rc := &config.ResolvedConfig{Profile: "work"}
	err := handleAccessError(&api.APIError{StatusCode: 403}, rc)
	if err == nil {
		t.Fatalf("expected wrapped access error")
	}

	after := loadStoreForTest(t)
	if after.Accounts["work"].Defaults.WorkspaceID != "" {
		t.Fatalf("work defaults should be cleared: %+v", after.Accounts["work"].Defaults)
	}
	if after.Accounts["personal"].Defaults.WorkspaceID != "ws_p" {
		t.Fatalf("personal (non-target) must be untouched: %+v", after.Accounts["personal"].Defaults)
	}
}

func TestHandleAccessError_EnvTokenDoesNotClearStore(t *testing.T) {
	// A 403 while using an env-override token must not write anything to disk —
	// the user's persisted profile had no say in this request.
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken: "wt",
				Defaults:    config.Defaults{WorkspaceID: "ws_1", ProjectID: "pr_1"},
			},
		},
	})

	rc := &config.ResolvedConfig{
		Profile:        "work",
		EnvAccessToken: "env-token",
	}
	_ = handleAccessError(&api.APIError{StatusCode: 403}, rc)

	after := loadStoreForTest(t)
	if after.Accounts["work"].Defaults.WorkspaceID != "ws_1" {
		t.Fatalf("env-token 403 must not clear persisted defaults: %+v", after.Accounts["work"].Defaults)
	}
}

func TestHandleAccessError_PassesNon403Through(t *testing.T) {
	orig := &api.APIError{StatusCode: 500, Message: "upstream"}
	got := handleAccessError(orig, &config.ResolvedConfig{Profile: "work"})
	if got != orig {
		t.Fatalf("non-403 error should pass through unchanged, got: %v", got)
	}
}

func TestHandleAccessError_ReconcilesStaleProjectBinding(t *testing.T) {
	// 403 with the project file bound to the failing workspace: the global
	// store clears silently, the project file clears after confirmation
	// (auto-confirmed here — tests run without a TTY), and the message states
	// both. Sibling fields clear together; environment and profile survive.
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				AccessToken: "wt",
				Defaults:    config.Defaults{WorkspaceID: "ws_stale", ProjectID: "pr_1"},
			},
		},
	})
	projDir := t.TempDir()
	pf, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID:      "ws_stale",
		Workspace:        "acme",
		ProjectID:        "pr_1",
		Project:          "api",
		Environment:      "staging",
		PreferredProfile: "work",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}

	rc := &config.ResolvedConfig{Profile: "work", WorkspaceID: "ws_stale", ProjectFile: pf}
	accessErr := handleAccessError(&api.APIError{StatusCode: 403}, rc)
	if accessErr == nil {
		t.Fatalf("expected wrapped access error")
	}
	if !strings.Contains(accessErr.Error(), "saved defaults") ||
		!strings.Contains(accessErr.Error(), "workspace binding") {
		t.Fatalf("message must state exactly what was cleared, got: %v", accessErr)
	}

	if after := loadStoreForTest(t); after.Accounts["work"].Defaults.WorkspaceID != "" {
		t.Fatalf("store defaults should be cleared: %+v", after.Accounts["work"].Defaults)
	}

	data := readProjectFileForTest(t, projDir)
	if data.WorkspaceID != "" || data.Workspace != "" || data.ProjectID != "" || data.Project != "" {
		t.Fatalf("workspace binding must be fully cleared, got: %+v", data)
	}
	if data.Environment != "staging" || data.PreferredProfile != "work" {
		t.Fatalf("environment/preferredProfile must be preserved, got: %+v", data)
	}
}

func TestHandleAccessError_DeclinedConfirmKeepsProjectFile(t *testing.T) {
	stubConfirm(t, false)
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	projDir := t.TempDir()
	pf, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID: "ws_stale", Workspace: "acme", ProjectID: "pr_1", Project: "api",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}

	rc := &config.ResolvedConfig{Profile: "work", WorkspaceID: "ws_stale", ProjectFile: pf}
	accessErr := handleAccessError(&api.APIError{StatusCode: 403}, rc)
	if accessErr == nil {
		t.Fatalf("expected wrapped access error")
	}
	if !strings.Contains(accessErr.Error(), "still references this workspace") {
		t.Fatalf("declined reconcile must be surfaced honestly, got: %v", accessErr)
	}
	if data := readProjectFileForTest(t, projDir); data.WorkspaceID != "ws_stale" {
		t.Fatalf("declined confirmation must leave the project file untouched, got: %+v", data)
	}
}

func TestHandleAccessError_UnrelatedWorkspaceLeavesProjectFileAlone(t *testing.T) {
	// The failing workspace came from a flag override, not from the file: the
	// committeable binding is not the culprit and must not be offered up.
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})
	projDir := t.TempDir()
	pf, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID: "ws_file", Workspace: "acme",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}

	rc := &config.ResolvedConfig{Profile: "work", Workspace: "other-team", ProjectFile: pf}
	accessErr := handleAccessError(&api.APIError{StatusCode: 403}, rc)
	if accessErr == nil {
		t.Fatalf("expected wrapped access error")
	}
	if strings.Contains(accessErr.Error(), "workspace binding") {
		t.Fatalf("message must not claim the project file was cleared, got: %v", accessErr)
	}
	if data := readProjectFileForTest(t, projDir); data.WorkspaceID != "ws_file" {
		t.Fatalf("unrelated project file must stay untouched, got: %+v", data)
	}
}

func TestValidateWorkspaceMembership_MemberPasses(t *testing.T) {
	client := workspacesTestClient(t, []api.Workspace{{ID: "ws_a", Slug: "alpha"}})

	byID := &config.ResolvedConfig{WorkspaceID: "ws_a", Sources: map[string]config.Source{}}
	if err := validateWorkspaceMembership(byID, client, false); err != nil {
		t.Fatalf("member by ID must pass: %v", err)
	}

	bySlug := &config.ResolvedConfig{Workspace: "alpha", Sources: map[string]config.Source{}}
	if err := validateWorkspaceMembership(bySlug, client, false); err != nil {
		t.Fatalf("member by slug must pass: %v", err)
	}
}

func TestValidateWorkspaceMembership_EmptyScopeSkips(t *testing.T) {
	client := workspacesTestClient(t, nil)
	rc := &config.ResolvedConfig{Sources: map[string]config.Source{}}
	if err := validateWorkspaceMembership(rc, client, false); err != nil {
		t.Fatalf("empty workspace scope must skip validation: %v", err)
	}
}

func TestValidateWorkspaceMembership_ListFailureSkipsGuard(t *testing.T) {
	// A defensive check must not cost availability: an unreachable
	// ListWorkspaces lets the command proceed to its own call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	client := api.NewClient("test-token")
	client.SetBaseURL(srv.URL)

	rc := &config.ResolvedConfig{WorkspaceID: "ws_a", Sources: map[string]config.Source{}}
	if err := validateWorkspaceMembership(rc, client, false); err != nil {
		t.Fatalf("ListWorkspaces failure must skip the guard: %v", err)
	}
}

func TestValidateWorkspaceMembership_NoTTYFullFlowNoLoop(t *testing.T) {
	// Full flow: repo bound to workspace A, profile re-authenticated as an
	// account that only sees workspace B. First scoped invocation errors with
	// an actionable message and reconciles the committed binding
	// (auto-confirm without a TTY); the second invocation resolves cleanly
	// instead of replaying the same failure.
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "b",
		Accounts: map[string]*config.Account{"b": {AccessToken: "tb"}},
	})
	projDir := t.TempDir()
	if _, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID: "ws_a", Workspace: "alpha", ProjectID: "pr_a", Project: "core",
		Environment: "development",
	}); err != nil {
		t.Fatalf("write project file: %v", err)
	}
	client := workspacesTestClient(t, []api.Workspace{{ID: "ws_b", Slug: "beta"}})

	first, err := config.Resolve(config.ResolveInput{Store: loadStoreForTest(t), CWD: projDir})
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if first.WorkspaceIdentifier() != "ws_a" {
		t.Fatalf("stale binding should resolve first, got %q", first.WorkspaceIdentifier())
	}

	firstErr := validateWorkspaceMembership(first, client, false)
	if firstErr == nil {
		t.Fatalf("mismatch must error in no-TTY")
	}
	if !strings.Contains(firstErr.Error(), "flagify projects pick") ||
		!strings.Contains(firstErr.Error(), "-w <slug>") {
		t.Fatalf("no-TTY error must name the recovery paths, got: %v", firstErr)
	}
	if data := readProjectFileForTest(t, projDir); data.WorkspaceID != "" || data.Environment != "development" {
		t.Fatalf("binding must be reconciled, environment preserved, got: %+v", data)
	}

	second, err := config.Resolve(config.ResolveInput{Store: loadStoreForTest(t), CWD: projDir})
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if second.WorkspaceIdentifier() != "" {
		t.Fatalf("second invocation must not resolve the stale workspace, got %q", second.WorkspaceIdentifier())
	}
	if err := validateWorkspaceMembership(second, client, false); err != nil {
		t.Fatalf("second invocation must not replay the mismatch: %v", err)
	}
}

func TestClearProjectFileBinding_PropagatesYesFlag(t *testing.T) {
	lastSkip := stubConfirm(t, true)
	projDir := t.TempDir()
	pf, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID: "ws_stale", Workspace: "acme", Environment: "staging",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}

	rc := &config.ResolvedConfig{ProjectFile: pf}
	if !clearProjectFileBinding(rc, "ws_stale", true) {
		t.Fatalf("expected the binding to be cleared")
	}
	if !*lastSkip {
		t.Fatalf("--yes must reach the confirmation as skip=true")
	}
	if data := readProjectFileForTest(t, projDir); data.WorkspaceID != "" || data.Environment != "staging" {
		t.Fatalf("binding cleared, environment preserved — got: %+v", data)
	}
}

func TestClearProjectFileBinding_EnvTokenNeverTouchesFile(t *testing.T) {
	projDir := t.TempDir()
	pf, err := config.WriteProjectFile(projDir, config.ProjectFileData{
		WorkspaceID: "ws_stale", Workspace: "acme",
	})
	if err != nil {
		t.Fatalf("write project file: %v", err)
	}

	rc := &config.ResolvedConfig{ProjectFile: pf, EnvAccessToken: "env-token"}
	if clearProjectFileBinding(rc, "ws_stale", true) {
		t.Fatalf("ephemeral env-token identity must never rewrite the project file")
	}
	if data := readProjectFileForTest(t, projDir); data.WorkspaceID != "ws_stale" {
		t.Fatalf("project file must be untouched, got: %+v", data)
	}
}
