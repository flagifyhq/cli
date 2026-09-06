package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/flagifyhq/cli/internal/config"
)

func TestEnsureProfileDeviceID(t *testing.T) {
	work := &config.Account{}
	personal := &config.Account{}

	if err := ensureProfileDeviceID(work, bytes.NewReader(bytes.Repeat([]byte{0x11}, profileDeviceIDBytes))); err != nil {
		t.Fatalf("generate work device ID: %v", err)
	}
	if err := ensureProfileDeviceID(personal, bytes.NewReader(bytes.Repeat([]byte{0x22}, profileDeviceIDBytes))); err != nil {
		t.Fatalf("generate personal device ID: %v", err)
	}
	if work.DeviceID == personal.DeviceID {
		t.Fatalf("profiles must receive distinct device IDs: %q", work.DeviceID)
	}
	if len(work.DeviceID) != len("cli-")+profileDeviceIDBytes*2 {
		t.Fatalf("unexpected device ID length: %q", work.DeviceID)
	}

	existing := work.DeviceID
	if err := ensureProfileDeviceID(work, bytes.NewReader(nil)); err != nil {
		t.Fatalf("existing device ID must be reused without reading randomness: %v", err)
	}
	if work.DeviceID != existing {
		t.Fatalf("existing device ID changed: got %q, want %q", work.DeviceID, existing)
	}
}

func TestEnsureProfileDeviceIDRejectsInvalidInputs(t *testing.T) {
	if err := ensureProfileDeviceID(nil, bytes.NewReader(nil)); err == nil {
		t.Fatal("expected nil profile error")
	}
	account := &config.Account{}
	if err := ensureProfileDeviceID(account, bytes.NewReader([]byte{0x01})); err == nil {
		t.Fatal("expected insufficient randomness error")
	}
	if account.DeviceID != "" {
		t.Fatalf("failed generation must not assign a partial ID: %q", account.DeviceID)
	}
}

func TestPrepareLoginProfilePersistsLegacyDeviceIDBeforeAuthorization(t *testing.T) {
	seedStore(t, &config.Store{
		Version: config.StoreVersion,
		Current: "personal",
		Accounts: map[string]*config.Account{
			"work":     {AccessToken: "legacy-access"},
			"personal": {},
		},
	})
	randomBytes := bytes.Repeat([]byte{0x33}, profileDeviceIDBytes)

	profile, cfg, err := prepareLoginProfile("work", bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatalf("prepare login profile: %v", err)
	}
	if profile != "work" {
		t.Fatalf("unexpected profile: %q", profile)
	}
	wantDeviceID := "cli-33333333333333333333333333333333"
	if cfg.DeviceID != wantDeviceID {
		t.Fatalf("prepared config device ID: got %q, want %q", cfg.DeviceID, wantDeviceID)
	}

	store := loadStoreForTest(t)
	if store.Current != "work" || store.Accounts["work"].DeviceID != wantDeviceID {
		t.Fatalf("device ID must be persisted before browser authorization: %+v", store)
	}
}

// scriptedAttempts returns a per-attempt function that yields the given outcomes
// in order and records how many times it was invoked.
func scriptedAttempts(outcomes ...attemptOutcome) (func(int) attemptOutcome, *int) {
	calls := 0
	fn := func(int) attemptOutcome {
		idx := calls
		calls++
		if idx >= len(outcomes) {
			return attemptOutcome{status: attemptExpired}
		}
		return outcomes[idx]
	}
	return fn, &calls
}

// 4.1 — empty-tokens callback on a TTY signals retry rather than terminal failure.
func TestRunLoginLoop_ExpiredThenSuccessRetries(t *testing.T) {
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptExpired},
		attemptOutcome{status: attemptSuccess, accessToken: "at", refreshToken: "rt"},
	)

	access, refresh, err := runLoginLoop(maxLoginAttempts, attempt)
	if err != nil {
		t.Fatalf("expected retry to recover, got error: %v", err)
	}
	if access != "at" || refresh != "rt" {
		t.Fatalf("unexpected tokens: access=%q refresh=%q", access, refresh)
	}
	if *calls != 2 {
		t.Fatalf("expected 2 attempts (expired then success), got %d", *calls)
	}
}

// 4.2 — N empty-tokens callbacks then a valid one → succeeds within the budget.
func TestRunLoginLoop_MultipleExpiredThenSuccessWithinBudget(t *testing.T) {
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptExpired},
		attemptOutcome{status: attemptExpired},
		attemptOutcome{status: attemptSuccess, accessToken: "final-at", refreshToken: "final-rt"},
	)

	access, refresh, err := runLoginLoop(maxLoginAttempts, attempt)
	if err != nil {
		t.Fatalf("expected success on the final attempt, got error: %v", err)
	}
	if access != "final-at" || refresh != "final-rt" {
		t.Fatalf("unexpected tokens: access=%q refresh=%q", access, refresh)
	}
	if *calls != maxLoginAttempts {
		t.Fatalf("expected %d attempts, got %d", maxLoginAttempts, *calls)
	}
}

// 4.3 — retry budget exhausted → single actionable error and no tokens.
func TestRunLoginLoop_ExhaustedReturnsActionableError(t *testing.T) {
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptExpired},
		attemptOutcome{status: attemptExpired},
		attemptOutcome{status: attemptExpired},
	)

	access, refresh, err := runLoginLoop(maxLoginAttempts, attempt)
	if !errors.Is(err, errAuthIncomplete) {
		t.Fatalf("expected errAuthIncomplete, got %v", err)
	}
	if access != "" || refresh != "" {
		t.Fatalf("expected no tokens on failure, got access=%q refresh=%q", access, refresh)
	}
	if *calls != maxLoginAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", maxLoginAttempts, *calls)
	}
}

// 4.4 — non-TTY empty-tokens callback → single attempt, no reopen/loop.
func TestRunLoginLoop_NonTTYDoesNotLoop(t *testing.T) {
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptExpired},
	)

	_, _, err := runLoginLoop(1, attempt) // non-TTY caps attempts at 1
	if !errors.Is(err, errAuthIncomplete) {
		t.Fatalf("expected errAuthIncomplete, got %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected exactly 1 attempt in non-TTY mode, got %d", *calls)
	}
}

// 4.5 — outer timeout bounds the flow: a timeout stops retrying immediately.
func TestRunLoginLoop_TimeoutStopsImmediately(t *testing.T) {
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptTimeout},
		attemptOutcome{status: attemptSuccess, accessToken: "should-not-reach"},
	)

	_, _, err := runLoginLoop(maxLoginAttempts, attempt)
	if !errors.Is(err, errAuthTimedOut) {
		t.Fatalf("expected errAuthTimedOut, got %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected timeout to stop after 1 attempt, got %d", *calls)
	}
}

// A setup failure (e.g. listener bind) surfaces immediately without retrying.
func TestRunLoginLoop_SetupErrorStopsImmediately(t *testing.T) {
	wantErr := errors.New("failed to start local server")
	attempt, calls := scriptedAttempts(
		attemptOutcome{status: attemptError, err: wantErr},
	)

	_, _, err := runLoginLoop(maxLoginAttempts, attempt)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the setup error, got %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected setup error to stop after 1 attempt, got %d", *calls)
	}
}

func writeProjectFile(t *testing.T, dir string, data config.ProjectFileData) {
	t.Helper()
	if _, err := config.WriteProjectFile(dir, data); err != nil {
		t.Fatalf("write project file: %v", err)
	}
}

func TestAdoptCandidate_NoProjectFileReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if got := adoptCandidate(repo, "work"); got != nil {
		t.Fatalf("expected nil when no project file is present, got %+v", got)
	}
}

func TestAdoptCandidate_EmptyProfileReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})

	if got := adoptCandidate(repo, ""); got != nil {
		t.Fatalf("expected nil when profile is empty, got %+v", got)
	}
}

func TestAdoptCandidate_AlreadyPinnedReturnsNil(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "work",
	})

	if got := adoptCandidate(repo, "work"); got != nil {
		t.Fatalf("expected nil when preferredProfile already matches, got %+v", got)
	}
}

func TestAdoptCandidate_MismatchReturnsProjectFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})

	got := adoptCandidate(repo, "work")
	if got == nil {
		t.Fatalf("expected project file when pin differs, got nil")
	}
	if got.Data.PreferredProfile != "default" {
		t.Fatalf("unexpected project file: %+v", got.Data)
	}
	if got.Dir != repo {
		// t.TempDir() may canonicalize through /private on macOS; compare with EvalSymlinks.
		expected, _ := filepath.EvalSymlinks(repo)
		actual, _ := filepath.EvalSymlinks(got.Dir)
		if expected != actual {
			t.Fatalf("expected dir %q, got %q", repo, got.Dir)
		}
	}
}

func TestAdoptCandidate_EmptyPinMatchesAnyProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:     config.ProjectFileVersion,
		WorkspaceID: "ws_1",
		ProjectID:   "pr_1",
		Environment: "development",
	})

	got := adoptCandidate(repo, "work")
	if got == nil {
		t.Fatalf("expected adoptCandidate to return project file with empty pin")
	}
	if got.Data.PreferredProfile != "" {
		t.Fatalf("expected empty preferredProfile, got %q", got.Data.PreferredProfile)
	}
}

func TestAdoptCandidate_WalksUpFromSubdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	writeProjectFile(t, repo, config.ProjectFileData{
		Version:          config.ProjectFileVersion,
		WorkspaceID:      "ws_1",
		ProjectID:        "pr_1",
		Environment:      "development",
		PreferredProfile: "default",
	})
	subdir := filepath.Join(repo, "apps", "marketing")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := adoptCandidate(subdir, "work")
	if got == nil {
		t.Fatalf("expected adoptCandidate to walk up and find project file from %q", subdir)
	}
	if got.Data.PreferredProfile != "default" {
		t.Fatalf("unexpected data: %+v", got.Data)
	}
}
