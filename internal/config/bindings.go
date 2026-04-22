package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// canonicalBindingPath normalizes repoPath so the same directory accessed via
// different surface paths (symlinked /var/folders → /private/var/folders on
// macOS, a relative "." vs an absolute path) always hashes to the same
// binding key. If EvalSymlinks fails because the path does not exist yet,
// the absolute form is used so binding is still deterministic.
func canonicalBindingPath(repoPath string) (string, error) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return abs, nil
		}
		return "", err
	}
	return real, nil
}

// BindProfile records in the store that repoPath uses profile. The key is
// always the absolute form of repoPath so lookups are stable regardless of
// how the CLI was invoked.
func BindProfile(s *Store, repoPath, profile string) error {
	if s == nil {
		return fmt.Errorf("nil store")
	}
	if repoPath == "" {
		return fmt.Errorf("empty repo path")
	}
	if profile == "" {
		return fmt.Errorf("empty profile name")
	}
	key, err := canonicalBindingPath(repoPath)
	if err != nil {
		return err
	}
	if s.Bindings == nil {
		s.Bindings = map[string]Binding{}
	}
	s.Bindings[key] = Binding{Profile: profile}
	return nil
}

// UnbindProfile removes any binding for repoPath. Missing bindings are a no-op.
func UnbindProfile(s *Store, repoPath string) error {
	if s == nil || s.Bindings == nil {
		return nil
	}
	key, err := canonicalBindingPath(repoPath)
	if err != nil {
		return err
	}
	delete(s.Bindings, key)
	return nil
}

// BindingFor returns the binding recorded for repoPath, or (Binding{}, false)
// if none. Always compares against the canonicalized path.
func BindingFor(s *Store, repoPath string) (Binding, bool) {
	if s == nil || s.Bindings == nil || repoPath == "" {
		return Binding{}, false
	}
	key, err := canonicalBindingPath(repoPath)
	if err != nil {
		return Binding{}, false
	}
	b, ok := s.Bindings[key]
	return b, ok
}

// PurgeBindingsForProfile removes every binding pointing to profile. Used when
// a profile is removed via `auth remove` to avoid leaving ghost bindings that
// reference a deleted identity.
func PurgeBindingsForProfile(s *Store, profile string) int {
	if s == nil || s.Bindings == nil || profile == "" {
		return 0
	}
	removed := 0
	for path, b := range s.Bindings {
		if b.Profile == profile {
			delete(s.Bindings, path)
			removed++
		}
	}
	return removed
}
