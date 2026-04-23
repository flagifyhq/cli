package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// StoreVersion is the on-disk schema version. Bumps are additive where possible.
	StoreVersion = 2

	// DefaultProfile is the profile name used when migrating a v1 single-account
	// config to v2, and as the fallback when the shim Save() has no other signal.
	DefaultProfile = "default"
)

// Store is the root of ~/.flagify/config.json in schema v2.
type Store struct {
	Version  int                 `json:"version"`
	Current  string              `json:"current,omitempty"`
	Accounts map[string]*Account `json:"accounts,omitempty"`
	Bindings map[string]Binding  `json:"bindings,omitempty"`
}

// Account is a single authenticated identity (local name, tokens, URLs, and
// workspace/project/environment defaults). Tokens never travel outside this struct.
type Account struct {
	AccessToken  string    `json:"accessToken,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	APIUrl       string    `json:"apiUrl,omitempty"`
	ConsoleUrl   string    `json:"consoleUrl,omitempty"`
	User         *UserInfo `json:"user,omitempty"`
	Defaults     Defaults  `json:"defaults,omitempty"`
}

// UserInfo is the cached identity of the logged-in user. Best-effort, updated on login.
type UserInfo struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// Defaults is the workspace/project/environment scope that a profile falls back to
// when no project file, binding, env var, or CLI flag provides something more specific.
type Defaults struct {
	Workspace   string `json:"workspace,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Project     string `json:"project,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// Binding associates a local repo path to a profile, independent of the committable
// project file. Never transmitted; purely a local preference.
type Binding struct {
	Profile string `json:"profile"`
}

// LoadStore reads ~/.flagify/config.json and returns the v2 store. Never writes.
// A missing file yields an empty v2 store. A v1 file is projected to v2 in memory
// without touching disk — use LoadOrMigrate when write-through migration is desired.
func LoadStore() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyStore(), nil
		}
		return nil, err
	}

	return parseStore(data, path)
}

// LoadOrMigrate reads the store, migrating a v1 file in place (with backup) on first
// access. This is the entry point for CLI commands; MCP and pure readers use LoadStore.
func LoadOrMigrate() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyStore(), nil
		}
		return nil, err
	}

	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if probe.Version == StoreVersion {
		return parseV2(data)
	}

	// Schema older than v2 (or unversioned v1) — migrate with backup.
	var v1 Config
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, fmt.Errorf("failed to parse v1 config at %s: %w", path, err)
	}

	store := migrateV1ToV2(&v1)

	if !v1IsEmpty(&v1) {
		if err := backupV1(path); err != nil {
			return nil, fmt.Errorf("failed to back up v1 config before migration: %w", err)
		}
	}

	if err := SaveStore(store); err != nil {
		return nil, fmt.Errorf("failed to write migrated config: %w", err)
	}

	return store, nil
}

// SaveStore writes the store atomically with 0600 on the file and 0700 on the dir.
func SaveStore(s *Store) error {
	if s == nil {
		return fmt.Errorf("nil store")
	}
	if s.Version == 0 {
		s.Version = StoreVersion
	}

	path, err := Path()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config.json.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	writeOK := false
	defer func() {
		if !writeOK {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	writeOK = true
	return nil
}

func parseStore(data []byte, path string) (*Store, error) {
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if probe.Version == StoreVersion {
		return parseV2(data)
	}

	var v1 Config
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, fmt.Errorf("failed to parse v1 config at %s: %w", path, err)
	}
	return migrateV1ToV2(&v1), nil
}

func parseV2(data []byte) (*Store, error) {
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Accounts == nil {
		s.Accounts = map[string]*Account{}
	}
	if s.Bindings == nil {
		s.Bindings = map[string]Binding{}
	}
	return &s, nil
}

func emptyStore() *Store {
	return &Store{
		Version:  StoreVersion,
		Accounts: map[string]*Account{},
		Bindings: map[string]Binding{},
	}
}

// ActiveAccount returns the account pointed to by Current, or nil if the store
// has no active profile yet.
func (s *Store) ActiveAccount() *Account {
	if s == nil || s.Current == "" {
		return nil
	}
	return s.Accounts[s.Current]
}
