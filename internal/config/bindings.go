package config

import (
	"fmt"
	"path/filepath"
)

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
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	if s.Bindings == nil {
		s.Bindings = map[string]Binding{}
	}
	s.Bindings[abs] = Binding{Profile: profile}
	return nil
}

// UnbindProfile removes any binding for repoPath. Missing bindings are a no-op.
func UnbindProfile(s *Store, repoPath string) error {
	if s == nil || s.Bindings == nil {
		return nil
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}
	delete(s.Bindings, abs)
	return nil
}

// BindingFor returns the binding recorded for repoPath, or (Binding{}, false)
// if none. Always compares against the absolute path.
func BindingFor(s *Store, repoPath string) (Binding, bool) {
	if s == nil || s.Bindings == nil || repoPath == "" {
		return Binding{}, false
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return Binding{}, false
	}
	b, ok := s.Bindings[abs]
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
