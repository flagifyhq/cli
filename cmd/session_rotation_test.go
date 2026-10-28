package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

// rotatingSessionServer stands in for the real API's auth behaviour: one
// session row per (user, device) holding a single refresh-token hash, rotated
// on every refresh. Presenting a superseded refresh token is answered with
// 401 session_revoked, the same way api/internal/domain/auth/service.go does.
type rotatingSessionServer struct {
	mu            sync.Mutex
	access        string
	refresh       string
	generation    int
	rotations     int
	revokedAnswer int
}

func newRotatingSessionServer(t *testing.T, access, refresh string) (*httptest.Server, *rotatingSessionServer) {
	t.Helper()
	state := &rotatingSessionServer{access: access, refresh: refresh}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		state.mu.Lock()
		defer state.mu.Unlock()

		if body.RefreshToken != state.refresh {
			state.revokedAnswer++
			writeAuthError(w, "session_revoked", "session has been revoked")
			return
		}
		state.generation++
		state.rotations++
		state.access = fmt.Sprintf("access-gen%d", state.generation)
		state.refresh = fmt.Sprintf("refresh-gen%d", state.generation)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  state.access,
			"refreshToken": state.refresh,
		})
	})
	mux.HandleFunc("/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		expected := "Bearer " + state.access
		state.mu.Unlock()

		if r.Header.Get("Authorization") != expected {
			writeAuthError(w, "token_expired", "access token expired")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "email": "mario@example.com"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, state
}

func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

func (s *rotatingSessionServer) snapshot() (rotations, revoked int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotations, s.revokedAnswer
}

// TestSecondInvocationAdoptsRotatedSessionInsteadOfFailing covers the reported
// symptom directly: two invocations of the same profile both start from the
// token pair that was on disk, the first rotates it, and the second used to
// present the now-consumed refresh token and be told to log in again — while a
// perfectly valid pair sat in config.json, which is why running the same
// command a minute later worked without anyone logging in.
func TestSecondInvocationAdoptsRotatedSessionInsteadOfFailing(t *testing.T) {
	srv, state := newRotatingSessionServer(t, "access-gen0", "refresh-gen0")

	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "cdr",
		Accounts: map[string]*config.Account{
			"cdr": {AccessToken: "access-expired", RefreshToken: "refresh-gen0", APIUrl: srv.URL},
		},
	})

	// Both invocations resolved the same snapshot, before any rotation.
	invocation := func() *config.ResolvedConfig {
		return &config.ResolvedConfig{
			Profile: "cdr",
			APIUrl:  srv.URL,
			Account: &config.Account{
				AccessToken:  "access-expired",
				RefreshToken: "refresh-gen0",
				APIUrl:       srv.URL,
			},
		}
	}

	first, err := getClientFromResolved(invocation())
	if err != nil {
		t.Fatalf("build first client: %v", err)
	}
	second, err := getClientFromResolved(invocation())
	if err != nil {
		t.Fatalf("build second client: %v", err)
	}

	if _, err := first.GetMe(); err != nil {
		t.Fatalf("first invocation should refresh and succeed: %v", err)
	}
	if _, err := second.GetMe(); err != nil {
		t.Fatalf("second invocation must adopt the rotated session, got: %v", err)
	}

	rotations, revoked := state.snapshot()
	if rotations != 1 {
		t.Fatalf("expected exactly one rotation for one expired session, got %d", rotations)
	}
	if revoked != 0 {
		t.Fatalf("the API answered session_revoked %d time(s); no invocation should present a superseded token", revoked)
	}

	stored := loadStoreForTest(t)
	if stored.Accounts["cdr"].RefreshToken != "refresh-gen1" {
		t.Fatalf("store does not hold the live pair: %q", stored.Accounts["cdr"].RefreshToken)
	}
}

// TestInvocationRefreshesWhenAdoptedAccessTokenIsAlsoStale covers the follow-up
// case: the pair on disk is the live one, but its access token has expired too
// (another invocation rotated it long enough ago). Adopting is not enough, so
// the invocation must go on to perform the exchange itself.
func TestInvocationRefreshesWhenAdoptedAccessTokenIsAlsoStale(t *testing.T) {
	srv, state := newRotatingSessionServer(t, "access-gen2", "refresh-gen2")

	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "cdr",
		Accounts: map[string]*config.Account{
			// Live refresh token, but the access token beside it is long expired.
			"cdr": {AccessToken: "access-gen1-expired", RefreshToken: "refresh-gen2", APIUrl: srv.URL},
		},
	})

	client, err := getClientFromResolved(&config.ResolvedConfig{
		Profile: "cdr",
		APIUrl:  srv.URL,
		Account: &config.Account{
			AccessToken:  "access-gen0-expired",
			RefreshToken: "refresh-gen1", // older than what is on disk
			APIUrl:       srv.URL,
		},
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	if _, err := client.GetMe(); err != nil {
		t.Fatalf("invocation should recover by exchanging the stored token: %v", err)
	}

	rotations, revoked := state.snapshot()
	if rotations != 1 {
		t.Fatalf("expected one exchange after the adopted access token failed, got %d", rotations)
	}
	if revoked != 0 {
		t.Fatalf("the API answered session_revoked %d time(s); the stored token should have been used", revoked)
	}
}

const (
	sessionHelperEnv    = "FLAGIFY_TEST_SESSION_HELPER"
	sessionHelperURLEnv = "FLAGIFY_TEST_SESSION_URL"
)

// TestConcurrentInvocationsShareOneRotation reproduces the day-to-day shape of
// the bug: several CLI processes on the same profile, all with an expired access
// token, racing to refresh a single-use token. Exactly one rotation may happen
// and nobody may be told the session was revoked.
func TestConcurrentInvocationsShareOneRotation(t *testing.T) {
	if os.Getenv(sessionHelperEnv) != "" {
		runSessionHelper(t)
		return
	}

	srv, state := newRotatingSessionServer(t, "access-gen0", "refresh-gen0")
	home := seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "cdr",
		Accounts: map[string]*config.Account{
			"cdr": {AccessToken: "access-expired", RefreshToken: "refresh-gen0", APIUrl: srv.URL},
		},
	})

	const invocations = 5
	var wg sync.WaitGroup
	failures := make([]string, invocations)
	for i := 0; i < invocations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=TestConcurrentInvocationsShareOneRotation")
			cmd.Env = append(os.Environ(),
				sessionHelperEnv+"="+strconv.Itoa(i),
				sessionHelperURLEnv+"="+srv.URL,
				"HOME="+home,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				failures[i] = string(out)
			}
		}(i)
	}
	wg.Wait()

	for i, out := range failures {
		if out != "" {
			t.Errorf("invocation %d failed:\n%s", i, out)
		}
	}

	rotations, revoked := state.snapshot()
	if revoked != 0 {
		t.Fatalf("the API answered session_revoked %d time(s) — a concurrent invocation presented a consumed token", revoked)
	}
	if rotations != 1 {
		t.Fatalf("expected one rotation shared by every invocation, got %d", rotations)
	}
}

// runSessionHelper is one CLI invocation: resolve the profile from the shared
// config and make an authenticated call.
func runSessionHelper(t *testing.T) {
	apiURL := os.Getenv(sessionHelperURLEnv)
	client, err := getClientFromResolved(&config.ResolvedConfig{
		Profile: "cdr",
		APIUrl:  apiURL,
		Account: &config.Account{
			AccessToken:  "access-expired",
			RefreshToken: "refresh-gen0",
			APIUrl:       apiURL,
		},
	})
	if err != nil {
		t.Fatalf("helper build client: %v", err)
	}
	if _, err := client.GetMe(); err != nil {
		t.Fatalf("helper call failed: %v", err)
	}
}
