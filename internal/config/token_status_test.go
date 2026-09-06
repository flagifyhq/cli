package config_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/flagifyhq/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func localTestToken(t *testing.T, tokenType string, expiration time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"type": tokenType,
		"exp":  expiration.Unix(),
	})
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestAccountAuthStatus(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	futureAccess := localTestToken(t, "access", now.Add(time.Minute))
	futureRefresh := localTestToken(t, "refresh", now.Add(time.Minute))
	expiredAccess := localTestToken(t, "access", now)
	expiredRefresh := localTestToken(t, "refresh", now.Add(-time.Second))

	tests := []struct {
		name    string
		account *config.Account
		want    config.AuthStatus
	}{
		{name: "nil account", account: nil, want: config.AuthStatusLoggedOut},
		{name: "no tokens", account: &config.Account{}, want: config.AuthStatusLoggedOut},
		{name: "active access", account: &config.Account{AccessToken: futureAccess}, want: config.AuthStatusActive},
		{name: "refreshable", account: &config.Account{AccessToken: expiredAccess, RefreshToken: futureRefresh}, want: config.AuthStatusRefreshable},
		{name: "both expired", account: &config.Account{AccessToken: expiredAccess, RefreshToken: expiredRefresh}, want: config.AuthStatusExpired},
		{name: "malformed", account: &config.Account{AccessToken: "not-a-jwt"}, want: config.AuthStatusInvalid},
		{name: "wrong access type", account: &config.Account{AccessToken: futureRefresh}, want: config.AuthStatusInvalid},
		{name: "malformed access recoverable", account: &config.Account{AccessToken: "not-a-jwt", RefreshToken: futureRefresh}, want: config.AuthStatusRefreshable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, config.AccountAuthStatus(tt.account, now))
		})
	}
}
