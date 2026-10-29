package cmd

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/flagifyhq/cli/internal/config"
)

func persistentTestCredential() string {
	return "e30." + base64.RawURLEncoding.EncodeToString([]byte(`{"type":"cli_refresh","uid":"account-one","did":"cli-account","jti":"random-grant","iat":1}`)) + ".signature"
}

func TestPersistentSessionSharedByFolders(t *testing.T) {
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
			w.WriteHeader(401)
			fmt.Fprint(w, `{"code":"token_invalid","message":"expired"}`)
			return
		}
		fmt.Fprint(w, `{"id":"account-one","email":"one@example.com"}`)
	}))
	defer server.Close()
	seedStore(t, &config.Store{Version: config.StoreVersion, Current: "other", Accounts: map[string]*config.Account{
		"account": {AccessToken: "expired", RefreshToken: credential, DeviceID: "cli-account", APIUrl: server.URL},
		"other":   {AccessToken: "other-access", RefreshToken: "other-refresh"},
	}})
	for _, project := range []string{"first", "second"} {
		folder := filepath.Join(t.TempDir(), project)
		_, err := config.WriteProjectFile(folder, config.ProjectFileData{PreferredProfile: "account", Project: project, Environment: "staging"})
		if err != nil {
			t.Fatal(err)
		}
		store := loadStoreForTest(t)
		resolved, err := config.Resolve(config.ResolveInput{CWD: folder, Store: store})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Profile != "account" || resolved.Project != project || resolved.Environment != "staging" {
			t.Fatalf("wrong folder scope: %+v", resolved)
		}
		client, err := getClientFromResolved(resolved)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.GetMe(); err != nil {
			t.Fatal(err)
		}
	}
	if refreshes != 1 {
		t.Fatalf("got %d refreshes, want one shared account refresh", refreshes)
	}
	store := loadStoreForTest(t)
	if store.Accounts["account"].RefreshToken != credential || store.Accounts["other"].AccessToken != "other-access" || store.Current != "other" {
		t.Fatal("credentials or current account unexpectedly changed")
	}
	if got := config.AccountAuthStatus(&config.Account{RefreshToken: credential}, time.Now().Add(365*24*time.Hour)); got != config.AuthStatusRefreshable {
		t.Fatalf("persistent credential became %s", got)
	}
}

func TestPersistentLogoutRevokesBeforeClearing(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/auth/logout" {
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
				}
				if fail {
					w.WriteHeader(503)
					fmt.Fprint(w, `{"code":"unavailable","message":"try later"}`)
					return
				}
				fmt.Fprint(w, `{"status":"ok"}`)
			}))
			defer server.Close()
			seedStore(t, &config.Store{Version: config.StoreVersion, Current: "account", Accounts: map[string]*config.Account{"account": {AccessToken: "access", RefreshToken: persistentTestCredential(), DeviceID: "cli-account", APIUrl: server.URL}}})
			err := runLogout("account", false)
			if (err != nil) != fail {
				t.Fatalf("logout error: %v", err)
			}
			if calls != 1 {
				t.Fatalf("revoke calls: %d", calls)
			}
			account := loadStoreForTest(t).Accounts["account"]
			if fail && account.RefreshToken == "" {
				t.Fatal("forgot unrevoked credential")
			}
			if !fail && (account.AccessToken != "" || account.RefreshToken != "") {
				t.Fatal("did not clear revoked credential")
			}
		})
	}
}
