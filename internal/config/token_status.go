package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type AuthStatus string

const (
	AuthStatusActive      AuthStatus = "active"
	AuthStatusRefreshable AuthStatus = "refreshable"
	AuthStatusExpired     AuthStatus = "expired"
	AuthStatusInvalid     AuthStatus = "invalid"
	AuthStatusLoggedOut   AuthStatus = "logged-out"
)

type localTokenState int

const (
	localTokenMissing localTokenState = iota
	localTokenActive
	localTokenExpired
	localTokenInvalid
)

type localTokenClaims struct {
	Type string `json:"type"`
	Exp  int64  `json:"exp"`
}

// AccountAuthStatus classifies only locally inspectable token metadata. It does
// not verify signatures or claim that a server-side session has not been revoked.
func AccountAuthStatus(account *Account, now time.Time) AuthStatus {
	if account == nil || (account.AccessToken == "" && account.RefreshToken == "") {
		return AuthStatusLoggedOut
	}

	accessState := inspectLocalToken(account.AccessToken, "access", now)
	refreshState := inspectLocalToken(account.RefreshToken, "refresh", now)
	if accessState == localTokenActive {
		return AuthStatusActive
	}
	if refreshState == localTokenActive {
		return AuthStatusRefreshable
	}
	if accessState == localTokenInvalid || refreshState == localTokenInvalid {
		return AuthStatusInvalid
	}
	return AuthStatusExpired
}

func inspectLocalToken(token, expectedType string, now time.Time) localTokenState {
	if token == "" {
		return localTokenMissing
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return localTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return localTokenInvalid
	}
	var claims localTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Type != expectedType || claims.Exp <= 0 {
		return localTokenInvalid
	}
	if claims.Exp <= now.Unix() {
		return localTokenExpired
	}
	return localTokenActive
}
