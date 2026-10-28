package config

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The CLI is invoked as several independent processes — five Claude sessions
// running `flagify` against the same ~/.flagify/config.json is an ordinary
// Tuesday. The store is rewritten whole on every save, and the refresh token it
// carries is single-use and rotated server-side, so one dropped write costs the
// user their session. These tests use real subprocesses on purpose: goroutines
// sharing one address space would not reproduce it, since the lock that matters
// is the one between processes.
const (
	concurrentRotators = 5

	helperModeEnv    = "FLAGIFY_TEST_ROTATOR_MODE"
	helperProfileEnv = "FLAGIFY_TEST_ROTATOR_PROFILE"
	helperBarrierEnv = "FLAGIFY_TEST_ROTATOR_BARRIER"

	modeTransaction = "transaction" // config.UpdateStore
	modeSplit       = "split"       // LoadStore … SaveStore, the pre-fix shape
)

// TestConcurrentProcessRotationsNeverLoseAWrite is the regression guard for the
// bug that made Flagify CLI sessions "expire": concurrent invocations each
// rewrote the whole store, and the last writer silently reverted the token
// rotation of the others. The reverted token is already consumed server-side,
// so the user is told the session was revoked and has to log in again.
//
// The subtests run the same race twice. The split shape — load, then save in a
// separate critical section — loses writes even though every individual save is
// atomic and locked, which is exactly why the fix had to make the transaction
// span the read. UpdateStore must not lose any.
func TestConcurrentProcessRotationsNeverLoseAWrite(t *testing.T) {
	if mode := os.Getenv(helperModeEnv); mode != "" {
		runRotatorHelper(t, mode)
		return
	}

	t.Run("split load and save loses rotations", func(t *testing.T) {
		survived := runRotationRace(t, modeSplit)
		lost := concurrentRotators - survived
		t.Logf("perdidas %d de %d rotaciones con LoadStore … SaveStore", lost, concurrentRotators)
		if lost == 0 {
			t.Fatal(
				"reading the store and saving it back in two critical sections no longer " +
					"loses concurrent writes — if LoadStore/SaveStore gained a compare-and-set, " +
					"this expectation is stale and should be updated, not deleted",
			)
		}
	})

	t.Run("UpdateStore keeps every rotation", func(t *testing.T) {
		survived := runRotationRace(t, modeTransaction)
		if survived != concurrentRotators {
			t.Fatalf(
				"UpdateStore lost %d of %d concurrent token rotations — a lost rotation is a "+
					"revoked session for the user",
				concurrentRotators-survived, concurrentRotators,
			)
		}
	})
}

// runRotationRace seeds one profile per rotator, launches that many real
// processes that each rotate their own profile's tokens at the same moment, and
// reports how many rotations are present in the file afterwards.
func runRotationRace(t *testing.T, mode string) int {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	seed := emptyStore()
	seed.Current = profileName(0)
	for i := 0; i < concurrentRotators; i++ {
		seed.Accounts[profileName(i)] = &Account{AccessToken: "seed", RefreshToken: "seed"}
	}
	if err := SaveStore(seed); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	// Every helper reads before any helper writes. That ordering is what a real
	// race produces intermittently; forcing it keeps the test deterministic
	// instead of flaky.
	barrier := home + "/release-the-writers"

	var wg sync.WaitGroup
	for i := 0; i < concurrentRotators; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=TestConcurrentProcessRotationsNeverLoseAWrite")
			cmd.Env = append(os.Environ(),
				helperModeEnv+"="+mode,
				helperProfileEnv+"="+profileName(i),
				helperBarrierEnv+"="+barrier,
				"HOME="+home,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("rotator %d failed: %v\n%s", i, err, out)
			}
		}(i)
	}

	waitForHelpersToRead()
	if err := os.WriteFile(barrier, []byte("go"), 0o600); err != nil {
		t.Fatalf("release barrier: %v", err)
	}
	wg.Wait()

	final, err := LoadStore()
	if err != nil {
		t.Fatalf("read store after race: %v", err)
	}

	survived := 0
	for i := 0; i < concurrentRotators; i++ {
		acc, ok := final.Accounts[profileName(i)]
		if !ok {
			t.Fatalf("profile %s vanished from the store", profileName(i))
		}
		if acc.AccessToken == rotatedToken(i) && acc.RefreshToken == rotatedToken(i) {
			survived++
		}
	}
	return survived
}

// runRotatorHelper is the subprocess side: one CLI invocation rotating one
// profile's tokens, either as a split read/write or as one transaction.
func runRotatorHelper(t *testing.T, mode string) {
	profile := os.Getenv(helperProfileEnv)
	barrier := os.Getenv(helperBarrierEnv)

	rotate := func(store *Store) error {
		acc, ok := store.Accounts[profile]
		if !ok {
			return os.ErrNotExist
		}
		token := "rotated-" + profile
		acc.AccessToken = token
		acc.RefreshToken = token
		return nil
	}

	switch mode {
	case modeSplit:
		store, err := LoadStore()
		if err != nil {
			t.Fatalf("helper load: %v", err)
		}
		awaitBarrier(barrier)
		if err := rotate(store); err != nil {
			t.Fatalf("helper rotate: %v", err)
		}
		if err := SaveStore(store); err != nil {
			t.Fatalf("helper save: %v", err)
		}
	case modeTransaction:
		awaitBarrier(barrier)
		if err := UpdateStore(rotate); err != nil {
			t.Fatalf("helper transaction: %v", err)
		}
	default:
		t.Fatalf("unknown rotator mode %q", mode)
	}
}

func awaitBarrier(path string) {
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitForHelpersToRead gives every subprocess time to start and take its read
// before the writes are released.
func waitForHelpersToRead() {
	time.Sleep(900 * time.Millisecond)
}

func profileName(i int) string { return "profile-" + strconv.Itoa(i) }

func rotatedToken(i int) string { return "rotated-" + profileName(i) }
