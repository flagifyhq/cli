package cmd

import (
	"testing"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
)

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
	client.OnTokenRefresh("wt-new", "wr-new")

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

func TestGetClientFromResolved_RefreshOnDeletedProfileIsNoOp(t *testing.T) {
	// Profile removed concurrently. Refresh callback must return silently — no
	// resurrected ghost profile in the store afterwards.
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
	client.OnTokenRefresh("wt-new", "wr-new")

	after := loadStoreForTest(t)
	if _, resurrected := after.Accounts["work"]; resurrected {
		t.Fatalf("deleted profile must not be recreated by refresh")
	}
	if after.Accounts["personal"].AccessToken != "pt" {
		t.Fatalf("sibling profile must be untouched")
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
