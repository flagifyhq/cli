package config

import (
	"os"
	"path/filepath"
)

// Config is the flat, single-account view of the active profile. It is kept as
// a compatibility shim so every cmd/*.go caller that used config.Load/Save keeps
// working while the multi-account store lands incrementally. New code should
// prefer LoadStore / LoadOrMigrate / SaveStore directly.
type Config struct {
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	APIUrl       string `json:"apiUrl,omitempty"`
	ConsoleUrl   string `json:"consoleUrl,omitempty"`
	Workspace    string `json:"workspace,omitempty"`
	WorkspaceID  string `json:"workspaceId,omitempty"`
	Project      string `json:"project,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	Environment  string `json:"environment,omitempty"`
	Token        string `json:"token,omitempty"` // deprecated, kept for compat

	// profile records which v2 account the shim materialized from / will write to.
	// Unexported so json.Marshal ignores it. Empty string means "fall back to Current".
	profile string
}

// IsLoggedIn returns true if the user has a valid access token.
func (c *Config) IsLoggedIn() bool {
	return c.AccessToken != ""
}

// GetToken returns the access token (or legacy token).
func (c *Config) GetToken() string {
	if c.AccessToken != "" {
		return c.AccessToken
	}
	return c.Token
}

// Path returns the absolute path to ~/.flagify/config.json.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".flagify", "config.json"), nil
}

// Load returns the flat Config view of the current profile, migrating a v1 file
// in place on first read. A missing file yields an empty Config, matching the
// legacy behavior callers already handle.
func Load() (*Config, error) {
	s, err := LoadOrMigrate()
	if err != nil {
		return &Config{}, err
	}
	return configFromStore(s), nil
}

// Save writes the given Config back to the active profile. If the store has no
// current profile yet (first login), one named DefaultProfile is created and
// marked as current, preserving single-account auth behavior for `flagify auth login`.
func Save(cfg *Config) error {
	if cfg == nil {
		return nil
	}

	s, err := LoadOrMigrate()
	if err != nil {
		return err
	}
	if s.Accounts == nil {
		s.Accounts = map[string]*Account{}
	}

	profile := cfg.profile
	if profile == "" {
		profile = s.Current
	}
	if profile == "" {
		profile = DefaultProfile
	}

	acc := s.Accounts[profile]
	if acc == nil {
		acc = &Account{}
		s.Accounts[profile] = acc
	}

	token := cfg.AccessToken
	if token == "" && cfg.Token != "" {
		token = cfg.Token
	}
	acc.AccessToken = token
	acc.RefreshToken = cfg.RefreshToken
	acc.APIUrl = cfg.APIUrl
	acc.ConsoleUrl = cfg.ConsoleUrl
	acc.Defaults.Workspace = cfg.Workspace
	acc.Defaults.WorkspaceID = cfg.WorkspaceID
	acc.Defaults.Project = cfg.Project
	acc.Defaults.ProjectID = cfg.ProjectID
	acc.Defaults.Environment = cfg.Environment

	if s.Current == "" {
		s.Current = profile
	}

	return SaveStore(s)
}

// configFromStore materializes the flat Config view of the store's active profile.
func configFromStore(s *Store) *Config {
	if s == nil || s.Current == "" {
		return &Config{}
	}
	acc := s.Accounts[s.Current]
	if acc == nil {
		return &Config{}
	}
	return &Config{
		AccessToken:  acc.AccessToken,
		RefreshToken: acc.RefreshToken,
		APIUrl:       acc.APIUrl,
		ConsoleUrl:   acc.ConsoleUrl,
		Workspace:    acc.Defaults.Workspace,
		WorkspaceID:  acc.Defaults.WorkspaceID,
		Project:      acc.Defaults.Project,
		ProjectID:    acc.Defaults.ProjectID,
		Environment:  acc.Defaults.Environment,
		profile:      s.Current,
	}
}
