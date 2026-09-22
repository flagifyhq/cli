package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flagifyhq/cli/internal/config"
)

// deviceServer is a scripted fake of the device endpoints: /device/code
// returns the configured expiry/interval and each /device/token poll pops
// the next scripted response (the last one repeats).
type deviceServer struct {
	mu        sync.Mutex
	expiresIn int
	polls     []func(w http.ResponseWriter)
	pollCount int
}

func deviceError(code string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": code})
	}
}

func deviceSuccess(email string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		json.NewEncoder(w).Encode(map[string]any{
			"user":   map[string]any{"email": email},
			"tokens": map[string]string{"accessToken": "device-at", "refreshToken": "device-rt"},
		})
	}
}

func (s *deviceServer) start(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch r.URL.Path {
		case "/v1/auth/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "secret-device-code",
				"user_code":                 "BCDF-GHJK",
				"verification_uri":          "https://console.test/device",
				"verification_uri_complete": "https://console.test/device?code=BCDF-GHJK",
				"expires_in":                s.expiresIn,
				"interval":                  5,
			})
		case "/v1/auth/device/token":
			idx := min(s.pollCount, len(s.polls)-1)
			s.pollCount++
			s.polls[idx](w)
		default:
			// maybeAutoSelect's workspace listing — not part of the device flow.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// recordDeviceWaits replaces deviceWait with a non-sleeping fake that records
// each requested interval and still honours ctx cancellation.
func recordDeviceWaits(t *testing.T) *[]time.Duration {
	t.Helper()
	var waits []time.Duration
	original := deviceWait
	deviceWait = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	t.Cleanup(func() { deviceWait = original })
	return &waits
}

// seedDeviceProfile seeds a profile with stale workspace/project state so the
// test can prove the device flow resets exactly what the browser flow resets.
func seedDeviceProfile(t *testing.T, apiURL string) *config.Config {
	t.Helper()
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "work",
		Accounts: map[string]*config.Account{
			"work": {
				DeviceID: "cli-0123456789abcdef0123456789abcdef",
				APIUrl:   apiURL,
				Defaults: config.Defaults{Workspace: "old-ws", Project: "old-project"},
			},
		},
	})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func readConfigBytes(t *testing.T) []byte {
	t.Helper()
	path, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return data
}

func TestLoginDevice_PendingThenSuccessPersistsCredentials(t *testing.T) {
	waits := recordDeviceWaits(t)
	srv := &deviceServer{expiresIn: 600, polls: []func(http.ResponseWriter){
		deviceError("authorization_pending"),
		deviceError("authorization_pending"),
		deviceSuccess("jane@acme.com"),
	}}
	cfg := seedDeviceProfile(t, srv.start(t))

	if err := loginDevice(cfg); err != nil {
		t.Fatalf("loginDevice: %v", err)
	}

	if srv.pollCount != 3 {
		t.Fatalf("expected 3 polls, got %d", srv.pollCount)
	}
	for _, d := range *waits {
		if d != 5*time.Second {
			t.Fatalf("pending must keep the server interval, got waits %v", *waits)
		}
	}

	store := loadStoreForTest(t)
	account := store.Accounts["work"]
	if account.AccessToken != "device-at" || account.RefreshToken != "device-rt" {
		t.Fatalf("tokens not persisted: %+v", account)
	}
	if account.Defaults.Workspace != "" || account.Defaults.Project != "" {
		t.Fatalf("workspace/project must be reset like the browser flow: %+v", account)
	}
	if account.DeviceID != "cli-0123456789abcdef0123456789abcdef" {
		t.Fatalf("profile device ID must be preserved: %q", account.DeviceID)
	}
}

func TestLoginDevice_SlowDownIncreasesInterval(t *testing.T) {
	waits := recordDeviceWaits(t)
	srv := &deviceServer{expiresIn: 600, polls: []func(http.ResponseWriter){
		deviceError("slow_down"),
		deviceError("slow_down"),
		deviceSuccess("jane@acme.com"),
	}}
	cfg := seedDeviceProfile(t, srv.start(t))

	if err := loginDevice(cfg); err != nil {
		t.Fatalf("loginDevice: %v", err)
	}

	want := []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}
	if len(*waits) != len(want) {
		t.Fatalf("waits: got %v, want %v", *waits, want)
	}
	for i := range want {
		if (*waits)[i] != want[i] {
			t.Fatalf("waits: got %v, want %v", *waits, want)
		}
	}
}

func TestLoginDevice_TerminalErrorsLeaveConfigUntouched(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		wantErr error
		wantMsg string
	}{
		{name: "expired", code: "expired_token", wantErr: errDeviceExpired},
		{name: "denied", code: "access_denied", wantErr: errDeviceDenied},
		{name: "invalid grant", code: "invalid_grant", wantMsg: "invalid_grant"},
		{name: "unknown error value", code: "unsupported_grant_type", wantMsg: "unsupported_grant_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordDeviceWaits(t)
			srv := &deviceServer{expiresIn: 600, polls: []func(http.ResponseWriter){deviceError(tc.code)}}
			cfg := seedDeviceProfile(t, srv.start(t))
			before := readConfigBytes(t)

			err := loginDevice(cfg)
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tc.wantMsg)) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantMsg)
			}
			if srv.pollCount != 1 {
				t.Fatalf("terminal error must stop polling, got %d polls", srv.pollCount)
			}
			if string(readConfigBytes(t)) != string(before) {
				t.Fatal("config must not be written on failure")
			}
		})
	}
}

func TestLoginDevice_ContextTimeoutReportsExpired(t *testing.T) {
	original := deviceWait
	deviceWait = func(ctx context.Context, d time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}
	t.Cleanup(func() { deviceWait = original })

	srv := &deviceServer{expiresIn: 0, polls: []func(http.ResponseWriter){deviceError("authorization_pending")}}
	cfg := seedDeviceProfile(t, srv.start(t))
	before := readConfigBytes(t)

	if err := loginDevice(cfg); !errors.Is(err, errDeviceExpired) {
		t.Fatalf("got %v, want %v", err, errDeviceExpired)
	}
	if srv.pollCount != 0 {
		t.Fatalf("expired context must not poll, got %d polls", srv.pollCount)
	}
	if string(readConfigBytes(t)) != string(before) {
		t.Fatal("config must not be written on timeout")
	}
}

func TestLoginDevice_InterruptExits130WithoutSaving(t *testing.T) {
	exitCodes := make(chan int, 1)
	originalExit := exitProcess
	exitProcess = func(code int) { exitCodes <- code }
	t.Cleanup(func() { exitProcess = originalExit })

	interrupted := false
	originalWait := deviceWait
	deviceWait = func(ctx context.Context, d time.Duration) error {
		if !interrupted {
			interrupted = true
			self, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Errorf("find self: %v", err)
				return err
			}
			if err := self.Signal(os.Interrupt); err != nil {
				t.Errorf("send interrupt: %v", err)
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}
	t.Cleanup(func() { deviceWait = originalWait })

	// Pending forever; the 1s expiry ends the loop after the interrupt fires
	// (exitProcess is faked, so the process keeps running).
	srv := &deviceServer{expiresIn: 1, polls: []func(http.ResponseWriter){deviceError("authorization_pending")}}
	cfg := seedDeviceProfile(t, srv.start(t))
	before := readConfigBytes(t)

	_ = loginDevice(cfg)

	select {
	case code := <-exitCodes:
		if code != 130 {
			t.Fatalf("exit code: got %d, want 130", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Ctrl-C must exit the process")
	}
	if string(readConfigBytes(t)) != string(before) {
		t.Fatal("config must not be written after Ctrl-C")
	}
}

func TestLoginDevice_CodeRequestFailureDoesNotPoll(t *testing.T) {
	recordDeviceWaits(t)
	cfg := seedDeviceProfile(t, "http://127.0.0.1:1")

	err := loginDevice(cfg)
	if err == nil || !strings.Contains(err.Error(), "could not start device login") {
		t.Fatalf("expected wrapped code-request error, got %v", err)
	}
}

func TestHumanizeMinutes(t *testing.T) {
	if got := humanizeMinutes(600); got != "10 min" {
		t.Fatalf("humanizeMinutes(600) = %q", got)
	}
}
