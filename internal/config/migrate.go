package config

import (
	"fmt"
	"os"
	"time"
)

// migrateV1ToV2 projects a flat v1 Config into the v2 Store shape. If the v1
// record is empty (no tokens, no scope), returns an empty v2 store instead of
// a ghost "default" profile.
func migrateV1ToV2(v1 *Config) *Store {
	s := emptyStore()
	if v1 == nil || v1IsEmpty(v1) {
		return s
	}

	token := v1.AccessToken
	if token == "" {
		token = v1.Token
	}

	acc := &Account{
		AccessToken:  token,
		RefreshToken: v1.RefreshToken,
		APIUrl:       v1.APIUrl,
		ConsoleUrl:   v1.ConsoleUrl,
		Defaults: Defaults{
			Workspace:   v1.Workspace,
			WorkspaceID: v1.WorkspaceID,
			Project:     v1.Project,
			ProjectID:   v1.ProjectID,
			Environment: v1.Environment,
		},
	}
	s.Accounts[DefaultProfile] = acc
	s.Current = DefaultProfile
	return s
}

func v1IsEmpty(v1 *Config) bool {
	return v1.AccessToken == "" &&
		v1.RefreshToken == "" &&
		v1.Token == "" &&
		v1.APIUrl == "" &&
		v1.ConsoleUrl == "" &&
		v1.Workspace == "" &&
		v1.WorkspaceID == "" &&
		v1.Project == "" &&
		v1.ProjectID == "" &&
		v1.Environment == ""
}

// backupV1 copies the existing v1 file next to it before it is overwritten.
// First backup lands at <path>.bak; subsequent backups use a UTC timestamp suffix
// so earlier backups are never clobbered.
func backupV1(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	bak := path + ".bak"
	if _, err := os.Stat(bak); err == nil {
		bak = fmt.Sprintf("%s.bak.%s", path, time.Now().UTC().Format("20060102-150405"))
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.WriteFile(bak, data, 0o600)
}
