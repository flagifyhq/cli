//go:build !windows

package config

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLockExclusive attempts a non-blocking exclusive flock. It reports false
// (without an error) when another process holds the lock.
func tryLockExclusive(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, unix.EWOULDBLOCK):
		return false, nil
	default:
		return false, err
	}
}

func unlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
