package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	APIUrl       string `json:"apiUrl,omitempty"`
	Project      string `json:"project,omitempty"`
	Token        string `json:"token,omitempty"` // deprecated, kept for compat
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

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".flagify", "config.json"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
