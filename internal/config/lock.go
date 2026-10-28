package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// processLock serializes goroutines of THIS process before they contend for the
// file lock. flock is owned by the open file description, so two goroutines
// opening the lock file separately would block each other at the syscall with no
// way to tell that apart from a real peer process. WithLock must never be
// nested: the inner call cannot get this mutex and fails with a clear error
// instead of hanging.
var processLock sync.Mutex

// lockTimeout bounds how long a CLI invocation waits for the config lock. It is
// deliberately longer than the API client's 10s HTTP timeout: a token refresh
// runs inside the critical section, so a waiter must outlast the worst-case
// refresh of the process ahead of it instead of giving up mid-rotation.
const lockTimeout = 15 * time.Second

// lockRetryInterval is the poll interval while the lock is contended.
const lockRetryInterval = 25 * time.Millisecond

// LockPath returns the absolute path of the advisory lock that serializes
// read-modify-write cycles over config.json. It is a sibling of the config and
// never holds credentials — only the lock state of the file descriptor.
func LockPath() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "config.lock"), nil
}

// WithLock runs fn while holding an exclusive, cross-process advisory lock on
// the config. Every mutation of config.json must go through here (normally via
// UpdateStore): the store is rewritten whole on each save, so an unsynchronized
// read-modify-write silently drops a concurrent process's write — including a
// freshly rotated, single-use refresh token, which costs the user their session.
func WithLock(fn func() error) error {
	path, err := LockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	if !acquireProcessLock(lockTimeout) {
		return fmt.Errorf(
			"timed out after %s waiting for this process's config lock — WithLock/UpdateStore must not be nested",
			lockTimeout,
		)
	}
	defer processLock.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open config lock at %s: %w", path, err)
	}
	defer f.Close()

	deadline := time.Now().Add(lockTimeout)
	for {
		acquired, err := tryLockExclusive(f)
		if err != nil {
			return fmt.Errorf("failed to lock %s: %w", path, err)
		}
		if acquired {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"timed out after %s waiting for the config lock at %s — another flagify process is still writing",
				lockTimeout, path,
			)
		}
		time.Sleep(lockRetryInterval)
	}
	defer func() { _ = unlock(f) }()

	return fn()
}

// ErrNoChange tells UpdateStore that the transaction decided not to change
// anything, so the file is left exactly as it is instead of being rewritten
// with identical content. UpdateStore reports success.
var ErrNoChange = errors.New("config: no change")

// UpdateStore is the only safe way to mutate config.json. It takes the lock,
// loads the store fresh from disk (so the caller never edits a stale snapshot),
// applies fn, and writes the result atomically and durably — all inside one
// critical section. fn may be called with a store that another process changed
// moments ago; that is the point.
//
// A non-nil error from fn aborts the update and leaves the file untouched;
// ErrNoChange does the same but is reported as success.
func UpdateStore(fn func(*Store) error) error {
	if fn == nil {
		return fmt.Errorf("nil update function")
	}
	return WithLock(func() error {
		store, err := loadOrMigrateLocked()
		if err != nil {
			return err
		}
		if err := fn(store); err != nil {
			if errors.Is(err, ErrNoChange) {
				return nil
			}
			return err
		}
		return saveStoreLocked(store)
	})
}

// acquireProcessLock takes the in-process mutex, giving up after timeout so a
// nested call reports a usable error instead of deadlocking forever.
func acquireProcessLock(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if processLock.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(lockRetryInterval)
	}
}
