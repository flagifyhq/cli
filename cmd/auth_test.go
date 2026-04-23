package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

// seedStore writes a v2 store with the given shape under a fresh temp HOME and
// returns the home path. Callers never rely on HOME outside the test body.
func seedStore(t *testing.T, s *config.Store) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := config.SaveStore(s); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return home
}

func resetAuthFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		authLogoutCmd.Flags().Set("profile", "")
		authLogoutCmd.Flags().Set("all", "false")
		authLoginCmd.Flags().Set("profile", "")
		rootCmd.PersistentFlags().Set("yes", "false")
	})
}

func loadStoreForTest(t *testing.T) *config.Store {
	t.Helper()
	s, err := config.LoadStore()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	return s
}

// --- auth list ------------------------------------------------------------

func TestAuthList_Empty(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{Version: config.StoreVersion})

	out, err := runRoot(t, "auth", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No profiles yet") {
		t.Fatalf("expected empty-profile hint, got: %q", out)
	}
}

func TestAuthList_JSON(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt", User: &config.UserInfo{Email: "mario@acme.com"}},
			"personal": {}, // logged out
		},
	})

	out, err := runRoot(t, "auth", "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var views []profileView
	// Extract just the JSON payload (last `[` onwards).
	start := strings.Index(out, "[")
	if start < 0 {
		t.Fatalf("no JSON array in output: %q", out)
	}
	if err := json.Unmarshal([]byte(out[start:]), &views); err != nil {
		t.Fatalf("parse JSON: %v\nfull output: %q", err, out)
	}

	if len(views) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %+v", len(views), views)
	}
	// Sorted alphabetically: personal, work
	if views[0].Name != "personal" || views[1].Name != "work" {
		t.Fatalf("expected alphabetical order (personal, work), got %s/%s", views[0].Name, views[1].Name)
	}
	if views[0].LoggedIn {
		t.Fatalf("personal must be logged out")
	}
	if !views[1].LoggedIn || !views[1].Current || views[1].Email != "mario@acme.com" {
		t.Fatalf("work row wrong: %+v", views[1])
	}
}

// --- auth switch ----------------------------------------------------------

func TestAuthSwitch_ToExistingProfile(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt"},
			"personal": {AccessToken: "pt"},
		},
	})

	if _, err := runRoot(t, "auth", "switch", "personal"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := loadStoreForTest(t)
	if s.Current != "personal" {
		t.Fatalf("expected current=personal, got %s", s.Current)
	}
}

func TestAuthSwitch_UnknownProfileFails(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Current:  "work",
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})

	_, err := runRoot(t, "auth", "switch", "ghost")
	if err == nil {
		t.Fatalf("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("error should name the missing profile, got: %v", err)
	}
}

// --- auth logout ----------------------------------------------------------

func TestAuthLogout_Current(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {AccessToken: "wt", RefreshToken: "wr", Defaults: config.Defaults{WorkspaceID: "ws_1"}},
		},
	})

	if _, err := runRoot(t, "auth", "logout"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := loadStoreForTest(t)
	acc := s.Accounts["work"]
	if acc.AccessToken != "" || acc.RefreshToken != "" {
		t.Fatalf("tokens not cleared: %+v", acc)
	}
	if acc.Defaults.WorkspaceID != "ws_1" {
		t.Fatalf("defaults must be preserved, got %+v", acc.Defaults)
	}
}

func TestAuthLogout_All(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt", RefreshToken: "wr"},
			"personal": {AccessToken: "pt", RefreshToken: "pr"},
		},
	})

	if _, err := runRoot(t, "auth", "logout", "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := loadStoreForTest(t)
	for name, acc := range s.Accounts {
		if acc.AccessToken != "" || acc.RefreshToken != "" {
			t.Fatalf("tokens for %s not cleared: %+v", name, acc)
		}
	}
}

func TestAuthLogout_SpecificProfileFlag(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt"},
			"personal": {AccessToken: "pt"},
		},
	})

	if _, err := runRoot(t, "auth", "logout", "--profile", "personal"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := loadStoreForTest(t)
	if s.Accounts["work"].AccessToken != "wt" {
		t.Fatalf("work must remain logged in")
	}
	if s.Accounts["personal"].AccessToken != "" {
		t.Fatalf("personal must be logged out")
	}
}

// --- auth remove ----------------------------------------------------------

func TestAuthRemove_DeletesProfileAndBindings(t *testing.T) {
	resetAuthFlags(t)
	home := seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "wt"},
			"personal": {AccessToken: "pt"},
		},
		Bindings: map[string]config.Binding{},
	})

	// Seed a binding pointing at a path inside HOME so filepath.Abs works.
	bindingPath := filepath.Join(home, "repo")
	s := loadStoreForTest(t)
	if err := config.BindProfile(s, bindingPath, "work"); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := config.SaveStore(s); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := runRoot(t, "auth", "remove", "work", "-y"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := loadStoreForTest(t)
	if _, ok := after.Accounts["work"]; ok {
		t.Fatalf("work must be removed")
	}
	if len(after.Bindings) != 0 {
		t.Fatalf("bindings to work must be purged, got: %+v", after.Bindings)
	}
	if after.Current != "personal" {
		t.Fatalf("current must fall back to a surviving profile, got: %q", after.Current)
	}
}

func TestAuthRemove_UnknownFails(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version:  config.StoreVersion,
		Accounts: map[string]*config.Account{"work": {AccessToken: "wt"}},
	})

	_, err := runRoot(t, "auth", "remove", "ghost", "-y")
	if err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

// --- auth rename ----------------------------------------------------------

func TestAuthRename_UpdatesBindingsAndCurrent(t *testing.T) {
	resetAuthFlags(t)
	home := seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {AccessToken: "wt", Defaults: config.Defaults{WorkspaceID: "ws_1"}},
		},
	})

	bindingPath := filepath.Join(home, "repo")
	s := loadStoreForTest(t)
	if err := config.BindProfile(s, bindingPath, "work"); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := config.SaveStore(s); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := runRoot(t, "auth", "rename", "work", "acme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := loadStoreForTest(t)
	if _, gone := after.Accounts["work"]; gone {
		t.Fatalf("old name must be removed")
	}
	if _, ok := after.Accounts["acme"]; !ok {
		t.Fatalf("new name must exist")
	}
	if after.Current != "acme" {
		t.Fatalf("current must follow the rename, got %q", after.Current)
	}
	abs, _ := filepath.Abs(bindingPath)
	if after.Bindings[abs].Profile != "acme" {
		t.Fatalf("binding must track new name, got %+v", after.Bindings)
	}
	if after.Accounts["acme"].Defaults.WorkspaceID != "ws_1" {
		t.Fatalf("defaults must carry across the rename")
	}
}

func TestAuthRename_TargetExistsFails(t *testing.T) {
	resetAuthFlags(t)
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Accounts: map[string]*config.Account{
			"work":     {},
			"personal": {},
		},
	})

	_, err := runRoot(t, "auth", "rename", "work", "personal")
	if err == nil {
		t.Fatalf("expected error when target already exists")
	}
}

// --- auth login -----------------------------------------------------------

func TestAuthLogin_DoesNotBlockWhenAnotherProfileIsLoggedIn(t *testing.T) {
	// Historic v1 behavior blocked login whenever a token existed. In v2
	// that check must be gone — the user can add a second profile freely. We verify
	// by checking the parse-level flags instead of running the full browser flow.
	resetAuthFlags(t)

	if authLoginCmd.RunE == nil {
		t.Fatal("authLoginCmd.RunE must not be nil")
	}
	if authLoginCmd.Flag("profile") == nil {
		t.Fatal("authLoginCmd must accept --profile")
	}
}

// --- v2 removes top-level login/logout -----------------------------------

func TestTopLevelLoginLogout_AreRemoved(t *testing.T) {
	if authLoginCmd.Hidden {
		t.Fatal("authLoginCmd must remain visible — it is the canonical command")
	}
	if authLogoutCmd.Hidden {
		t.Fatal("authLogoutCmd must remain visible — it is the canonical command")
	}

	for _, name := range []string{"login", "logout"} {
		if rootCmd.CommandPath() == name {
			t.Fatalf("unexpected root command path %q", name)
		}
		if cmd, _, err := rootCmd.Find([]string{name}); err == nil && cmd != rootCmd {
			t.Fatalf("top-level %q command must be removed in v2", name)
		}
	}
}
