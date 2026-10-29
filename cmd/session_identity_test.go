package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

func testRefreshIdentity(user, device, generation string) string {
	payload, _ := json.Marshal(map[string]string{"type": "refresh", "uid": user, "did": device, "jti": generation})
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".test"
}

func TestRefreshSessionRejectsReplacedIdentity(t *testing.T) {
	for _, change := range []string{"user", "device", "server", "uninspectable"} {
		t.Run(change, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests > 1 {
					t.Error("replacement credentials reached the original server")
				}
				writeAuthError(w, "token_invalid", "expired")
			}))
			defer server.Close()
			original := &config.Account{AccessToken: "expired-a", RefreshToken: testRefreshIdentity("a", "device-a", "old"), DeviceID: "device-a", APIUrl: server.URL}
			seedStore(t, &config.Store{Version: config.StoreVersion, Current: "shared", Accounts: map[string]*config.Account{"shared": original}})
			client, err := getClientFromResolved(&config.ResolvedConfig{Profile: "shared", APIUrl: server.URL, Account: original})
			if err != nil {
				t.Fatal(err)
			}
			if err := config.UpdateStore(func(store *config.Store) error {
				replacement := *store.Accounts["shared"]
				replacement.AccessToken = "access-b"
				switch change {
				case "user":
					replacement.RefreshToken = testRefreshIdentity("b", "device-a", "new")
				case "device":
					replacement.DeviceID = "device-b"
				case "server":
					replacement.APIUrl = "https://other.invalid"
				case "uninspectable":
					replacement.RefreshToken = "opaque-replacement"
				}
				store.Accounts["shared"] = &replacement
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetMe(); err == nil || !strings.Contains(err.Error(), "changed account, device or API server") {
				t.Fatalf("expected identity-change rejection, got %v", err)
			}
			if requests != 1 {
				t.Fatalf("sent %d requests, expected original request only", requests)
			}
		})
	}
}

func TestPersistentSnapshotsShareOneRefresh(t *testing.T) {
	credential := persistentTestCredential()
	refreshes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/auth/refresh" {
			refreshes++
			fmt.Fprintf(w, `{"accessToken":"fresh-access","refreshToken":%q}`, credential)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh-access" {
			writeAuthError(w, "token_invalid", "expired")
			return
		}
		fmt.Fprint(w, `{"id":"account-one","email":"one@example.com"}`)
	}))
	defer server.Close()
	account := &config.Account{AccessToken: "expired", RefreshToken: credential, DeviceID: "cli-account", APIUrl: server.URL}
	seedStore(t, &config.Store{Version: config.StoreVersion, Current: "account", Accounts: map[string]*config.Account{"account": account}})
	first, err := getClientFromResolved(&config.ResolvedConfig{Profile: "account", APIUrl: server.URL, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	second, err := getClientFromResolved(&config.ResolvedConfig{Profile: "account", APIUrl: server.URL, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.GetMe(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.GetMe(); err != nil {
		t.Fatal(err)
	}
	if refreshes != 1 {
		t.Fatalf("got %d refreshes for the same expired snapshot", refreshes)
	}
}

func TestConfigBootstrap(t *testing.T) {
	seedStore(t, &config.Store{Version: config.StoreVersion, Accounts: map[string]*config.Account{}})
	for _, pair := range [][2]string{{"api-url", "https://private.example"}, {"console-url", "https://console.private.example"}} {
		if err := configSetCmd.RunE(configSetCmd, pair[:]); err != nil {
			t.Fatal(err)
		}
	}
	store := loadStoreForTest(t)
	account := store.ActiveAccount()
	if account == nil || account.APIUrl != "https://private.example" || account.ConsoleUrl != "https://console.private.example" {
		t.Fatal("bootstrap URLs were not preserved")
	}
	if account.AccessToken != "" || account.RefreshToken != "" {
		t.Fatal("configuration unexpectedly authenticated")
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"invalid", "value"}); err == nil {
		t.Fatal("unknown config key accepted")
	}

	// A store with profiles but no explicit selection is valid; bootstrap must
	// reuse the existing default profile instead of deleting its credentials.
	seedStore(t, &config.Store{Version: config.StoreVersion, Accounts: map[string]*config.Account{
		"default": {AccessToken: "existing-access", RefreshToken: "existing-refresh", Defaults: config.Defaults{Project: "existing-project"}},
		"other":   {AccessToken: "other-access"},
	}})
	if err := configSetCmd.RunE(configSetCmd, []string{"api-url", "https://private.example"}); err != nil {
		t.Fatal(err)
	}
	store = loadStoreForTest(t)
	account = store.Accounts["default"]
	if account.AccessToken != "existing-access" || account.RefreshToken != "existing-refresh" || account.Defaults.Project != "existing-project" || store.Accounts["other"].AccessToken != "other-access" {
		t.Fatal("bootstrap replaced existing profile credentials or defaults")
	}
}
